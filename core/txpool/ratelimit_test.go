package txpool

import (
	"math"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/types/coregeth"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
)

type dummySubPool struct {
	seen      map[common.Hash]bool
	nextNonce uint64
	signer    types.Signer
	content   map[common.Address]map[uint64]*types.Transaction
}

func newDummySubPool(signer types.Signer) *dummySubPool {
	return &dummySubPool{
		seen:    make(map[common.Hash]bool),
		signer:  signer,
		content: make(map[common.Address]map[uint64]*types.Transaction),
	}
}

func (d *dummySubPool) Filter(tx *types.Transaction) bool { return true }

func (d *dummySubPool) Init(gasTip uint64, head *types.Header, reserve AddressReserver) error {
	return nil
}

func (d *dummySubPool) Close() error { return nil }

func (d *dummySubPool) Reset(oldHead, newHead *types.Header) {}

func (d *dummySubPool) SetGasTip(tip *big.Int) {}

func (d *dummySubPool) Has(hash common.Hash) bool { return d.seen[hash] }

func (d *dummySubPool) Get(common.Hash) *types.Transaction { return nil }

func (d *dummySubPool) Add(txs []*types.Transaction, local bool, sync bool) []error {
	errs := make([]error, len(txs))
	for i, tx := range txs {
		hash := tx.Hash()
		if d.seen[hash] {
			errs[i] = ErrAlreadyKnown
			continue
		}
		d.seen[hash] = true
		from, err := types.Sender(d.signer, tx)
		if err != nil {
			errs[i] = ErrInvalidSender
			continue
		}
		byNonce := d.content[from]
		if byNonce == nil {
			byNonce = make(map[uint64]*types.Transaction)
			d.content[from] = byNonce
		}
		byNonce[tx.Nonce()] = tx
		if n := tx.Nonce() + 1; n > d.nextNonce {
			d.nextNonce = n
		}
	}
	return errs
}

func (d *dummySubPool) Pending(PendingFilter) map[common.Address][]*LazyTransaction { return nil }

func (d *dummySubPool) SubscribeTransactions(ch chan<- core.NewTxsEvent, reorgs bool) event.Subscription {
	return event.NewSubscription(func(quit <-chan struct{}) error {
		<-quit
		return nil
	})
}

func (d *dummySubPool) Nonce(common.Address) uint64 { return d.nextNonce }

func (d *dummySubPool) Stats() (int, int) { return 0, 0 }

func (d *dummySubPool) Content() (map[common.Address][]*types.Transaction, map[common.Address][]*types.Transaction) {
	return nil, nil
}

func (d *dummySubPool) Locals() []common.Address { return nil }

func (d *dummySubPool) Status(common.Hash) TxStatus { return TxStatusUnknown }

type dummyChain struct {
	head   *types.Header
	config ctypes.ChainConfigurator
	state  *state.StateDB
}

func (d *dummyChain) Config() ctypes.ChainConfigurator { return d.config }

func (d *dummyChain) CurrentBlock() *types.Header { return d.head }

func (d *dummyChain) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return event.NewSubscription(func(quit <-chan struct{}) error {
		<-quit
		return nil
	})
}

func (d *dummyChain) StateAt(root common.Hash) (*state.StateDB, error) {
	return d.state, nil
}

func newOlivetumConfig(t *testing.T) ctypes.ChainConfigurator {
	t.Helper()
	cfg := &coregeth.CoreGethChainConfig{}
	if err := cfg.SetChainID(big.NewInt(30216931)); err != nil {
		t.Fatalf("set chain id: %v", err)
	}
	return cfg
}

func newTestPool(t *testing.T) (*TxPool, *dummyChain, *state.StateDB, types.Signer) {
	t.Helper()
	memdb := rawdb.NewMemoryDatabase()
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabase(memdb), nil)
	head := &types.Header{
		Number: big.NewInt(0),
		Root:   types.EmptyRootHash,
		Time:   uint64(12 * 3600),
	}
	chain := &dummyChain{
		head:   head,
		config: newOlivetumConfig(t),
		state:  statedb,
	}
	signer := types.LatestSigner(chain.config)
	sub := newDummySubPool(signer)
	pool, err := New(0, chain, []SubPool{sub})
	if err != nil {
		t.Fatalf("new txpool: %v", err)
	}
	t.Cleanup(func() {
		_ = pool.Close()
	})
	return pool, chain, statedb, signer
}

