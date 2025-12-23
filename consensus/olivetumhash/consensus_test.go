//go:build olivetest
// +build olivetest

package olivetumhash

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
	geparams "github.com/ethereum/go-ethereum/params/types/goethereum"
	"github.com/ethereum/go-ethereum/params/vars"
)

// Slot used by olivetum supply tracking (mirrors core/olivetum_supply.go).
var totalMintedSlot = common.Hash{0: 0x0c}

type headerChainMock struct {
	config  ctypes.ChainConfigurator
	headers map[common.Hash]*types.Header
	head    *types.Header
}

func newHeaderChainMock(config ctypes.ChainConfigurator, genesis *types.Header) *headerChainMock {
	m := &headerChainMock{
		config:  config,
		headers: make(map[common.Hash]*types.Header),
		head:    genesis,
	}
	hash := genesis.Hash()
	m.headers[hash] = genesis
	return m
}

func (m *headerChainMock) Config() ctypes.ChainConfigurator {
	return m.config
}

func (m *headerChainMock) CurrentHeader() *types.Header {
	return m.head
}

func (m *headerChainMock) GetHeader(hash common.Hash, number uint64) *types.Header {
	if header, ok := m.headers[hash]; ok && header.Number.Uint64() == number {
		return header
	}
	return nil
}

func (m *headerChainMock) GetHeaderByNumber(number uint64) *types.Header {
	for _, header := range m.headers {
		if header.Number.Uint64() == number {
			return header
		}
	}
	return nil
}

func newStateDB(t *testing.T) *state.StateDB {
	t.Helper()
	db := state.NewDatabase(rawdb.NewMemoryDatabase())
	statedb, err := state.New(types.EmptyRootHash, db, nil)
	if err != nil {
		t.Fatalf("failed to create state db: %v", err)
	}
	return statedb
}

func (m *headerChainMock) GetHeaderByHash(hash common.Hash) *types.Header {
	return m.headers[hash]
}

// Ensure headerChainMock satisfies consensus.ChainHeaderReader.
var _ consensus.ChainHeaderReader = (*headerChainMock)(nil)

func (m *headerChainMock) GetBlock(hash common.Hash, number uint64) *types.Block {
	return nil
}

func (m *headerChainMock) GetTd(hash common.Hash, number uint64) *big.Int {
	return nil
}

var _ consensus.ChainReader = (*headerChainMock)(nil)

