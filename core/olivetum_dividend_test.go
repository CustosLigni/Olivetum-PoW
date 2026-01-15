package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

func newDividendState(t *testing.T) *state.StateDB {
	t.Helper()
	memdb := rawdb.NewMemoryDatabase()
	statedb, err := state.New(types.EmptyRootHash, state.NewDatabase(memdb), nil)
	if err != nil {
		t.Fatalf("new state: %v", err)
	}
	return statedb
}

func TestDividendRecentEntriesHaveIndependentTimestamps(t *testing.T) {
	statedb := newDividendState(t)
	addr := common.HexToAddress("0x1")
	t1 := uint64(1000)
	t2 := uint64(2000)
	t3 := uint64(3000)

	AddHolding(statedb, addr, big.NewInt(10), t1)
	AddHolding(statedb, addr, big.NewInt(20), t2)
	AddHolding(statedb, addr, big.NewInt(30), t3)

	if head := getRecentHead(statedb, addr); head != 0 {
		t.Fatalf("expected head 0, got %d", head)
	}
	if tail := getRecentTail(statedb, addr); tail != 3 {
		t.Fatalf("expected tail 3, got %d", tail)
	}

	amt, ts := getRecentEntry(statedb, addr, 0)
	if amt.Cmp(big.NewInt(10)) != 0 || ts != t1 {
		t.Fatalf("entry 0 mismatch: amt=%v ts=%d", amt, ts)
	}
	amt, ts = getRecentEntry(statedb, addr, 1)
	if amt.Cmp(big.NewInt(20)) != 0 || ts != t2 {
		t.Fatalf("entry 1 mismatch: amt=%v ts=%d", amt, ts)
	}
	amt, ts = getRecentEntry(statedb, addr, 2)
	if amt.Cmp(big.NewInt(30)) != 0 || ts != t3 {
		t.Fatalf("entry 2 mismatch: amt=%v ts=%d", amt, ts)
	}
}

func TestDividendRemoveHoldingUsesLifoForRecent(t *testing.T) {
	statedb := newDividendState(t)
	addr := common.HexToAddress("0x2")
	base := uint64(1_000_000)

	AddHolding(statedb, addr, big.NewInt(10), base)
	AddHolding(statedb, addr, big.NewInt(20), base+10)
	AddHolding(statedb, addr, big.NewInt(30), base+20)

	RemoveHolding(statedb, addr, big.NewInt(25), base+30)

	if head := getRecentHead(statedb, addr); head != 0 {
		t.Fatalf("expected head 0, got %d", head)
	}
	if tail := getRecentTail(statedb, addr); tail != 3 {
		t.Fatalf("expected tail 3, got %d", tail)
	}

	amt, _ := getRecentEntry(statedb, addr, 0)
	if amt.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("entry 0 amount mismatch: %v", amt)
	}
	amt, _ = getRecentEntry(statedb, addr, 1)
	if amt.Cmp(big.NewInt(20)) != 0 {
		t.Fatalf("entry 1 amount mismatch: %v", amt)
	}
	amt, _ = getRecentEntry(statedb, addr, 2)
	if amt.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("entry 2 amount mismatch: %v", amt)
	}
	if getHeldAmount(statedb, addr).Sign() != 0 {
		t.Fatalf("expected held amount 0, got %v", getHeldAmount(statedb, addr))
	}
}

func TestDividendMatureRecentMovesOnlyEligible(t *testing.T) {
	statedb := newDividendState(t)
	addr := common.HexToAddress("0x3")
	tOld := uint64(1_000_000)
	tNew := tOld + 100

	AddHolding(statedb, addr, big.NewInt(10), tOld)
	AddHolding(statedb, addr, big.NewInt(20), tNew)

	now := tOld + dividendQualify + 1
	matureRecent(statedb, addr, now)

	if held := getHeldAmount(statedb, addr); held.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("expected held 10, got %v", held)
	}
	if holdTime := getHoldingTime(statedb, addr); holdTime != tOld {
		t.Fatalf("expected holding time %d, got %d", tOld, holdTime)
	}
	if head := getRecentHead(statedb, addr); head != 1 {
		t.Fatalf("expected head 1, got %d", head)
	}
	if tail := getRecentTail(statedb, addr); tail != 2 {
		t.Fatalf("expected tail 2, got %d", tail)
	}
	amt, ts := getRecentEntry(statedb, addr, 1)
	if amt.Cmp(big.NewInt(20)) != 0 || ts != tNew {
		t.Fatalf("entry 1 mismatch: amt=%v ts=%d", amt, ts)
	}

	view := GetDividendView(statedb, addr, now)
	if view.EligibleNow.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("expected eligible 10, got %v", view.EligibleNow)
	}
	if view.Pending.Cmp(big.NewInt(20)) != 0 {
		t.Fatalf("expected pending 20, got %v", view.Pending)
	}
}
