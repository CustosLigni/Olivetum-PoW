package olivetumhash

import (
	"encoding/binary"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

var errNoMiningWork = errors.New("no mining work available yet")

type remoteSealer struct {
	works        map[common.Hash]*types.Block
	currentBlock *types.Block
	currentWork  [4]string
	results      chan<- *types.Block

	olivetumhash   *Olivetumhash
	workCh         chan *sealTask
	fetchWorkCh    chan *sealWork
	fetchWorkForCh chan *sealWorkFor
	submitWorkCh   chan *mineResult
	fetchRateCh    chan chan uint64
	submitRateCh   chan *hashrate
	exitCh         chan struct{}

	stats   map[common.Address]*minerStats
	statsMu sync.Mutex
}

type sealTask struct {
	block   *types.Block
	results chan<- *types.Block
}

type hashrate struct {
	done chan struct{}
	rate uint64
	id   common.Hash
}

type sealWork struct {
	errc chan error
	res  chan [4]string
}

type sealWorkFor struct {
	address common.Address
	errc    chan error
	res     chan [4]string
}

type mineResult struct {
	nonce     types.BlockNonce
	mixDigest common.Hash
	hash      common.Hash

	errc chan error
}

const staleThreshold = 7

type minerStats struct {
	LastWork       time.Time
	LastSubmit     time.Time
	LastHashrate   time.Time
	ReportedHR     uint64
	WorkCount      uint64
	SubmitCount    uint64
	EffectiveShare uint64
}

func startRemoteSealer(o *Olivetumhash) *remoteSealer {
	s := &remoteSealer{
		olivetumhash:   o,
		works:          make(map[common.Hash]*types.Block),
		workCh:         make(chan *sealTask),
		fetchWorkCh:    make(chan *sealWork),
		fetchWorkForCh: make(chan *sealWorkFor),
		submitWorkCh:   make(chan *mineResult),
		fetchRateCh:    make(chan chan uint64),
		submitRateCh:   make(chan *hashrate),
		exitCh:         make(chan struct{}),
		stats:          make(map[common.Address]*minerStats),
	}
	go s.loop()
	return s
}

func (s *remoteSealer) loop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case work := <-s.workCh:
			s.results = work.results
			s.makeWork(work.block)

		case req := <-s.fetchWorkCh:
			if s.currentBlock == nil {
				req.errc <- errNoMiningWork
			} else {
				req.res <- s.currentWork
			}

		case req := <-s.fetchWorkForCh:
			block, results, err := s.olivetumhash.requestWorkFor(req.address)
			if err != nil {
				req.errc <- err
				continue
			}
			s.results = results
			s.updateWorkStat(req.address)
			work := s.makeWork(block)
			req.res <- work

		case result := <-s.submitWorkCh:
			if s.submitWork(result.nonce, result.mixDigest, result.hash) {
				result.errc <- nil
			} else {
				result.errc <- errInvalidPoW
			}

		case rate := <-s.submitRateCh:
			if rate != nil && rate.done != nil {
				close(rate.done)
			}

		case req := <-s.fetchRateCh:
			req <- 0

		case <-ticker.C:
			s.pruneStale()

		case <-s.exitCh:
			return
		}
	}
}

func (s *remoteSealer) pruneStale() {
	if s.currentBlock == nil {
		return
	}
	current := s.currentBlock.NumberU64()
	for hash, block := range s.works {
		if block.NumberU64()+7 <= current {
			delete(s.works, hash)
		}
	}
}

func (s *remoteSealer) makeWork(block *types.Block) [4]string {
	header := block.Header()
	hash := s.olivetumhash.SealHash(header)
	number := block.NumberU64()
	epoch := number / s.olivetumhash.config.epochLength

	var seed [32]byte
	binary.LittleEndian.PutUint64(seed[:8], epoch)
	copy(seed[8:], []byte("OlivetumhashDatasetSeed.........."))

	s.currentWork[0] = hash.Hex()
	s.currentWork[1] = common.BytesToHash(seed[:]).Hex()
	target := new(big.Int).Div(maxUint256, header.Difficulty)
	s.currentWork[2] = common.BytesToHash(target.Bytes()).Hex()
	s.currentWork[3] = hexutil.EncodeBig(block.Number())

	s.currentBlock = block
	s.works[hash] = block
	return s.currentWork
}

func (s *remoteSealer) submitWork(nonce types.BlockNonce, mixDigest common.Hash, sealhash common.Hash) bool {
	if s.currentBlock == nil {
		return false
	}
	block := s.works[sealhash]
	if block == nil {
		return false
	}
	header := block.Header()
	header.Nonce = nonce
	header.MixDigest = mixDigest
	if err := s.olivetumhash.verifySeal(header); err != nil {
		return false
	}
	s.updateSubmitStat(header.Coinbase)
	if s.results == nil {
		return false
	}
	select {
	case s.results <- block.WithSeal(header):
		return true
	default:
		return false
	}
}

func (s *remoteSealer) stop() {
	close(s.exitCh)
}

func (s *remoteSealer) updateWorkStat(addr common.Address) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	st := s.stats[addr]
	if st == nil {
		st = &minerStats{}
		s.stats[addr] = st
	}
	st.LastWork = time.Now()
	st.WorkCount++
}

func (s *remoteSealer) updateSubmitStat(addr common.Address) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	st := s.stats[addr]
	if st == nil {
		st = &minerStats{}
		s.stats[addr] = st
	}
	st.LastSubmit = time.Now()
	st.SubmitCount++
}

func (s *remoteSealer) updateHashrate(addr common.Address, hr uint64) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	st := s.stats[addr]
	if st == nil {
		st = &minerStats{}
		s.stats[addr] = st
	}
	st.LastHashrate = time.Now()
	st.ReportedHR = hr
}

func (s *remoteSealer) snapshotStats(window time.Duration) GatewayStats {
	now := time.Now()
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	res := GatewayStats{}
	for addr, st := range s.stats {
		active := now.Sub(st.LastWork) <= window || now.Sub(st.LastSubmit) <= window || now.Sub(st.LastHashrate) <= window
		res.TotalWork += st.WorkCount
		res.TotalSubmits += st.SubmitCount
		res.TotalReportedHashrate += st.ReportedHR
		if active {
			res.ActiveMiners++
		}
		res.Miners = append(res.Miners, GatewayMinerStat{
			Address:          addr,
			WorkCount:        st.WorkCount,
			SubmitCount:      st.SubmitCount,
			ReportedHashrate: st.ReportedHR,
			LastWork:         st.LastWork,
			LastSubmit:       st.LastSubmit,
			LastHashrate:     st.LastHashrate,
			Active:           active,
		})
	}
	return res
}
