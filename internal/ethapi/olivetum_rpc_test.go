package ethapi

import (
	"context"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/types/genesisT"
)

func TestGetEconomyStatsReturnsStateCounters(t *testing.T) {
	minted := big.NewInt(1000)
	burned := big.NewInt(100)
	burnedTransfers := big.NewInt(70)
	burnedGas := big.NewInt(30)
	minerShare := big.NewInt(5)
	dividends := big.NewInt(40)

	rewardVaultStorage := map[common.Hash]common.Hash{
		{0: 0x0c}: common.BigToHash(minted),
		{0: 0x0d}: common.BigToHash(burned),
		{0: 0x0e}: common.BigToHash(dividends),
		{0: 0x0f}: common.BigToHash(burnedTransfers),
		{0: 0x10}: common.BigToHash(burnedGas),
		{0: 0x11}: common.BigToHash(minerShare),
	}
	burnStorage := map[common.Hash]common.Hash{
		{}: common.BigToHash(new(big.Int).SetUint64(150)),
	}
	dividendStorage := map[common.Hash]common.Hash{
		{0: 2}: common.BigToHash(new(big.Int).SetUint64(200)),
	}

	genesis := &genesisT.Genesis{
		Config: params.TestChainConfig,
		Alloc: genesisT.GenesisAlloc{
			params.RewardVault:    {Storage: rewardVaultStorage},
			core.BurnContract:     {Storage: burnStorage},
			core.DividendContract: {Storage: dividendStorage},
		},
	}
	backend := newTestBackend(t, 0, genesis, beacon.New(ethash.NewFaker()), func(i int, b *core.BlockGen) {})
	api := NewEthereumAPI(backend)

	stats, err := api.GetEconomyStats(context.Background())
	if err != nil {
		t.Fatalf("GetEconomyStats error: %v", err)
	}
	if (*big.Int)(stats.TotalMinted).Cmp(minted) != 0 {
		t.Fatalf("totalMinted mismatch: got %v want %v", (*big.Int)(stats.TotalMinted), minted)
	}
	if (*big.Int)(stats.Burned).Cmp(burned) != 0 {
		t.Fatalf("burned mismatch: got %v want %v", (*big.Int)(stats.Burned), burned)
	}
	if (*big.Int)(stats.BurnedTransfers).Cmp(burnedTransfers) != 0 {
		t.Fatalf("burnedTransfers mismatch: got %v want %v", (*big.Int)(stats.BurnedTransfers), burnedTransfers)
	}
	if (*big.Int)(stats.BurnedGas).Cmp(burnedGas) != 0 {
		t.Fatalf("burnedGas mismatch: got %v want %v", (*big.Int)(stats.BurnedGas), burnedGas)
	}
	if (*big.Int)(stats.MinerBurnShare).Cmp(minerShare) != 0 {
		t.Fatalf("minerBurnShare mismatch: got %v want %v", (*big.Int)(stats.MinerBurnShare), minerShare)
	}
	expectedGross := new(big.Int).Add(new(big.Int).Set(burned), minerShare)
	if (*big.Int)(stats.GrossBurnCharged).Cmp(expectedGross) != 0 {
		t.Fatalf("grossBurnCharged mismatch: got %v want %v", (*big.Int)(stats.GrossBurnCharged), expectedGross)
	}
	expectedNet := new(big.Int).Sub(new(big.Int).Set(burned), dividends)
	if (*big.Int)(stats.NetBurnedAfterDividends).Cmp(expectedNet) != 0 {
		t.Fatalf("netBurnedAfterDividends mismatch: got %v want %v", (*big.Int)(stats.NetBurnedAfterDividends), expectedNet)
	}
	if stats.BurnRate != 150 {
		t.Fatalf("burnRate mismatch: got %d want %d", stats.BurnRate, 150)
	}
	if stats.DividendRate != 200 {
		t.Fatalf("dividendRate mismatch: got %d want %d", stats.DividendRate, 200)
	}
}

func offSessionBudgetWindowSlot(addr common.Address) common.Hash {
	var b [32]byte
	b[0] = 0x01
	copy(b[12:], addr.Bytes())
	return common.BytesToHash(b[:])
}

func offSessionBudgetSpentSlot(addr common.Address) common.Hash {
	var b [32]byte
	b[0] = 0x02
	copy(b[12:], addr.Bytes())
	return common.BytesToHash(b[:])
}