func (d *dummySubPool) ContentFrom(addr common.Address) ([]*types.Transaction, []*types.Transaction) {
	byNonce := d.content[addr]
	if len(byNonce) == 0 {
		return nil, nil
	}
	nonces := make([]uint64, 0, len(byNonce))
	for nonce := range byNonce {
		nonces = append(nonces, nonce)
	}
	sort.Slice(nonces, func(i, j int) bool { return nonces[i] < nonces[j] })
	pending := make([]*types.Transaction, 0, len(nonces))
	for _, nonce := range nonces {
		pending = append(pending, byNonce[nonce])
	}
	return pending, nil
}

func configureRuntime(t *testing.T, statedb *state.StateDB, limit uint64) {
	t.Helper()
	oldMin := params.GetMinTxAmount()
	oldMax := params.GetOffSessionMaxPerTx()
	oldLimit := params.GetTxRateLimit()

	params.SetMinTxAmount(new(big.Int))
	params.SetOffSessionMaxPerTx(new(big.Int).SetUint64(math.MaxUint64))
	params.SetTxRateLimit(limit)
	core.ResetTxRateUsage(statedb)

	t.Cleanup(func() {
		params.SetMinTxAmount(oldMin)
		params.SetOffSessionMaxPerTx(oldMax)
		params.SetTxRateLimit(oldLimit)
	})
}

func TestTxPoolRateLimit(t *testing.T) {
	pool, chain, statedb, signer := newTestPool(t)
	configureRuntime(t, statedb, 5)

	key, _ := crypto.GenerateKey()
	from := crypto.PubkeyToAddress(key.PublicKey)
	to := common.HexToAddress("0x0000000000000000000000000000000000000001")
	sessionTime := uint64(12 * 3600)
	chain.head.Time = sessionTime

	usage := core.GetTxRateUsage(statedb, from)
	usage.Count = params.GetTxRateLimit()
	usage.Start = sessionTime
	usage.Epoch = core.GetTxRateEpoch(statedb)
	core.SetTxRateUsage(statedb, from, usage)

	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: big.NewInt(1), Gas: 21000, GasPrice: big.NewInt(0)})
	if err := pool.Add([]*types.Transaction{tx}, true, true)[0]; err != ErrRateLimit {
		t.Fatalf("expected ErrRateLimit, got %v", err)
	}

	usage.Count = params.GetTxRateLimit() - 1
	core.SetTxRateUsage(statedb, from, usage)
	tx2 := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 1, To: &to, Value: big.NewInt(1), Gas: 21000, GasPrice: big.NewInt(0)})
	if err := pool.Add([]*types.Transaction{tx2}, true, true)[0]; err != nil {
		t.Fatalf("expected acceptance with allowance remaining, got %v", err)
	}
}

func TestTxPoolRateLimitIgnoresDuplicates(t *testing.T) {
	pool, chain, statedb, signer := newTestPool(t)
	configureRuntime(t, statedb, 2)

	key, _ := crypto.GenerateKey()
	from := crypto.PubkeyToAddress(key.PublicKey)
	to := common.HexToAddress("0x0000000000000000000000000000000000000001")
	chain.head.Time = 200

	usage := core.GetTxRateUsage(statedb, from)
	usage.Count = params.GetTxRateLimit() - 1
	usage.Start = chain.head.Time
	usage.Epoch = core.GetTxRateEpoch(statedb)
	core.SetTxRateUsage(statedb, from, usage)

	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: big.NewInt(1), Gas: 21000, GasPrice: big.NewInt(0)})
	if err := pool.Add([]*types.Transaction{tx}, true, true)[0]; err != nil {
		t.Fatalf("first tx rejected: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := pool.Add([]*types.Transaction{tx}, true, true)[0]; err != ErrAlreadyKnown {
			t.Fatalf("duplicate #%d expected ErrAlreadyKnown, got %v", i, err)
		}
	}
}

