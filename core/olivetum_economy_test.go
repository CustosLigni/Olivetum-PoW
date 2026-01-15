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

func newOlivetumEVMWithState(t *testing.T, statedb *state.StateDB, blockNumber uint64, blockTime uint64, coinbase common.Address) (*vm.EVM, *GasPool) {
	t.Helper()
	header := &types.Header{
		Number:     new(big.Int).SetUint64(blockNumber),
		Time:       blockTime,
		GasLimit:   15_000_000,
		Difficulty: big.NewInt(1),
		Coinbase:   coinbase,
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
	return evm, gp
}

func newStateDB(t *testing.T) *state.StateDB {
	t.Helper()
	memdb := rawdb.NewMemoryDatabase()
	statedb, err := state.New(types.EmptyRootHash, state.NewDatabase(memdb), nil)
	if err != nil {
		t.Fatalf("new state: %v", err)
	}
	return statedb
}

func TestApplyEconomyBaselineAtForkBlock(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(10))

	statedb := newStateDB(t)
	if GetTotalBurned(statedb).Sign() != 0 {
		t.Fatalf("expected 0 burned")
	}
	ApplyEconomyBaseline(statedb, big.NewInt(9))
	if GetTotalBurned(statedb).Sign() != 0 {
		t.Fatalf("expected 0 burned before fork")
	}
	ApplyEconomyBaseline(statedb, big.NewInt(10))
	if GetTotalBurned(statedb).Cmp(params.EconomyBaselineBurnedWei()) != 0 {
		t.Fatalf("unexpected burned baseline: %v", GetTotalBurned(statedb))
	}
	if GetTotalBurnedTransfers(statedb).Cmp(params.EconomyBaselineBurnedTransfersWei()) != 0 {
		t.Fatalf("unexpected burned transfers baseline: %v", GetTotalBurnedTransfers(statedb))
	}
	if GetTotalBurnedGas(statedb).Sign() != 0 {
		t.Fatalf("expected 0 burned gas baseline")
	}
	if GetTotalMinerBurnShare(statedb).Cmp(params.EconomyBaselineMinerBurnShareWei()) != 0 {
		t.Fatalf("unexpected miner burn share baseline: %v", GetTotalMinerBurnShare(statedb))
	}
	if GetTotalDividendsMinted(statedb).Sign() != 0 {
		t.Fatalf("expected 0 dividends minted baseline")
	}
}

func TestStateTransitionTracksBurnedAfterFork(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	statedb := newStateDB(t)
	coinbase := common.HexToAddress("0xc0")
	blockTime := uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix())
	evm, gp := newOlivetumEVMWithState(t, statedb, 1, blockTime, coinbase)

	from := common.HexToAddress("0x1")
	to := common.HexToAddress("0x2")
	value := new(big.Int).Mul(big.NewInt(100), big.NewInt(vars.Ether))
	fundAccount(statedb, from, new(big.Int).Mul(big.NewInt(1000), big.NewInt(vars.Ether)))
	msg := Message{
		From:      from,
		To:        &to,
		Value:     value,
		GasLimit:  21000,
		GasPrice:  big.NewInt(0),
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Nonce:     0,
	}
	st := NewStateTransition(evm, &msg, gp)
	if _, err := st.TransitionDb(); err != nil {
		t.Fatalf("tx failed: %v", err)
	}

	burnRate := GetBurnRate(statedb)
	burn := new(big.Int).Mul(value, new(big.Int).SetUint64(burnRate))
	burn.Div(burn, big.NewInt(10000))
	minerShare := new(big.Int).Mul(burn, big.NewInt(int64(MinerBurnShareBps)))
	minerShare.Div(minerShare, big.NewInt(10000))
	expected := new(big.Int).Sub(burn, minerShare)

	if GetTotalBurned(statedb).Cmp(expected) != 0 {
		t.Fatalf("unexpected burned total: got %v want %v", GetTotalBurned(statedb), expected)
	}
	if GetTotalBurnedTransfers(statedb).Cmp(expected) != 0 {
		t.Fatalf("unexpected burned transfers total: got %v want %v", GetTotalBurnedTransfers(statedb), expected)
	}
	if GetTotalBurnedGas(statedb).Sign() != 0 {
		t.Fatalf("expected 0 gas burned total, got %v", GetTotalBurnedGas(statedb))
	}
	if GetTotalMinerBurnShare(statedb).Cmp(minerShare) != 0 {
		t.Fatalf("unexpected miner burn share total: got %v want %v", GetTotalMinerBurnShare(statedb), minerShare)
	}
}

