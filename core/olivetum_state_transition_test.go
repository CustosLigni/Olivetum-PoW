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
