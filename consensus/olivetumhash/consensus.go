package olivetumhash

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/mutations"
	"github.com/ethereum/go-ethereum/params/vars"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/holiman/uint256"
	"golang.org/x/crypto/sha3"
)

const (
	maxDifficultyIncreaseFactor = 2
	// Relax the downward clamp so difficulty can decay quicker on low-hash networks.
	maxDifficultyDecreaseDivisor = 16
)

var (
	allowedFutureBlockTimeSeconds = int64(15)
	maxUint256                    = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	burnContractAddress           = common.HexToAddress("0x0000000000000000000000000000000000000b00")
	burnStorageSlot               = common.Hash{}
	burnDenominator               = big.NewInt(10000)

	errOlderBlockTime    = errors.New("timestamp older than parent")
	errTimestampTooClose = errors.New("timestamp increment below minimum")
	errTooManyUncles     = errors.New("uncles not supported")
	errInvalidMixDigest  = errors.New("invalid mix digest")
	errInvalidPoW        = errors.New("invalid proof-of-work")

	timestampTooCloseCounter = metrics.NewRegisteredCounter("olivetum/consensus/timestamp_too_close", nil)
	difficultyClampCounter   = metrics.NewRegisteredCounter("olivetum/consensus/difficulty_clamp", nil)
)

type olivetumPeriodProvider interface {
	OlivetumBlockPeriod(hash common.Hash, number uint64) (uint64, bool)
}

const defaultBurnRate = 50

// Olivetumhash implements a memory-hard PoW engine tailored for Olivetum.
type Olivetumhash struct {
	config engineConfig

	datasetLock sync.RWMutex
	datasets    map[uint64]*dataset
	prefetching map[uint64]struct{}
	cacheDir    string

	threads       int
	hashrateMeter metrics.Meter

	totalHashes   atomic.Uint64
	hashrate      atomic.Uint64
	lastHashCount atomic.Uint64

	fakeFull bool
	remote   *remoteSealer

	workForMu       sync.RWMutex
	workForProducer WorkForProducer
}

// New creates a new Olivetumhash engine from the given chain configuration.
func New(cfg *params.OlivetumhashConfig) *Olivetumhash {
	engine := &Olivetumhash{
		config:        resolveConfig(cfg),
		datasets:      make(map[uint64]*dataset),
		prefetching:   make(map[uint64]struct{}),
		cacheDir:      defaultCacheDir(),
		threads:       0,
		hashrateMeter: metrics.NewMeter(),
	}
	engine.remote = startRemoteSealer(engine)
	go engine.hashrateSampler()
	return engine
}

// NewFaker returns an engine that bypasses PoW verification, useful for tests.
func NewFaker() *Olivetumhash {
	engine := New(nil)
	engine.fakeFull = true
	return engine
}

// Author returns the proposer address (coinbase).
func (o *Olivetumhash) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// WorkForProducer builds a sealing block for a custom coinbase and returns the block
// alongside the result channel used to deliver sealed blocks back to the worker.
type WorkForProducer func(common.Address) (*types.Block, chan<- *types.Block, error)

// SetWorkForProducer wires a callback that can generate sealing work for custom
// coinbases (used by getWorkFor/submitWorkFor).
func (o *Olivetumhash) SetWorkForProducer(fn WorkForProducer) {
	o.workForMu.Lock()
	defer o.workForMu.Unlock()
	o.workForProducer = fn
}

func (o *Olivetumhash) requestWorkFor(address common.Address) (*types.Block, chan<- *types.Block, error) {
	o.workForMu.RLock()
	producer := o.workForProducer
	o.workForMu.RUnlock()
	if producer == nil {
		return nil, nil, errors.New("no work-for producer configured")
	}
	return producer(address)
}

// VerifyHeader verifies an individual header.
func (o *Olivetumhash) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	if o.fakeFull {
		return nil
	}
	number := header.Number.Uint64()
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	return o.verifyHeader(chain, header, parent, false, seal, time.Now().Unix())
}