func TestTxPoolRateLimitAppliesToAdminTransfersAfterFork(t *testing.T) {
	pool, chain, statedb, signer := newTestPool(t)
	configureRuntime(t, statedb, 2)

	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	originalAdmin := params.TxRateLimitAdmin
	t.Cleanup(func() { params.TxRateLimitAdmin = originalAdmin })

	key, _ := crypto.GenerateKey()
	admin := crypto.PubkeyToAddress(key.PublicKey)
	params.TxRateLimitAdmin = admin

	chain.head.Time = uint64(12 * 3600)
	usage := core.GetTxRateUsage(statedb, admin)
	usage.Count = params.GetTxRateLimit()
	usage.Start = chain.head.Time
	usage.Epoch = core.GetTxRateEpoch(statedb)
	core.SetTxRateUsage(statedb, admin, usage)

	to := common.HexToAddress("0x0000000000000000000000000000000000000002")
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: big.NewInt(1), Gas: 21000, GasPrice: big.NewInt(0)})
	if err := pool.Add([]*types.Transaction{tx}, true, true)[0]; err != ErrRateLimit {
		t.Fatalf("expected ErrRateLimit, got %v", err)
	}

	toContract := params.TxRateLimitContract
	adminTx := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &toContract, Value: big.NewInt(0), Gas: 21000, GasPrice: big.NewInt(0), Data: []byte{0x02}})
	if err := pool.Add([]*types.Transaction{adminTx}, true, true)[0]; err != nil {
		t.Fatalf("admin management tx rejected: %v", err)
	}
}

func TestTxPoolRejectsUnauthorizedManagementTx(t *testing.T) {
	pool, _, statedb, signer := newTestPool(t)
	configureRuntime(t, statedb, 5)

	key, _ := crypto.GenerateKey()
	to := params.MinTxAmountContract
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: big.NewInt(0), Gas: 21000, GasPrice: big.NewInt(0), Data: []byte{0x01}})
	if err := pool.Add([]*types.Transaction{tx}, true, true)[0]; err != ErrManagementUnauthorized {
		t.Fatalf("expected ErrManagementUnauthorized, got %v", err)
	}
}

func TestDividendClaimsRequireZeroValue(t *testing.T) {
	pool, _, statedb, signer := newTestPool(t)
	configureRuntime(t, statedb, 5)

	key, _ := crypto.GenerateKey()
	to := core.DividendContract
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: big.NewInt(1), Gas: 21000, GasPrice: big.NewInt(0)})
	if err := pool.Add([]*types.Transaction{tx}, true, true)[0]; err != ErrDividendNotEligible {
		t.Fatalf("expected ErrDividendNotEligible for non-zero claim, got %v", err)
	}
}

func TestDividendAdminRespectsCooldown(t *testing.T) {
	pool, chain, statedb, signer := newTestPool(t)
	configureRuntime(t, statedb, 5)

	key, _ := crypto.GenerateKey()
	admin := crypto.PubkeyToAddress(key.PublicKey)
	originalAdmin := core.DividendAdmin
	core.DividendAdmin = admin
	t.Cleanup(func() { core.DividendAdmin = originalAdmin })

	core.SetDividendRate(50)
	t.Cleanup(func() { core.SetDividendRate(50) })

	now := uint64(time.Now().Unix())
	if !core.TriggerDividend(statedb, now) {
		t.Fatalf("failed to start dividend round")
	}
	chain.head.Time = now + 10

	to := core.DividendContract
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: big.NewInt(0), Gas: 21000, GasPrice: big.NewInt(0), Data: []byte{0x01}})
	if err := pool.Add([]*types.Transaction{tx}, true, true)[0]; err != ErrDividendRoundTooSoon {
		t.Fatalf("expected ErrDividendRoundTooSoon, got %v", err)
	}
}
