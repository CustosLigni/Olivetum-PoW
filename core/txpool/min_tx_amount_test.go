package txpool

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

func TestTxPoolMinAmountDoesNotExemptAdminRecipientAfterFork(t *testing.T) {
	cfg := newOlivetumConfig(t)
	signer := types.LatestSigner(cfg)
	head := &types.Header{
		Number:   big.NewInt(0),
		Time:     uint64(12 * 3600),
		GasLimit: 15_000_000,
	}
	opts := &ValidationOptions{
		Config:  cfg,
		Accept:  1 << types.LegacyTxType,
		MaxSize: 1024 * 1024,
		MinTip:  new(big.Int),
	}

	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(big.NewInt(10))

	key, _ := crypto.GenerateKey()
	to := params.MinTxAmountAdmin
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: big.NewInt(5), Gas: 21000, GasPrice: big.NewInt(0)})
	if err := ValidateTransaction(tx, head, signer, opts); err != ErrUnderMinAmount {
		t.Fatalf("expected ErrUnderMinAmount, got %v", err)
	}
}

func TestTxPoolMinAmountStillExemptsDividendRecipientAfterFork(t *testing.T) {
	cfg := newOlivetumConfig(t)
	signer := types.LatestSigner(cfg)
	head := &types.Header{
		Number:   big.NewInt(0),
		Time:     uint64(12 * 3600),
		GasLimit: 15_000_000,
	}
	opts := &ValidationOptions{
		Config:  cfg,
		Accept:  1 << types.LegacyTxType,
		MaxSize: 1024 * 1024,
		MinTip:  new(big.Int),
	}

	oldFork := params.GetEconomyForkBlock()
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })
	params.SetEconomyForkBlock(big.NewInt(1))

	oldMin := params.GetMinTxAmount()
	t.Cleanup(func() { params.SetMinTxAmount(oldMin) })
	params.SetMinTxAmount(big.NewInt(10))

	key, _ := crypto.GenerateKey()
	to := core.DividendContract
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: new(big.Int), Gas: 22000, GasPrice: big.NewInt(0), Data: []byte{0x01, 0x02}})
	if err := ValidateTransaction(tx, head, signer, opts); err != nil {
		t.Fatalf("expected acceptance, got %v", err)
	}
}