func TestSealAndVerifyHeader(t *testing.T) {
	engine := New(&params.OlivetumhashConfig{
		EpochLength:        32,
		DatasetInitBytes:   4096,
		DatasetGrowthBytes: 0,
		MixRounds:          8,
	})

	config := &geparams.ChainConfig{
		ChainID:                 big.NewInt(2024),
		HomesteadBlock:          big.NewInt(0),
		EIP150Block:             big.NewInt(0),
		EIP155Block:             big.NewInt(0),
		EIP158Block:             big.NewInt(0),
		ByzantiumBlock:          big.NewInt(0),
		ConstantinopleBlock:     big.NewInt(0),
		PetersburgBlock:         big.NewInt(0),
		IstanbulBlock:           big.NewInt(0),
		BerlinBlock:             big.NewInt(0),
		TerminalTotalDifficulty: big.NewInt(0),
		Olivetumhash:            &params.OlivetumhashConfig{},
	}

	parent := &types.Header{
		Number:      big.NewInt(0),
		Difficulty:  new(big.Int).Set(vars.MinimumDifficulty),
		Time:        1,
		GasLimit:    8_000_000,
		GasUsed:     0,
		Root:        types.EmptyRootHash,
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		UncleHash:   types.EmptyUncleHash,
		Extra:       []byte("olivetumhash-test"),
	}

	chain := newHeaderChainMock(config, parent)

	header := &types.Header{
		ParentHash:  parent.Hash(),
		Number:      big.NewInt(1),
		Time:        parent.Time + 15,
		GasLimit:    parent.GasLimit,
		GasUsed:     0,
		Root:        types.EmptyRootHash,
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		UncleHash:   types.EmptyUncleHash,
		Extra:       []byte("mine"),
	}

	if err := engine.Prepare(chain, header); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	headerHash := engine.SealHash(header)
	target := difficultyToTarget(header.Difficulty)
	epoch := header.Number.Uint64() / engine.config.epochLength

	found := false
	for nonce := uint64(0); nonce < 1<<22; nonce++ {
		encoded := types.EncodeNonce(nonce)
		mix, digest := engine.computeSeal(headerHash, encoded, epoch)
		if compareDigest(digest, target) {
			header.Nonce = encoded
			header.MixDigest = mix
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("failed to find valid nonce")
	}

	if err := engine.verifySeal(header); err != nil {
		t.Fatalf("verifySeal failed: %v", err)
	}

	if err := engine.VerifyHeader(chain, header, true); err != nil {
		t.Fatalf("VerifyHeader failed: %v", err)
	}
}

// TestEtcForkDifficulty verifies difficulty behavior after the ETC fork (gap-drop 60s).
func TestEtcForkDifficulty(t *testing.T) {
	fork := params.GetDifficultyEtcForkBlock()
	if fork == nil {
		t.Skip("ETC fork height not set")
	}
	prevPeriod := params.GetBlockPeriod()
	defer params.SetBlockPeriod(prevPeriod)
	// Force a known period so the outcome is deterministic.
	params.SetBlockPeriod(15)

	parent := &types.Header{
		Number:     fork,
		Time:       1_000,
		Difficulty: big.NewInt(10_000_000_000), // 10 Gh
	}
	config := &geparams.ChainConfig{Olivetumhash: &params.OlivetumhashConfig{}}
	chain := newHeaderChainMock(config, parent)

	engine := New(nil)

	// Case 1: block at the target time - difficulty should remain unchanged.
	child := types.CopyHeader(parent)
	child.Number = new(big.Int).Add(parent.Number, common.Big1)
	child.Time = parent.Time + params.GetBlockPeriod()
	child.ParentHash = parent.Hash()
	engine.Prepare(chain, child)
	if child.Difficulty.Cmp(parent.Difficulty) != 0 {
		t.Fatalf("expected difficulty to stay the same at target, have %v want %v", child.Difficulty, parent.Difficulty)
	}

	// Case 2: delay >= 60s triggers the step-drop (ceil(time/60)).
	// With 120s delay we expect roughly half of the parent difficulty.
	child2 := types.CopyHeader(parent)
	child2.Number = new(big.Int).Add(parent.Number, common.Big1)
	child2.Time = parent.Time + 120
	child2.ParentHash = parent.Hash()
	engine.Prepare(chain, child2)

	pctTimes100 := new(big.Int).Mul(child2.Difficulty, big.NewInt(100))
	pctTimes100.Div(pctTimes100, parent.Difficulty) // percent of the parent difficulty
	if pctTimes100.Cmp(big.NewInt(45)) < 0 || pctTimes100.Cmp(big.NewInt(55)) > 0 {
		t.Fatalf("expected ~50%% difficulty for 120s delay, have %v%%", pctTimes100)
	}
}

func TestVerifyUnclesRejects(t *testing.T) {
	engine := New(nil)

	config := &geparams.ChainConfig{
		ChainID:        big.NewInt(1),
		HomesteadBlock: big.NewInt(0),
	}

	genesis := &types.Header{
		Number:      big.NewInt(0),
		Difficulty:  new(big.Int).Set(vars.MinimumDifficulty),
		Time:        1,
		GasLimit:    8_000_000,
		Root:        types.EmptyRootHash,
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		UncleHash:   types.EmptyUncleHash,
	}

	chain := newHeaderChainMock(config, genesis)

	header := types.CopyHeader(genesis)
	header.Number = big.NewInt(1)
	header.ParentHash = genesis.Hash()
	header.Time = genesis.Time + 15

	block := types.NewBlockWithHeader(header)
	block = block.WithBody(nil, []*types.Header{types.CopyHeader(header)})

	if err := engine.VerifyUncles(chain, block); !errors.Is(err, errTooManyUncles) {
		t.Fatalf("expected errTooManyUncles, got %v", err)
	}
}

func TestCalcDifficultyAdjustsWithTimestamps(t *testing.T) {
	engine := New(nil)
	prevPeriod := params.GetBlockPeriod()
	defer params.SetBlockPeriod(prevPeriod)
	params.SetBlockPeriod(15)
	parent := &types.Header{
		Number:     big.NewInt(1),
		Time:       100,
		Difficulty: big.NewInt(8_000_000),
	}

	same := engine.CalcDifficulty(nil, parent.Time+15, parent)
	if same.Cmp(parent.Difficulty) != 0 {
		t.Fatalf("expected difficulty to remain %v, got %v", parent.Difficulty, same)
	}

	fast := engine.CalcDifficulty(nil, parent.Time+5, parent)
	if want := big.NewInt(24_000_000); fast.Cmp(want) != 0 {
		t.Fatalf("expected difficulty %v for fast block, got %v", want, fast)
	}

	slow := engine.CalcDifficulty(nil, parent.Time+60, parent)
	if want := big.NewInt(2_000_000); slow.Cmp(want) != 0 {
		t.Fatalf("expected difficulty %v for slow block, got %v", want, slow)
	}

	verySlow := engine.CalcDifficulty(nil, parent.Time+600, parent)
	if want := big.NewInt(1_000_000); verySlow.Cmp(want) != 0 {
		t.Fatalf("expected difficulty clamp %v for very slow block, got %v", want, verySlow)
	}
}

func TestCalcDifficultyHonoursMinimum(t *testing.T) {
	engine := New(nil)
	prevPeriod := params.GetBlockPeriod()
	defer params.SetBlockPeriod(prevPeriod)
	params.SetBlockPeriod(15)
	minimumParent := &types.Header{
		Number:     big.NewInt(1),
		Time:       100,
		Difficulty: new(big.Int).Set(vars.MinimumDifficulty),
	}

	result := engine.CalcDifficulty(nil, minimumParent.Time+600, minimumParent)
	if result.Cmp(vars.MinimumDifficulty) != 0 {
		t.Fatalf("expected difficulty to stay at minimum %v, got %v", vars.MinimumDifficulty, result)
	}
}

func TestCalcDifficultyRespectsRuntimePeriod(t *testing.T) {
	engine := New(nil)
	parent := &types.Header{
		Number:     big.NewInt(1),
		Time:       200,
		Difficulty: big.NewInt(8_000_000),
	}

	prevPeriod := params.GetBlockPeriod()
	defer params.SetBlockPeriod(prevPeriod)

	params.SetBlockPeriod(30)
	fast := engine.CalcDifficulty(nil, parent.Time+15, parent)
	if want := big.NewInt(16_000_000); fast.Cmp(want) != 0 {
		t.Fatalf("expected difficulty %v when block faster than 30s target, got %v", want, fast)
	}

	params.SetBlockPeriod(10)
	slow := engine.CalcDifficulty(nil, parent.Time+20, parent)
	if want := big.NewInt(4_000_000); slow.Cmp(want) != 0 {
		t.Fatalf("expected difficulty %v when block slower than 10s target, got %v", want, slow)
	}
}

// TestEtcStepForkDifficulty checks the gentler step-drop after the new fork (>=76000).
func TestEtcStepForkDifficulty(t *testing.T) {
	fork := params.GetDifficultyEtcStepForkBlock()
	if fork == nil {
		t.Skip("ETC step fork height not set")
	}

	// Keep period and step-drop parameters deterministic.
	prevPeriod := params.GetBlockPeriod()
	sStart, sInterval, sDrop, sMax := params.GetDifficultyStepDrop()
	defer params.SetBlockPeriod(prevPeriod)
	defer params.SetDifficultyStepDrop(sStart, sInterval, sDrop, sMax)
	params.SetBlockPeriod(15)
	params.SetDifficultyStepDrop(120, 60, 200, 5000) // 2% every 60s, max 50%

	parent := &types.Header{
		Number:     fork,
		Time:       1_000,
		Difficulty: big.NewInt(10_000_000_000),
	}
	config := &geparams.ChainConfig{Olivetumhash: &params.OlivetumhashConfig{}}
	chain := newHeaderChainMock(config, parent)
	engine := New(nil)

	// Block at target time: no difficulty change.
	child := types.CopyHeader(parent)
	child.Number = new(big.Int).Add(parent.Number, common.Big1)
	child.Time = parent.Time + params.GetBlockPeriod()
	child.ParentHash = parent.Hash()
	engine.Prepare(chain, child)
	if child.Difficulty.Cmp(parent.Difficulty) != 0 {
		t.Fatalf("expected difficulty to stay the same at target, have %v want %v", child.Difficulty, parent.Difficulty)
	}

	// 120s delay: ETC base and first step-drop (~2%).
	child2 := types.CopyHeader(parent)
	child2.Number = new(big.Int).Add(parent.Number, common.Big1)
	child2.Time = parent.Time + 120
	child2.ParentHash = parent.Hash()
	engine.Prepare(chain, child2)
	ratio2 := new(big.Int).Mul(child2.Difficulty, big.NewInt(10000))
	ratio2.Div(ratio2, parent.Difficulty) // w bp (1/10000)
	if ratio2.Cmp(big.NewInt(9600)) < 0 || ratio2.Cmp(big.NewInt(9900)) > 0 {
		t.Fatalf("expected ~98%% difficulty after 120s delay, have %v bp", ratio2)
	}

	// 180s delay: second step-drop (~4% total).
	child3 := types.CopyHeader(parent)
	child3.Number = new(big.Int).Add(parent.Number, common.Big1)
	child3.Time = parent.Time + 180
	child3.ParentHash = parent.Hash()
	engine.Prepare(chain, child3)
	ratio3 := new(big.Int).Mul(child3.Difficulty, big.NewInt(10000))
	ratio3.Div(ratio3, parent.Difficulty)
	if ratio3.Cmp(big.NewInt(9300)) < 0 || ratio3.Cmp(big.NewInt(9700)) > 0 {
		t.Fatalf("expected ~95%% difficulty after 180s delay, have %v bp", ratio3)
	}
	if ratio3.Cmp(ratio2) >= 0 {
		t.Fatalf("expected further drop at 180s, have %v bp vs %v bp", ratio3, ratio2)
	}
}

func TestDatasetCacheRoundTrip(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv(cacheEnvVar, cacheDir)

	engine := New(nil)
	epoch := uint64(3)

	data1 := engine.dataset(epoch)
	if len(data1) == 0 {
		t.Fatalf("dataset should not be empty")
	}

	size := datasetSize(epoch, engine.config)
	cachePath := engine.cacheFilePath(epoch, size)
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected cache file to exist: %v", err)
	}

	engine.datasetLock.Lock()
	delete(engine.datasets, epoch)
	engine.datasetLock.Unlock()

	data2 := engine.dataset(epoch)
	if !bytes.Equal(data1, data2) {
		t.Fatalf("dataset loaded from cache differs from original")
	}

	// Corrupt the cache and ensure it gets rebuilt cleanly.
	if err := os.Truncate(cachePath, int64(len(data1)/2)); err != nil {
		t.Fatalf("failed to truncate cache file: %v", err)
	}
	engine.datasetLock.Lock()
	delete(engine.datasets, epoch)
	engine.datasetLock.Unlock()

	data3 := engine.dataset(epoch)
	if !bytes.Equal(data1, data3) {
		t.Fatalf("dataset after rebuild differs from original")
	}

	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("expected cache file after rebuild: %v", err)
	}
	if info.Size() != int64(len(data1)) {
		t.Fatalf("expected rebuilt cache size %d, got %d", len(data1), info.Size())
	}

	// Ensure cached files live where expected.
	if filepath.Dir(cachePath) != cacheDir {
		t.Fatalf("cache stored outside configured directory: %s", cachePath)
	}
}