// VerifyHeaders verifies a batch of headers concurrently.
func (o *Olivetumhash) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))
	if o.fakeFull || len(headers) == 0 {
		go func() {
			defer close(results)
			for range headers {
				results <- nil
			}
		}()
		return abort, results
	}

	go func() {
		defer close(results)
		// Use a stable timestamp for the whole batch like ethash does.
		unixNow := time.Now().Unix()

		// Verify the batch sequentially and prefer in-batch parents to avoid
		// ErrUnknownAncestor before headers are inserted into the DB.
		// This mirrors the behavior expected by the downloader which provides
		// a contiguous chain segment.
		for i, header := range headers {
			select {
			case <-abort:
				return
			default:
			}

			// Determine whether to check the seal for this header.
			seal := true
			if len(seals) > i {
				seal = seals[i]
			}

			// Try to resolve the parent from within the batch first
			var parent *types.Header
			if i > 0 {
				prev := headers[i-1]
				if prev.Hash() == header.ParentHash && prev.Number != nil && header.Number != nil &&
					prev.Number.Uint64()+1 == header.Number.Uint64() {
					parent = prev
				}
			}
			// Fallback to the chain database if the immediate previous header
			// is not the parent (defensive, though downloader sends contiguous sequences).
			if parent == nil {
				parent = chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
			}

			var err error
			if parent == nil {
				err = consensus.ErrUnknownAncestor
			} else {
				err = o.verifyHeader(chain, header, parent, false, seal, unixNow)
			}

			select {
			case results <- err:
			case <-abort:
				return
			}
		}
	}()

	return abort, results
}

func (o *Olivetumhash) verifyHeader(chain consensus.ChainHeaderReader, header, parent *types.Header, uncle bool, seal bool, unixNow int64) error {
	if uint64(len(header.Extra)) > vars.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.Extra), vars.MaximumExtraDataSize)
	}
	if !uncle {
		if header.Time > uint64(unixNow+allowedFutureBlockTimeSeconds) {
			return consensus.ErrFutureBlock
		}
	}
	if header.Time <= parent.Time {
		return errOlderBlockTime
	}
	blockPeriod := resolveBlockPeriod(chain, parent)
	minDelta := minTimestampIncrement(blockPeriod, params.IsAfterDifficultyFork(header.Number.Uint64()))
	if header.Time-parent.Time < minDelta {
		timestampTooCloseCounter.Inc(1)
		return fmt.Errorf("%w: have %d, want >= %d", errTimestampTooClose, header.Time-parent.Time, minDelta)
	}
	expected := o.CalcDifficulty(chain, header.Time, parent)
	if expected.Cmp(header.Difficulty) != 0 {
		return fmt.Errorf("invalid difficulty: have %v, want %v", header.Difficulty, expected)
	}
	if header.GasLimit > vars.MaxGasLimit {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, vars.MaxGasLimit)
	}
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}
	if header.BaseFee != nil {
		return fmt.Errorf("invalid baseFee: have %d, expected nil", header.BaseFee)
	}
	if err := misc.VerifyGaslimit(parent.GasLimit, header.GasLimit); err != nil {
		return err
	}
	if diff := new(big.Int).Sub(header.Number, parent.Number); diff.Cmp(big.NewInt(1)) != 0 {
		return consensus.ErrInvalidNumber
	}
	if header.WithdrawalsHash != nil {
		return fmt.Errorf("invalid withdrawalsHash: have %x, expected nil", header.WithdrawalsHash)
	}
	switch {
	case header.ExcessBlobGas != nil:
		return fmt.Errorf("invalid excessBlobGas: have %d, expected nil", header.ExcessBlobGas)
	case header.BlobGasUsed != nil:
		return fmt.Errorf("invalid blobGasUsed: have %d, expected nil", header.BlobGasUsed)
	case header.ParentBeaconRoot != nil:
		return fmt.Errorf("invalid parentBeaconRoot: have %#x, expected nil", header.ParentBeaconRoot)
	}
	if o.fakeFull {
		return nil
	}
	if err := mutations.VerifyDAOHeaderExtraData(chain.Config(), header); err != nil {
		return err
	}
	if !seal {
		return nil
	}
	return o.verifySeal(header)
}

// VerifyUncles ensures we don't include uncles (unsupported).
func (o *Olivetumhash) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if o.fakeFull {
		return nil
	}
	if len(block.Uncles()) > 0 {
		return errTooManyUncles
	}
	return nil
}

// Prepare sets the difficulty field ready for mining.
func (o *Olivetumhash) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	// Enforce minimal timestamp increment (future-stamp policy) before difficulty calc.
	period := resolveBlockPeriod(chain, parent)
	minDelta := minTimestampIncrement(period, params.IsAfterDifficultyFork(parent.Number.Uint64()+1))
	earliest := parent.Time + minDelta
	if header.Time < earliest {
		header.Time = earliest
	}

	header.Difficulty = o.CalcDifficulty(chain, header.Time, parent)
	header.MixDigest = common.Hash{}
	header.Nonce = types.BlockNonce{}
	return nil
}

