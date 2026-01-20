package core

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/types/coregeth"
	"github.com/ethereum/go-ethereum/params/vars"
	"github.com/holiman/uint256"
)

func newOlivetumEnv(t *testing.T, blockTime uint64) (*vm.EVM, *state.StateDB, *GasPool) {
	t.Helper()
	memdb := rawdb.NewMemoryDatabase()
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabase(memdb), nil)

	header := &types.Header{
		Number:     big.NewInt(1),
		Time:       blockTime,
		GasLimit:   15_000_000,
		Difficulty: big.NewInt(1),
	}
	cfg := &coregeth.CoreGethChainConfig{}
	if err := cfg.SetChainID(big.NewInt(30216931)); err != nil {
		t.Fatalf("set chain id: %v", err)
	}
	params.ApplyOlivetumDefaults(cfg)
	blockCtx := vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash: func(uint64) common.Hash {
			return common.Hash{}
		},
		Coinbase:    header.Coinbase,
		BlockNumber: new(big.Int).Set(header.Number),
		Time:        header.Time,
		Difficulty:  new(big.Int).Set(header.Difficulty),
		GasLimit:    header.GasLimit,
	}
	evm := vm.NewEVM(blockCtx, vm.TxContext{}, statedb, cfg, vm.Config{})
	gp := new(GasPool).AddGas(header.GasLimit)
	return evm, statedb, gp
}

func fundedMessage(from common.Address, to *common.Address, value *big.Int) Message {
	return Message{
		From:      from,
		To:        to,
		Value:     value,
		GasLimit:  21000,
		GasPrice:  big.NewInt(1),
		GasFeeCap: big.NewInt(1),
		GasTipCap: big.NewInt(1),
		Nonce:     0,
	}
}

func fundAccount(state *state.StateDB, addr common.Address, amount *big.Int) {
	state.AddBalance(addr, uint256.MustFromBig(amount))
}

func runTx(t *testing.T, blockTime uint64, msg Message) error {
	t.Helper()
	evm, statedb, gp := newOlivetumEnv(t, blockTime)
	fundAccount(statedb, msg.From, etherBig(1000))
	st := NewStateTransition(evm, &msg, gp)
	_, err := st.TransitionDb()
	return err
}

func etherBig(mult int64) *big.Int {
	unit := new(big.Int).SetUint64(vars.Ether)
	return new(big.Int).Mul(big.NewInt(mult), unit)
}

func TestStateTransitionRejectsSelfTransfer(t *testing.T) {
	from := common.HexToAddress("0x1")
	AllowSelfTransfersForTesting(false)
	t.Cleanup(func() { AllowSelfTransfersForTesting(false) })
	msg := fundedMessage(from, &from, etherBig(1))
	err := runTx(t, uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix()), msg)
	if err == nil || err.Error() != ErrSelfTransfer.Error() {
		t.Fatalf("expected ErrSelfTransfer, got %v", err)
	}
}

func TestStateTransitionEnforcesMinimumAmount(t *testing.T) {
	min := etherBig(5)
	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(min)

	from := common.HexToAddress("0x2")
	to := common.HexToAddress("0x3")
	msg := fundedMessage(from, &to, etherBig(1))
	err := runTx(t, uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix()), msg)
	if err == nil || err.Error() != "transaction value below minimum" {
		t.Fatalf("expected minimum amount error, got %v", err)
	}
}

func TestStateTransitionAllowsUnderMinToAdminBeforeFork(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(10))

	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(etherBig(10))

	from := common.HexToAddress("0x2")
	to := params.MinTxAmountAdmin
	msg := fundedMessage(from, &to, etherBig(5))
	err := runTx(t, uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix()), msg)
	if err != nil {
		t.Fatalf("expected under-min to admin accepted before fork, got %v", err)
	}
}

func TestStateTransitionRejectsUnderMinToAdminAfterFork(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(etherBig(10))

	from := common.HexToAddress("0x2")
	to := params.MinTxAmountAdmin
	msg := fundedMessage(from, &to, etherBig(5))
	err := runTx(t, uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix()), msg)
	if err == nil || err.Error() != "transaction value below minimum" {
		t.Fatalf("expected minimum amount error, got %v", err)
	}
}

func TestStateTransitionAllowsZeroValueToDividendContractAfterFork(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(etherBig(10))

	from := common.HexToAddress("0x2")
	target := DividendContract
	msg := Message{
		From:      from,
		To:        &target,
		Value:     new(big.Int),
		GasLimit:  22000,
		GasPrice:  big.NewInt(1),
		GasFeeCap: big.NewInt(1),
		GasTipCap: big.NewInt(1),
		Nonce:     0,
		Data:      []byte{0x01, 0x02},
	}
	err := runTx(t, uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix()), msg)
	if err != nil {
		t.Fatalf("expected zero-value dividend tx allowed after fork, got %v", err)
	}
}

func TestStateTransitionEnforcesOffSessionCap(t *testing.T) {
	oldCap := params.GetOffSessionMaxPerTx()
	t.Cleanup(func() { params.SetOffSessionMaxPerTx(oldCap) })
	params.SetOffSessionMaxPerTx(etherBig(2))
	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(etherBig(1))
	oldTz := params.GetSessionTzOffsetSeconds()
	t.Cleanup(func() { params.SetSessionTzOffsetSeconds(oldTz) })
	params.SetSessionTzOffsetSeconds(0)

	from := common.HexToAddress("0x4")
	to := common.HexToAddress("0x5")
	msg := fundedMessage(from, &to, etherBig(5))
	// Sunday 10:00 UTC -> off-session
	ts := uint64(time.Date(2024, time.March, 3, 10, 0, 0, 0, time.UTC).Unix())
	err := runTx(t, ts, msg)
	if err == nil || err.Error() != ErrOverMaxOffSession.Error() {
		t.Fatalf("expected ErrOverMaxOffSession, got %v", err)
	}
}