func TestStateTransitionTracksBurnedOnGasFeesAfterFork(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(new(big.Int))

	statedb := newStateDB(t)
	coinbase := common.HexToAddress("0xc0ffee")
	blockTime := uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix())
	evm, gp := newOlivetumEVMWithState(t, statedb, 1, blockTime, coinbase)

	SetBurnRate(statedb, 150)

	from := common.HexToAddress("0x1")
	to := common.HexToAddress("0x2")
	fundAccount(statedb, from, etherBig(1000))

	gasPrice := big.NewInt(1000)
	msg := Message{
		From:      from,
		To:        &to,
		Value:     new(big.Int),
		GasLimit:  21000,
		GasPrice:  new(big.Int).Set(gasPrice),
		GasFeeCap: new(big.Int).Set(gasPrice),
		GasTipCap: new(big.Int).Set(gasPrice),
		Nonce:     0,
	}
	st := NewStateTransition(evm, &msg, gp)
	res, err := st.TransitionDb()
	if err != nil {
		t.Fatalf("tx failed: %v", err)
	}

	fee := new(big.Int).Mul(new(big.Int).SetUint64(res.UsedGas), gasPrice)
	burn := new(big.Int).Mul(fee, big.NewInt(150))
	burn.Div(burn, big.NewInt(10000))
	minerShare := new(big.Int).Mul(burn, big.NewInt(int64(MinerBurnShareBps)))
	minerShare.Div(minerShare, big.NewInt(10000))
	expectedBurned := new(big.Int).Sub(burn, minerShare)

	if GetTotalBurned(statedb).Cmp(expectedBurned) != 0 {
		t.Fatalf("unexpected burned total: got %v want %v", GetTotalBurned(statedb), expectedBurned)
	}
	if GetTotalBurnedTransfers(statedb).Sign() != 0 {
		t.Fatalf("expected 0 burned transfers total, got %v", GetTotalBurnedTransfers(statedb))
	}
	if GetTotalBurnedGas(statedb).Cmp(expectedBurned) != 0 {
		t.Fatalf("unexpected burned gas total: got %v want %v", GetTotalBurnedGas(statedb), expectedBurned)
	}
	if GetTotalMinerBurnShare(statedb).Cmp(minerShare) != 0 {
		t.Fatalf("unexpected miner burn share total: got %v want %v", GetTotalMinerBurnShare(statedb), minerShare)
	}

	expectedCoinbase := new(big.Int).Sub(fee, expectedBurned)
	if statedb.GetBalance(coinbase).ToBig().Cmp(expectedCoinbase) != 0 {
		t.Fatalf("unexpected coinbase balance: got %v want %v", statedb.GetBalance(coinbase).ToBig(), expectedCoinbase)
	}

	if tail := getRecentTail(statedb, coinbase); tail != 1 {
		t.Fatalf("expected one coinbase holding, got tail=%d", tail)
	}
	amt, ts := getRecentEntry(statedb, coinbase, 0)
	if amt.Cmp(expectedCoinbase) != 0 || ts != blockTime {
		t.Fatalf("unexpected coinbase holding: amt=%v ts=%d", amt, ts)
	}
}