// Finalize applies block rewards.
func (o *Olivetumhash) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, withdrawals []*types.Withdrawal) {
	accumulateRewards(state, header)
}

// FinalizeAndAssemble finalizes the state and assembles the block.
func (o *Olivetumhash) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt, withdrawals []*types.Withdrawal) (*types.Block, error) {
	if len(withdrawals) > 0 {
		return nil, errors.New("olivetumhash does not support withdrawals")
	}
	o.Finalize(chain, header, state, txs, uncles, withdrawals)
	header.Root = state.IntermediateRoot(chain.Config().IsEnabled(chain.Config().GetEIP161dTransition, header.Number))
	return types.NewBlock(header, txs, uncles, receipts, trie.NewStackTrie(nil)), nil
}

// SealHash is the hash used for mining.
func (o *Olivetumhash) SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra,
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	if header.WithdrawalsHash != nil || header.ExcessBlobGas != nil || header.BlobGasUsed != nil || header.ParentBeaconRoot != nil {
		panic("olivetumhash: unexpected post-merge fields in header")
	}
	if err := rlp.Encode(hasher, enc); err != nil {
		panic(err)
	}
	hasher.Sum(hash[:0])
	return hash
}

// Seal performs mining on the given block.
func (o *Olivetumhash) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	if o.fakeFull {
		go func() {
			select {
			case <-stop:
				return
			default:
			}
			header := types.CopyHeader(block.Header())
			header.Nonce = types.EncodeNonce(0)
			header.MixDigest = common.Hash{}
			select {
			case results <- block.WithSeal(header):
			case <-stop:
			}
		}()
		return nil
	}

	if o.remote != nil {
		select {
		case o.remote.workCh <- &sealTask{block: block, results: results}:
		default:
		}
	}

	header := block.Header()
	headerHash := o.SealHash(header)
	diff := header.Difficulty
	if diff == nil || diff.Sign() <= 0 {
		return errInvalidPoW
	}
	target := difficultyToTarget(diff)
	number := header.Number.Uint64()
	epoch := number / o.config.epochLength

	threads := o.Threads()
	if threads == 0 {
		threads = runtime.NumCPU()
	}
	if threads < 0 {
		return nil
	}

	found := make(chan *types.Block, 1)
	abort := make(chan struct{})
	var abortOnce sync.Once

	for i := 0; i < threads; i++ {
		startNonce := rand.New(rand.NewSource(time.Now().UnixNano() + int64(i))).Uint64()
		go o.mine(header, headerHash, target, epoch, startNonce, abort, stop, found, block)
	}

	go func() {
		defer abortOnce.Do(func() { close(abort) })

		select {
		case sealed := <-found:
			select {
			case results <- sealed:
			case <-stop:
			}
		case <-stop:
		}
	}()
	return nil
}

func (o *Olivetumhash) mine(header *types.Header, headerHash common.Hash, target *big.Int, epoch uint64, startNonce uint64, abort <-chan struct{}, stop <-chan struct{}, found chan<- *types.Block, block *types.Block) {
	defer func() {
		if r := recover(); r != nil {
			// avoid panics bubbling up from hashing
		}
	}()

	nonce := startNonce
	for {
		select {
		case <-abort:
			return
		case <-stop:
			return
		default:
		}

		localHeader := types.CopyHeader(header)
		localHeader.Nonce = types.EncodeNonce(nonce)
		mix, digest := o.computeSeal(headerHash, localHeader.Nonce, epoch)
		if compareDigest(digest, target) {
			localHeader.MixDigest = mix
			for now := time.Now().Unix(); now < int64(localHeader.Time); {
				select {
				case <-abort:
					return
				case <-stop:
					return
				case <-time.After(time.Second):
					now = time.Now().Unix()
				}
			}
			select {
			case found <- block.WithSeal(localHeader):
			default:
			}
			return
		}
		nonce++
		o.hashrateMeter.Mark(1)
		o.totalHashes.Add(1)
	}
}

// Hashrate returns the measured rate of hash computations. Olivetumhash does not
// currently track this value, so zero is reported.
func (o *Olivetumhash) Hashrate() float64 {
	local := o.hashrate.Load()
	if o.remote == nil {
		return float64(local)
	}
	agg := local
	req := make(chan uint64, 1)
	select {
	case o.remote.fetchRateCh <- req:
		agg += <-req
	default:
	}
	return float64(agg)
}

