package txpool

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

func TestTxPoolRejectsCalldataForRegularTransfersAfterFork(t *testing.T) {
	cfg := newOlivetumConfig(t)
	signer := types.LatestSigner(cfg)
	head := &types.Header{
		Number:   big.NewInt(0),
		Time:     uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix()),
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
	params.SetMinTxAmount(new(big.Int))

	key, _ := crypto.GenerateKey()
	to := common.HexToAddress("0x0000000000000000000000000000000000000002")
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: big.NewInt(1), Gas: 22000, GasPrice: big.NewInt(0), Data: []byte{0x01}})
	if err := ValidateTransaction(tx, head, signer, opts); err != ErrTxDataNotAllowed {
		t.Fatalf("expected ErrTxDataNotAllowed, got %v", err)
	}
}

func TestTxPoolRejectsCalldataForDividendClaimsAfterFork(t *testing.T) {
	cfg := newOlivetumConfig(t)
	signer := types.LatestSigner(cfg)
	head := &types.Header{
		Number:   big.NewInt(0),
		Time:     uint64(time.Date(2024, time.March, 4, 13, 0, 0, 0, time.UTC).Unix()),
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
	params.SetMinTxAmount(new(big.Int))

	key, _ := crypto.GenerateKey()
	to := core.DividendContract
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{Nonce: 0, To: &to, Value: new(big.Int), Gas: 22000, GasPrice: big.NewInt(0), Data: []byte{0x01}})
	if err := ValidateTransaction(tx, head, signer, opts); err != ErrTxDataNotAllowed {
		t.Fatalf("expected ErrTxDataNotAllowed, got %v", err)
	}
}