func TestStateTransitionDoesNotBurnGasFeesBeforeFork(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(10))

	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(new(big.Int))

	statedb := newStateDB(t)
	coinbase := common.HexToAddress("0xc0ffee")
	blockTime := uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix())
	evm, gp := newOlivetumEVMWithState(t, statedb, 1, blockTime, coinbase)

	SetBurnRate(statedb, 150)

	from := common.HexToAddress("0x1")
	to := common.HexToAddress("0x2")
	fundAccount(statedb, from, etherBig(1000))

	gasPrice := big.NewInt(1000)
	msg := Message{
		From:      from,
		To:        &to,
		Value:     new(big.Int),
		GasLimit:  21000,
		GasPrice:  new(big.Int).Set(gasPrice),
		GasFeeCap: new(big.Int).Set(gasPrice),
		GasTipCap: new(big.Int).Set(gasPrice),
		Nonce:     0,
	}
	st := NewStateTransition(evm, &msg, gp)
	res, err := st.TransitionDb()
	if err != nil {
		t.Fatalf("tx failed: %v", err)
	}

	fee := new(big.Int).Mul(new(big.Int).SetUint64(res.UsedGas), gasPrice)
	if statedb.GetBalance(coinbase).ToBig().Cmp(fee) != 0 {
		t.Fatalf("unexpected coinbase balance: got %v want %v", statedb.GetBalance(coinbase).ToBig(), fee)
	}
	if GetTotalBurned(statedb).Sign() != 0 {
		t.Fatalf("expected 0 burned before fork, got %v", GetTotalBurned(statedb))
	}
	if GetTotalBurnedTransfers(statedb).Sign() != 0 {
		t.Fatalf("expected 0 burned transfers before fork, got %v", GetTotalBurnedTransfers(statedb))
	}
	if GetTotalBurnedGas(statedb).Sign() != 0 {
		t.Fatalf("expected 0 burned gas before fork, got %v", GetTotalBurnedGas(statedb))
	}
	if GetTotalMinerBurnShare(statedb).Sign() != 0 {
		t.Fatalf("expected 0 miner burn share before fork, got %v", GetTotalMinerBurnShare(statedb))
	}
}

func TestDividendClaimMintsMinerTipAndTracksDividendMint(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	statedb := newStateDB(t)
	coinbase := common.HexToAddress("0xc0ffee")
	now := uint64(time.Now().Unix())
	evm, gp := newOlivetumEVMWithState(t, statedb, 1, now, coinbase)

	SetBurnRate(statedb, 150)
	if !TriggerDividend(statedb, now) {
		t.Fatalf("expected dividend round trigger")
	}

	claimer := common.HexToAddress("0xabc")
	startBal := new(big.Int).Mul(big.NewInt(1000), big.NewInt(vars.Ether))
	statedb.AddBalance(claimer, uint256.MustFromBig(startBal))

	msg := Message{
		From:      claimer,
		To:        &DividendContract,
		Value:     new(big.Int),
		GasLimit:  21000,
		GasPrice:  big.NewInt(0),
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Nonce:     0,
	}
	st := NewStateTransition(evm, &msg, gp)
	if _, err := st.TransitionDb(); err != nil {
		t.Fatalf("claim tx failed: %v", err)
	}

	rate := GetDividendRate(statedb)
	reward := new(big.Int).Mul(startBal, new(big.Int).SetUint64(rate))
	reward.Div(reward, big.NewInt(10000))
	if GetTotalDividendsMinted(statedb).Cmp(reward) != 0 {
		t.Fatalf("unexpected dividends minted: got %v want %v", GetTotalDividendsMinted(statedb), reward)
	}

	virtualBurn := new(big.Int).Mul(reward, big.NewInt(150))
	virtualBurn.Div(virtualBurn, big.NewInt(10000))
	tip := new(big.Int).Mul(virtualBurn, big.NewInt(int64(MinerBurnShareBps)))
	tip.Div(tip, big.NewInt(10000))

	if statedb.GetBalance(coinbase).ToBig().Cmp(tip) != 0 {
		t.Fatalf("unexpected miner tip balance: got %v want %v", statedb.GetBalance(coinbase).ToBig(), tip)
	}
	if GetTotalMinted(statedb).Cmp(tip) != 0 {
		t.Fatalf("unexpected total minted: got %v want %v", GetTotalMinted(statedb), tip)
	}
}