func (o *Olivetumhash) hashrateSampler() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		o.sampleHashrate()
	}
}

func (o *Olivetumhash) sampleHashrate() {
	cur := o.totalHashes.Load()
	prev := o.lastHashCount.Swap(cur)
	if cur >= prev {
		o.hashrate.Store(cur - prev)
	} else {
		// Counter wrapped; reset to avoid negative delta.
		o.hashrate.Store(cur)
	}
}

// Threads returns the configured mining threads (0 = NumCPU, <0 disables local mining).
func (o *Olivetumhash) Threads() int {
	return o.threads
}

// SetThreads updates the mining threads (0 = NumCPU, <0 disables local mining).
func (o *Olivetumhash) SetThreads(threads int) {
	o.threads = threads
}

// APIs exposes no additional RPC APIs.
func (o *Olivetumhash) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return PublicAPIs(o)
}

// CalcDifficulty adjusts difficulty targeting ~15s block time.
func (o *Olivetumhash) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	parentDiff := new(big.Int)
	if parent.Difficulty == nil || parent.Difficulty.Sign() == 0 {
		parentDiff.Set(vars.MinimumDifficulty)
	} else {
		parentDiff.Set(parent.Difficulty)
	}

	var timeDiff int64
	if time <= parent.Time {
		timeDiff = 1
	} else {
		timeDiff = int64(time - parent.Time)
		if timeDiff < 1 {
			timeDiff = 1
		}
	}

	blockPeriod := resolveBlockPeriod(chain, parent)
	nextNumber := parent.Number.Uint64() + 1
	target := new(big.Int).SetUint64(blockPeriod)
	denominator := big.NewInt(timeDiff)

	// ETC-style difficulty after the new fork: base formula plus optional step drop.
	if params.IsAfterDifficultyEtcStepFork(nextNumber) {
		candidate := calcEtcDifficulty(parentDiff, blockPeriod, timeDiff)
		start, interval, dropBps, maxDropBps := params.GetDifficultyStepDrop()
		if start > 0 && interval > 0 && dropBps > 0 && timeDiff >= int64(start) {
			steps := uint64((timeDiff - int64(start)) / int64(interval))
			steps += 1
			totalBps := steps * dropBps
			if maxDropBps > 0 && totalBps > maxDropBps {
				totalBps = maxDropBps
			}
			factorBp := int64(10000) - int64(totalBps)
			if factorBp < 1 {
				factorBp = 1
			}
			candidate.Mul(candidate, big.NewInt(factorBp))
			candidate.Div(candidate, big.NewInt(10000))
		}
		if candidate.Sign() == 0 {
			candidate.SetInt64(1)
		}
		if candidate.Cmp(vars.MinimumDifficulty) < 0 {
			candidate.Set(vars.MinimumDifficulty)
		}
		return candidate
	}

	if params.IsAfterDifficultyEtcFork(nextNumber) {
		quantizedDiff := timeDiff
		gapSeconds, gapMaxDiv := params.GetDifficultyGapDrop()
		if gapSeconds > 0 && timeDiff >= int64(gapSeconds) {
			step := int64(gapSeconds)
			// Only quantize above the threshold to avoid lowering difficulty for normal blocks.
			quantizedDiff = (timeDiff + step - 1) / step
			if quantizedDiff < 1 {
				quantizedDiff = 1
			}
			quantizedDiff *= step
		}

		candidate := calcEtcDifficulty(parentDiff, blockPeriod, quantizedDiff)
		if gapSeconds > 0 && quantizedDiff >= int64(gapSeconds) {
			ratio := (uint64(quantizedDiff) + gapSeconds - 1) / gapSeconds // ceil
			if ratio == 0 {
				ratio = 1
			}
			if gapMaxDiv > 0 && ratio > gapMaxDiv {
				ratio = gapMaxDiv
			}
			candidate.Div(candidate, new(big.Int).SetUint64(ratio))
		}
		if candidate.Sign() == 0 {
			candidate.SetInt64(1)
		}
		if candidate.Cmp(vars.MinimumDifficulty) < 0 {
			candidate.Set(vars.MinimumDifficulty)
		}
		return candidate
	}

	candidate := new(big.Int).Mul(parentDiff, target)
	candidate.Div(candidate, denominator)
	if candidate.Sign() == 0 {
		candidate.SetInt64(1)
	}

	incNum := int64(maxDifficultyIncreaseFactor)
	incDen := int64(1)
	decDiv := int64(maxDifficultyDecreaseDivisor)
	var gapSeconds uint64
	var gapMaxDiv uint64
	if params.IsAfterDifficultyFork(nextNumber) {
		pIncNum, pIncDen, pDecDiv := params.GetPostForkDifficultyClamps()
		if pIncNum > 0 {
			incNum = int64(pIncNum)
		}
		if pIncDen > 0 {
			incDen = int64(pIncDen)
		}
		if pDecDiv > 0 {
			decDiv = int64(pDecDiv)
		}
		gapSeconds, gapMaxDiv = params.GetDifficultyGapDrop()
	}
	liveDrop := params.IsAfterDifficultyLiveDropFork(nextNumber)

	if gapSeconds > 0 && uint64(timeDiff) >= gapSeconds {
		var ratio uint64
		if liveDrop {
			ratio = (uint64(timeDiff) + gapSeconds - 1) / gapSeconds
		} else {
			ratio = uint64(timeDiff) / gapSeconds
		}
		if ratio == 0 {
			ratio = 1
		}
		if gapMaxDiv > 0 && ratio > gapMaxDiv {
			ratio = gapMaxDiv
		}
		candidate.Div(parentDiff, new(big.Int).SetUint64(ratio))
		if candidate.Sign() == 0 {
			candidate.SetInt64(1)
		}
		log.Info("Difficulty gap drop applied", "block", nextNumber, "parent", parent.Number.Uint64(), "delay", timeDiff, "ratio", ratio, "newDiff", candidate)
	}

	upperBound := new(big.Int).Mul(parentDiff, big.NewInt(incNum))
	if incDen != 0 {
		upperBound.Div(upperBound, big.NewInt(incDen))
	}
	if upperBound.Sign() == 0 {
		upperBound.SetInt64(1)
	}
	if candidate.Cmp(upperBound) > 0 {
		difficultyClampCounter.Inc(1)
		candidate.Set(upperBound)
	}

	lowerDivisor := new(big.Int).SetInt64(decDiv)
	lowerBound := new(big.Int).Div(parentDiff, lowerDivisor)
	if lowerBound.Sign() == 0 {
		lowerBound.SetInt64(1)
	}
	if candidate.Cmp(lowerBound) < 0 {
		candidate.Set(lowerBound)
	}

	if candidate.Cmp(vars.MinimumDifficulty) < 0 {
		candidate.Set(vars.MinimumDifficulty)
	}
	return candidate
}