func offSessionBudgetWindowLocal(ts uint64) uint64 {
	off := int64(params.GetSessionTzOffsetSeconds())
	local := int64(ts) + off
	if local < 0 {
		return 0
	}
	t := time.Unix(local, 0).UTC()
	if t.Weekday() != time.Sunday && t.Hour() >= 12 && t.Hour() < 24 {
		return 0
	}
	day := int64(24 * time.Hour / time.Second)
	window := (local / day) * day
	if t.Weekday() == time.Monday && t.Hour() < 12 {
		window -= day
		if window < 0 {
			window = 0
		}
	}
	return uint64(window)
}

func hashUint64(v uint64) common.Hash {
	var b [32]byte
	binary.BigEndian.PutUint64(b[24:], v)
	return common.BytesToHash(b[:])
}

func TestGetOffSessionBudgetIncludesTxPoolPending(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldCap := params.GetOffSessionMaxPerTx()
	t.Cleanup(func() { params.SetOffSessionMaxPerTx(oldCap) })
	params.SetOffSessionMaxPerTx(big.NewInt(10))

	oldTz := params.GetSessionTzOffsetSeconds()
	t.Cleanup(func() { params.SetSessionTzOffsetSeconds(oldTz) })
	params.SetSessionTzOffsetSeconds(0)

	addr := common.HexToAddress("0x0000000000000000000000000000000000001234")
	ts := uint64(time.Date(2024, time.March, 3, 10, 0, 0, 0, time.UTC).Unix())
	windowLocal := offSessionBudgetWindowLocal(ts)

	storage := map[common.Hash]common.Hash{
		offSessionBudgetWindowSlot(addr): hashUint64(windowLocal),
		offSessionBudgetSpentSlot(addr):  common.BigToHash(big.NewInt(7)),
	}

	genesis := &genesisT.Genesis{
		Config:    params.TestChainConfig,
		Timestamp: ts,
		Alloc: genesisT.GenesisAlloc{
			params.OffSessionMaxPerTxContract: {Storage: storage},
		},
	}
	backend := newTestBackend(t, 0, genesis, beacon.New(ethash.NewFaker()), func(i int, b *core.BlockGen) {})
	pendingTx := types.NewTransaction(0, common.HexToAddress("0x1"), big.NewInt(3), 21000, big.NewInt(0), nil)
	queuedTx := types.NewTransaction(1, common.HexToAddress("0x2"), big.NewInt(2), 21000, big.NewInt(0), nil)
	backend.setTxPoolContentFrom(addr, []*types.Transaction{pendingTx}, []*types.Transaction{queuedTx})

	api := NewEthereumAPI(backend)
	budget, err := api.GetOffSessionBudget(context.Background(), addr)
	if err != nil {
		t.Fatalf("GetOffSessionBudget error: %v", err)
	}
	if budget.Session {
		t.Fatalf("expected off-session for timestamp %d", ts)
	}
	if !budget.Enforced {
		t.Fatalf("expected budget enforced at fork")
	}
	if (*big.Int)(budget.Limit).Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("limit mismatch: got %v want %v", (*big.Int)(budget.Limit), big.NewInt(10))
	}
	if (*big.Int)(budget.SpentConfirmed).Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("spentConfirmed mismatch: got %v want %v", (*big.Int)(budget.SpentConfirmed), big.NewInt(7))
	}
	if (*big.Int)(budget.SpentPending).Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("spentPending mismatch: got %v want %v", (*big.Int)(budget.SpentPending), big.NewInt(5))
	}
	if (*big.Int)(budget.SpentTotal).Cmp(big.NewInt(12)) != 0 {
		t.Fatalf("spentTotal mismatch: got %v want %v", (*big.Int)(budget.SpentTotal), big.NewInt(12))
	}
	if (*big.Int)(budget.Remaining).Sign() != 0 {
		t.Fatalf("expected remaining 0, got %v", (*big.Int)(budget.Remaining))
	}

	start := uint64(time.Date(2024, time.March, 3, 0, 0, 0, 0, time.UTC).Unix())
	end := start + uint64(36*time.Hour/time.Second)
	if uint64(budget.WindowStart) != start {
		t.Fatalf("windowStart mismatch: got %d want %d", uint64(budget.WindowStart), start)
	}
	if uint64(budget.WindowEnd) != end {
		t.Fatalf("windowEnd mismatch: got %d want %d", uint64(budget.WindowEnd), end)
	}
	if uint64(budget.ResetIn) != end-ts {
		t.Fatalf("resetIn mismatch: got %d want %d", uint64(budget.ResetIn), end-ts)
	}
}