func TestAccumulateRewardsMinting(t *testing.T) {
	defer params.SetRewardForkBlock(nil)
	params.SetRewardForkBlock(big.NewInt(0))

	statedb := newStateDB(t)

	const burnRate = 300 // 3%
	statedb.SetState(burnContractAddress, burnStorageSlot, common.BigToHash(new(big.Int).SetUint64(burnRate)))

	header := &types.Header{
		Coinbase: common.HexToAddress("0x9539c940a4adf4b26b539a56cc4c1372671acd97"),
		Number:   big.NewInt(0),
	}

	accumulateRewards(statedb, header)

	expectedGross := params.RewardBase()
	expectedBurn := new(big.Int).Mul(expectedGross, big.NewInt(burnRate))
	expectedBurn.Div(expectedBurn, burnDenominator)
	expectedPayout := new(big.Int).Sub(new(big.Int).Set(expectedGross), expectedBurn)

	coinbaseBalance := statedb.GetBalance(header.Coinbase).ToBig()
	if coinbaseBalance.Cmp(expectedPayout) != 0 {
		t.Fatalf("payout mismatch: have %s want %s", coinbaseBalance, expectedPayout)
	}

	minted := new(big.Int).SetBytes(statedb.GetState(params.RewardVault, totalMintedSlot).Bytes())
	if minted.Cmp(expectedGross) != 0 {
		t.Fatalf("minted total mismatch: have %s want %s", minted, expectedGross)
	}
}