func TestStateTransitionBlocksUnauthorizedManagementTx(t *testing.T) {
	from := common.HexToAddress("0x6")
	target := params.MinTxAmountContract
	msg := fundedMessage(from, &target, big.NewInt(0))
	err := runTx(t, uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix()), msg)
	if err == nil || err.Error() != ErrUnauthorizedManagementTx.Error() {
		t.Fatalf("expected ErrUnauthorizedManagementTx, got %v", err)
	}
}

func TestStateTransitionRateLimitsAdminTransfersAfterFork(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldLimit := params.GetTxRateLimit()
	t.Cleanup(func() { params.SetTxRateLimit(oldLimit) })
	params.SetTxRateLimit(1)

	admin := params.TxRateLimitAdmin
	to := common.HexToAddress("0x100")
	ts := uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix())

	evm, statedb, gp := newOlivetumEnv(t, ts)
	fundAccount(statedb, admin, etherBig(1000))

	usage := GetTxRateUsage(statedb, admin)
	usage.Count = params.GetTxRateLimit()
	usage.Start = ts
	usage.Epoch = GetTxRateEpoch(statedb)
	SetTxRateUsage(statedb, admin, usage)

	msg := fundedMessage(admin, &to, etherBig(1))
	st := NewStateTransition(evm, &msg, gp)
	if _, err := st.TransitionDb(); err == nil || err.Error() != ErrRateLimit.Error() {
		t.Fatalf("expected ErrRateLimit, got %v", err)
	}

	target := params.TxRateLimitContract
	adminMsg := Message{
		From:      admin,
		To:        &target,
		Value:     new(big.Int),
		GasLimit:  30000,
		GasPrice:  big.NewInt(1),
		GasFeeCap: big.NewInt(1),
		GasTipCap: big.NewInt(1),
		Nonce:     0,
		Data:      []byte{0x01},
	}
	st2 := NewStateTransition(evm, &adminMsg, gp)
	if _, err := st2.TransitionDb(); err != nil {
		t.Fatalf("admin management tx failed: %v", err)
	}
}

func TestStateTransitionDoesNotRateLimitAdminBeforeFork(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(10))

	oldLimit := params.GetTxRateLimit()
	t.Cleanup(func() { params.SetTxRateLimit(oldLimit) })
	params.SetTxRateLimit(1)

	admin := params.TxRateLimitAdmin
	to := common.HexToAddress("0x101")
	ts := uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix())

	evm, statedb, gp := newOlivetumEnv(t, ts)
	fundAccount(statedb, admin, etherBig(1000))

	usage := GetTxRateUsage(statedb, admin)
	usage.Count = params.GetTxRateLimit()
	usage.Start = ts
	usage.Epoch = GetTxRateEpoch(statedb)
	SetTxRateUsage(statedb, admin, usage)

	msg := fundedMessage(admin, &to, etherBig(1))
	st := NewStateTransition(evm, &msg, gp)
	if _, err := st.TransitionDb(); err != nil {
		t.Fatalf("expected admin transfer acceptance before fork, got %v", err)
	}
}

func TestStateTransitionAdminCanTriggerAndClaimDividendWithRateLimitOne(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldLimit := params.GetTxRateLimit()
	t.Cleanup(func() { params.SetTxRateLimit(oldLimit) })
	params.SetTxRateLimit(1)

	prevDividend := currentDividend
	t.Cleanup(func() { currentDividend = prevDividend })

	admin := params.TxRateLimitAdmin
	ts := uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix())

	evm, statedb, gp := newOlivetumEnv(t, ts)
	fundAccount(statedb, admin, etherBig(1000))

	target := DividendContract
	triggerMsg := Message{
		From:      admin,
		To:        &target,
		Value:     new(big.Int),
		GasLimit:  30000,
		GasPrice:  big.NewInt(1),
		GasFeeCap: big.NewInt(1),
		GasTipCap: big.NewInt(1),
		Nonce:     0,
		Data:      []byte{0x00},
	}
	st := NewStateTransition(evm, &triggerMsg, gp)
	if _, err := st.TransitionDb(); err != nil {
		t.Fatalf("trigger dividend failed: %v", err)
	}

	if usage := GetTxRateUsage(statedb, admin); usage.Count != 0 {
		t.Fatalf("expected trigger tx to be exempt from rate limit, got count=%d", usage.Count)
	}

	claimMsg := Message{
		From:      admin,
		To:        &target,
		Value:     new(big.Int),
		GasLimit:  30000,
		GasPrice:  big.NewInt(1),
		GasFeeCap: big.NewInt(1),
		GasTipCap: big.NewInt(1),
		Nonce:     1,
	}
	st2 := NewStateTransition(evm, &claimMsg, gp)
	if _, err := st2.TransitionDb(); err != nil {
		t.Fatalf("claim dividend failed: %v", err)
	}

	if usage := GetTxRateUsage(statedb, admin); usage.Count != 1 {
		t.Fatalf("expected claim tx to consume one rate-limit slot, got count=%d", usage.Count)
	}
	if status := GetDividendStatus(statedb, admin); !status.Claimed {
		t.Fatalf("expected dividend to be marked claimed for admin")
	}
}
