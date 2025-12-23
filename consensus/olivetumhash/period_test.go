package olivetumhash

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
)

type periodTestChain struct {
	cfg       ctypes.ChainConfigurator
	period    uint64
	hasPeriod bool
}

func newPeriodTestChain() *periodTestChain {
	return &periodTestChain{cfg: params.AllEthashProtocolChanges}
}

func (c *periodTestChain) Config() ctypes.ChainConfigurator { return c.cfg }
func (c *periodTestChain) CurrentHeader() *types.Header     { return nil }
func (c *periodTestChain) GetHeader(hash common.Hash, number uint64) *types.Header {
	return nil
}
func (c *periodTestChain) GetHeaderByNumber(number uint64) *types.Header { return nil }
func (c *periodTestChain) GetHeaderByHash(common.Hash) *types.Header     { return nil }
func (c *periodTestChain) GetTd(common.Hash, uint64) *big.Int            { return nil }
func (c *periodTestChain) OlivetumBlockPeriod(common.Hash, uint64) (uint64, bool) {
	if c.hasPeriod {
		return c.period, true
	}
	return 0, false
}

var _ consensus.ChainHeaderReader = (*periodTestChain)(nil)
var _ olivetumPeriodProvider = (*periodTestChain)(nil)

func TestResolveBlockPeriodPrefersProvider(t *testing.T) {
	prev := params.GetBlockPeriod()
	t.Cleanup(func() { params.SetBlockPeriod(prev) })
	params.SetBlockPeriod(15)

	parent := &types.Header{Number: big.NewInt(1)}
	chain := newPeriodTestChain()
	chain.period = 6
	chain.hasPeriod = true

	if got := resolveBlockPeriod(chain, parent); got != 6 {
		t.Fatalf("expected period 6 from provider, got %d", got)
	}
}

func TestResolveBlockPeriodFallsBackToParams(t *testing.T) {
	prev := params.GetBlockPeriod()
	t.Cleanup(func() { params.SetBlockPeriod(prev) })
	params.SetBlockPeriod(11)

	parent := &types.Header{Number: big.NewInt(1)}
	chain := newPeriodTestChain()

	if got := resolveBlockPeriod(chain, parent); got != 11 {
		t.Fatalf("expected fallback period 11, got %d", got)
	}
}

func TestVerifyHeaderRejectsSmallTimestampDelta(t *testing.T) {
	prev := params.GetBlockPeriod()
	t.Cleanup(func() { params.SetBlockPeriod(prev) })
	params.SetBlockPeriod(20)

	chain := newPeriodTestChain()
	chain.period = 8
	chain.hasPeriod = true

	engine := New(nil)

	parent := &types.Header{
		Number:      big.NewInt(1),
		Time:        100,
		Difficulty:  big.NewInt(2),
		GasLimit:    8_000_000,
		GasUsed:     0,
		Root:        types.EmptyRootHash,
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		UncleHash:   types.EmptyUncleHash,
		Extra:       []byte("test"),
	}

	minDelta := minTimestampIncrement(chain.period, false)

	header := &types.Header{
		ParentHash:  parent.Hash(),
		Number:      big.NewInt(2),
		Time:        parent.Time + minDelta - 1,
		GasLimit:    parent.GasLimit,
		GasUsed:     0,
		Difficulty:  big.NewInt(2),
		Root:        types.EmptyRootHash,
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		UncleHash:   types.EmptyUncleHash,
		Extra:       []byte("child"),
	}

	err := engine.verifyHeader(chain, header, parent, false, false, int64(header.Time))
	if err == nil || !errors.Is(err, errTimestampTooClose) {
		t.Fatalf("expected timestamp-too-close error, got %v", err)
	}
}