func TestAccumulateRewardsSupplyCap(t *testing.T) {
	statedb := newStateDB(t)

	header := &types.Header{
		Coinbase: common.HexToAddress("0x31e4d731b5fa6026f25aad7bf5a7ae97ab1c9d52"),
		Number:   big.NewInt(0),
	}

	remaining := new(big.Int).Sub(params.MaxSupply(), big.NewInt(1))
	statedb.SetState(params.RewardVault, totalMintedSlot, common.BigToHash(remaining))

	accumulateRewards(statedb, header)

	minted := new(big.Int).SetBytes(statedb.GetState(params.RewardVault, totalMintedSlot).Bytes())
	if minted.Cmp(params.MaxSupply()) != 0 {
		t.Fatalf("expected minted total to reach max supply, have %s want %s", minted, params.MaxSupply())
	}
	if bal := statedb.GetBalance(header.Coinbase).ToBig(); bal.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("expected last payout of 1 wei, got %s", bal)
	}

	header.Number = big.NewInt(1)
	accumulateRewards(statedb, header)
	mintedAfter := new(big.Int).SetBytes(statedb.GetState(params.RewardVault, totalMintedSlot).Bytes())
	if mintedAfter.Cmp(params.MaxSupply()) != 0 {
		t.Fatalf("minted should stay capped: have %s want %s", mintedAfter, params.MaxSupply())
	}
	if bal := statedb.GetBalance(header.Coinbase).ToBig(); bal.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("expected no additional payout after cap, got %s", bal)
	}
}

func TestRewardForBlockHalving(t *testing.T) {
	defer params.SetRewardForkBlock(nil)
	params.SetRewardForkBlock(big.NewInt(0))

	base := params.RewardBase()
	half := new(big.Int).Rsh(new(big.Int).Set(base), 1)
	eighth := new(big.Int).Rsh(new(big.Int).Set(base), 3)
	floor := params.RewardFloor()

	tests := []struct {
		number uint64
		want   *big.Int
	}{
		{0, base},
		{params.RewardHalvingInterval, half},
		{params.RewardHalvingInterval * 3, eighth},
		{params.RewardHalvingInterval * 5, floor},
		{params.RewardHalvingInterval * 12, floor},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("block-%d", tc.number), func(t *testing.T) {
			got := rewardForBlock(new(big.Int).SetUint64(tc.number))
			if got.Cmp(tc.want) != 0 {
				t.Fatalf("reward mismatch at block %d: have %s want %s", tc.number, got, tc.want)
			}
		})
	}
}
