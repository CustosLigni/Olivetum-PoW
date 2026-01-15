package txpool

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

func TestTxPoolOffSessionBudgetRejectsCumulativeSpend(t *testing.T) {
	pool, chain, statedb, signer := newTestPool(t)
	configureRuntime(t, statedb, 1000)

	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldOffRate := params.GetOffSessionTxRate()
	t.Cleanup(func() { params.SetOffSessionTxRate(oldOffRate) })
	params.SetOffSessionTxRate(1000)

	oldCap := params.GetOffSessionMaxPerTx()
	t.Cleanup(func() { params.SetOffSessionMaxPerTx(oldCap) })
	params.SetOffSessionMaxPerTx(big.NewInt(10))

	chain.head.Time = 0
	chain.head.Number = big.NewInt(0)

	key, _ := crypto.GenerateKey()
	to := common.HexToAddress("0x0000000000000000000000000000000000000001")

	tx1 := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: big.NewInt(7), Gas: 21000, GasPrice: big.NewInt(0)})
	if err := pool.Add([]*types.Transaction{tx1}, true, true)[0]; err != nil {
		t.Fatalf("tx1 rejected: %v", err)
	}

	tx2 := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 1, To: &to, Value: big.NewInt(5), Gas: 21000, GasPrice: big.NewInt(0)})
	if err := pool.Add([]*types.Transaction{tx2}, true, true)[0]; err != ErrOverMaxOffSessionBudget {
		t.Fatalf("expected ErrOverMaxOffSessionBudget, got %v", err)
	}
}

func TestTxPoolOffSessionBudgetAllowsExactLimit(t *testing.T) {
	pool, chain, statedb, signer := newTestPool(t)
	configureRuntime(t, statedb, 1000)

	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldOffRate := params.GetOffSessionTxRate()
	t.Cleanup(func() { params.SetOffSessionTxRate(oldOffRate) })
	params.SetOffSessionTxRate(1000)

	oldCap := params.GetOffSessionMaxPerTx()
	t.Cleanup(func() { params.SetOffSessionMaxPerTx(oldCap) })
	params.SetOffSessionMaxPerTx(big.NewInt(10))

	chain.head.Time = 0
	chain.head.Number = big.NewInt(0)

	key, _ := crypto.GenerateKey()
	to := common.HexToAddress("0x0000000000000000000000000000000000000001")

	tx1 := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: big.NewInt(7), Gas: 21000, GasPrice: big.NewInt(0)})
	if err := pool.Add([]*types.Transaction{tx1}, true, true)[0]; err != nil {
		t.Fatalf("tx1 rejected: %v", err)
	}

	tx2 := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 1, To: &to, Value: big.NewInt(3), Gas: 21000, GasPrice: big.NewInt(0)})
	if err := pool.Add([]*types.Transaction{tx2}, true, true)[0]; err != nil {
		t.Fatalf("tx2 rejected: %v", err)
	}
}