func TestOffSessionBudgetCountsCumulativeValue(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldCap := params.GetOffSessionMaxPerTx()
	t.Cleanup(func() { params.SetOffSessionMaxPerTx(oldCap) })
	params.SetOffSessionMaxPerTx(new(big.Int).Mul(big.NewInt(10), big.NewInt(vars.Ether)))

	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(new(big.Int).Mul(big.NewInt(1), big.NewInt(vars.Ether)))

	oldTz := params.GetSessionTzOffsetSeconds()
	t.Cleanup(func() { params.SetSessionTzOffsetSeconds(oldTz) })
	params.SetSessionTzOffsetSeconds(0)

	statedb := newStateDB(t)
	coinbase := common.HexToAddress("0xc0")
	ts := uint64(time.Date(2024, time.March, 3, 10, 0, 0, 0, time.UTC).Unix())
	evm, gp := newOlivetumEVMWithState(t, statedb, 1, ts, coinbase)

	from := common.HexToAddress("0x1")
	to := common.HexToAddress("0x2")
	statedb.AddBalance(from, uint256.MustFromBig(new(big.Int).Mul(big.NewInt(100), big.NewInt(vars.Ether))))

	msg1 := Message{
		From:      from,
		To:        &to,
		Value:     new(big.Int).Mul(big.NewInt(7), big.NewInt(vars.Ether)),
		GasLimit:  21000,
		GasPrice:  big.NewInt(0),
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Nonce:     0,
	}
	st1 := NewStateTransition(evm, &msg1, gp)
	if _, err := st1.TransitionDb(); err != nil {
		t.Fatalf("tx1 failed: %v", err)
	}

	evm2, gp2 := newOlivetumEVMWithState(t, statedb, 1, ts+60, coinbase)
	msg2 := Message{
		From:      from,
		To:        &to,
		Value:     new(big.Int).Mul(big.NewInt(5), big.NewInt(vars.Ether)),
		GasLimit:  21000,
		GasPrice:  big.NewInt(0),
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Nonce:     1,
	}
	st2 := NewStateTransition(evm2, &msg2, gp2)
	if _, err := st2.TransitionDb(); err == nil || err.Error() != ErrOverMaxOffSessionBudget.Error() {
		t.Fatalf("expected ErrOverMaxOffSessionBudget, got %v", err)
	}
}

func TestOffSessionBudgetDoesNotResetAcrossSundayToMonday(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldCap := params.GetOffSessionMaxPerTx()
	t.Cleanup(func() { params.SetOffSessionMaxPerTx(oldCap) })
	params.SetOffSessionMaxPerTx(new(big.Int).Mul(big.NewInt(10), big.NewInt(vars.Ether)))

	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(new(big.Int).Mul(big.NewInt(1), big.NewInt(vars.Ether)))

	oldTz := params.GetSessionTzOffsetSeconds()
	t.Cleanup(func() { params.SetSessionTzOffsetSeconds(oldTz) })
	params.SetSessionTzOffsetSeconds(0)

	statedb := newStateDB(t)
	coinbase := common.HexToAddress("0xc0")
	from := common.HexToAddress("0x1")
	to := common.HexToAddress("0x2")
	statedb.AddBalance(from, uint256.MustFromBig(new(big.Int).Mul(big.NewInt(100), big.NewInt(vars.Ether))))

	sun := uint64(time.Date(2024, time.March, 3, 10, 0, 0, 0, time.UTC).Unix())
	evm1, gp1 := newOlivetumEVMWithState(t, statedb, 1, sun, coinbase)
	msg1 := Message{
		From:      from,
		To:        &to,
		Value:     new(big.Int).Mul(big.NewInt(7), big.NewInt(vars.Ether)),
		GasLimit:  21000,
		GasPrice:  big.NewInt(0),
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Nonce:     0,
	}
	st1 := NewStateTransition(evm1, &msg1, gp1)
	if _, err := st1.TransitionDb(); err != nil {
		t.Fatalf("tx1 failed: %v", err)
	}

	mon := uint64(time.Date(2024, time.March, 4, 10, 0, 0, 0, time.UTC).Unix())
	evm2, gp2 := newOlivetumEVMWithState(t, statedb, 1, mon, coinbase)
	msg2 := Message{
		From:      from,
		To:        &to,
		Value:     new(big.Int).Mul(big.NewInt(7), big.NewInt(vars.Ether)),
		GasLimit:  21000,
		GasPrice:  big.NewInt(0),
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Nonce:     1,
	}
	st2 := NewStateTransition(evm2, &msg2, gp2)
	if _, err := st2.TransitionDb(); err == nil || err.Error() != ErrOverMaxOffSessionBudget.Error() {
		t.Fatalf("expected ErrOverMaxOffSessionBudget, got %v", err)
	}
}