func resolveBlockPeriod(chain consensus.ChainHeaderReader, parent *types.Header) uint64 {
	period := params.GetBlockPeriod()
	if parent == nil {
		if period == 0 {
			return 1
		}
		return period
	}
	if provider, ok := chain.(olivetumPeriodProvider); ok {
		if stored, ok := provider.OlivetumBlockPeriod(parent.Hash(), parent.Number.Uint64()); ok && stored != 0 {
			return stored
		}
	}
	if period == 0 {
		return 1
	}
	return period
}

func minTimestampIncrement(period uint64, postFork bool) uint64 {
	if postFork {
		num, den := params.GetPostForkTimestampFraction()
		if den == 0 {
			den = 1
		}
		min := (period*num + den - 1) / den // ceil
		if min == 0 {
			min = 1
		}
		return min
	}
	min := (period + 5) / 6 // allow ~17% of target period, ceil
	if min == 0 {
		min = 1
	}
	return min
}

// Close releases resources.
func (o *Olivetumhash) Close() error {
	if o.remote != nil {
		o.remote.stop()
	}
	return nil
}

// calcEtcDifficulty implements the Ethash/EIP-100 style adjustment:
// new = parent + parent/2048 * clamp(1 - timeDiff/target, -99).
func calcEtcDifficulty(parentDiff *big.Int, blockPeriod uint64, timeDiff int64) *big.Int {
	target := blockPeriod
	if target == 0 {
		target = 1
	}
	factor := int64(1) - timeDiff/int64(target)
	if factor < -99 {
		factor = -99
	}
	quot := new(big.Int).Div(parentDiff, big.NewInt(2048))
	adj := new(big.Int).Mul(quot, big.NewInt(factor))
	candidate := new(big.Int).Add(parentDiff, adj)
	if candidate.Sign() <= 0 {
		candidate.SetInt64(1)
	}
	return candidate
}