func TestOffSessionBudgetResetsAfterSessionToNextOffSession(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldCap := params.GetOffSessionMaxPerTx()
	t.Cleanup(func() { params.SetOffSessionMaxPerTx(oldCap) })
	params.SetOffSessionMaxPerTx(new(big.Int).Mul(big.NewInt(10), big.NewInt(vars.Ether)))

	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(new(big.Int).Mul(big.NewInt(1), big.NewInt(vars.Ether)))

	oldTz := params.GetSessionTzOffsetSeconds()
	t.Cleanup(func() { params.SetSessionTzOffsetSeconds(oldTz) })
	params.SetSessionTzOffsetSeconds(0)

	statedb := newStateDB(t)
	coinbase := common.HexToAddress("0xc0")
	from := common.HexToAddress("0x1")
	to := common.HexToAddress("0x2")
	statedb.AddBalance(from, uint256.MustFromBig(new(big.Int).Mul(big.NewInt(100), big.NewInt(vars.Ether))))

	sun := uint64(time.Date(2024, time.March, 3, 10, 0, 0, 0, time.UTC).Unix())
	evm1, gp1 := newOlivetumEVMWithState(t, statedb, 1, sun, coinbase)
	msg1 := Message{
		From:      from,
		To:        &to,
		Value:     new(big.Int).Mul(big.NewInt(7), big.NewInt(vars.Ether)),
		GasLimit:  21000,
		GasPrice:  big.NewInt(0),
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Nonce:     0,
	}
	st1 := NewStateTransition(evm1, &msg1, gp1)
	if _, err := st1.TransitionDb(); err != nil {
		t.Fatalf("tx1 failed: %v", err)
	}

	mon := uint64(time.Date(2024, time.March, 4, 10, 0, 0, 0, time.UTC).Unix())
	evm2, gp2 := newOlivetumEVMWithState(t, statedb, 1, mon, coinbase)
	msg2 := Message{
		From:      from,
		To:        &to,
		Value:     new(big.Int).Mul(big.NewInt(3), big.NewInt(vars.Ether)),
		GasLimit:  21000,
		GasPrice:  big.NewInt(0),
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Nonce:     1,
	}
	st2 := NewStateTransition(evm2, &msg2, gp2)
	if _, err := st2.TransitionDb(); err != nil {
		t.Fatalf("tx2 failed: %v", err)
	}

	tue := uint64(time.Date(2024, time.March, 5, 10, 0, 0, 0, time.UTC).Unix())
	evm3, gp3 := newOlivetumEVMWithState(t, statedb, 1, tue, coinbase)
	msg3 := Message{
		From:      from,
		To:        &to,
		Value:     new(big.Int).Mul(big.NewInt(7), big.NewInt(vars.Ether)),
		GasLimit:  21000,
		GasPrice:  big.NewInt(0),
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Nonce:     2,
	}
	st3 := NewStateTransition(evm3, &msg3, gp3)
	if _, err := st3.TransitionDb(); err != nil {
		t.Fatalf("tx3 failed: %v", err)
	}
}