func (o *Olivetumhash) verifySeal(header *types.Header) error {
	if header.Difficulty == nil || header.Difficulty.Sign() <= 0 {
		return errInvalidPoW
	}
	headerHash := o.SealHash(header)
	number := header.Number.Uint64()
	epoch := number / o.config.epochLength
	mix, digest := o.computeSeal(headerHash, header.Nonce, epoch)
	if mix != header.MixDigest {
		return errInvalidMixDigest
	}
	target := difficultyToTarget(header.Difficulty)
	if !compareDigest(digest, target) {
		return errInvalidPoW
	}
	return nil
}

func difficultyToTarget(diff *big.Int) *big.Int {
	if diff.Sign() <= 0 {
		return new(big.Int)
	}
	target := new(big.Int).Set(maxUint256)
	return target.Div(target, diff)
}

func compareDigest(digest common.Hash, target *big.Int) bool {
	if target.Sign() == 0 {
		return false
	}
	value := new(big.Int).SetBytes(digest[:])
	return value.Cmp(target) <= 0
}

func accumulateRewards(state *state.StateDB, header *types.Header) {
	if state == nil {
		return
	}

	gross := rewardForBlock(header.Number)
	if gross.Sign() == 0 {
		return
	}

	minted := getTotalMinted(state)
	remaining := new(big.Int).Sub(params.MaxSupply(), minted)
	if remaining.Sign() <= 0 {
		return
	}
	if gross.Cmp(remaining) > 0 {
		gross = remaining
	}
	if gross.Sign() == 0 {
		return
	}

	burn := computeRewardBurn(gross, state)
	payout := new(big.Int).Sub(new(big.Int).Set(gross), burn)

	newMinted := new(big.Int).Add(minted, gross)
	setTotalMinted(state, newMinted)

	if payout.Sign() > 0 {
		payoutU256, _ := uint256.FromBig(payout)
		state.AddBalance(header.Coinbase, payoutU256)
		core.AddHolding(state, header.Coinbase, payout, header.Time)
	}
}

func rewardForBlock(number *big.Int) *big.Int {
	reward := params.RewardBase()
	start := params.GetRewardForkBlock()
	if start == nil {
		start = big.NewInt(0)
	}
	num := number.Uint64()
	startNum := start.Uint64()
	floor := params.RewardFloor()
	interval := params.RewardHalvingInterval
	if num < startNum {
		return new(big.Int).Set(reward)
	}
	if interval == 0 {
		if reward.Cmp(floor) < 0 {
			return floor
		}
		return reward
	}
	halvings := (num - startNum) / interval
	if halvings == 0 {
		if reward.Cmp(floor) < 0 {
			return floor
		}
		return reward
	}
	for i := uint64(0); i < halvings; i++ {
		if reward.Cmp(floor) <= 0 {
			return floor
		}
		reward.Rsh(reward, 1)
	}
	if reward.Cmp(floor) < 0 {
		return floor
	}
	return reward
}

func computeRewardBurn(amount *big.Int, state *state.StateDB) *big.Int {
	if amount.Sign() == 0 {
		return new(big.Int)
	}
	rate := readBurnRate(state)
	if rate == 0 {
		return new(big.Int)
	}
	rateBig := new(big.Int).SetUint64(rate)
	burn := new(big.Int).Mul(amount, rateBig)
	burn.Div(burn, burnDenominator)
	if burn.Cmp(amount) > 0 {
		return new(big.Int).Set(amount)
	}
	return burn
}

func readBurnRate(state *state.StateDB) uint64 {
	if state == nil {
		return defaultBurnRate
	}
	stored := state.GetState(burnContractAddress, burnStorageSlot).Big().Uint64()
	if stored == 0 {
		return defaultBurnRate
	}
	return stored
}

func getTotalMinted(state *state.StateDB) *big.Int {
	if state == nil {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(state.GetState(params.RewardVault, common.Hash{0: 0x0c}).Bytes())
}

func setTotalMinted(state *state.StateDB, amount *big.Int) {
	if state == nil {
		return
	}
	if state.GetNonce(params.RewardVault) == 0 {
		state.SetNonce(params.RewardVault, 1)
	}
	state.SetState(params.RewardVault, common.Hash{0: 0x0c}, common.BigToHash(amount))
}
