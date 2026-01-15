package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

func ApplyEconomyBaseline(state vm.StateDB, blockNumber *big.Int) {
	if state == nil || blockNumber == nil {
		return
	}
	fork := params.GetEconomyForkBlock()
	if fork.Sign() == 0 || blockNumber.Cmp(fork) != 0 {
		return
	}
	AddTotalBurned(state, params.EconomyBaselineBurnedWei())
	AddTotalBurnedTransfers(state, params.EconomyBaselineBurnedTransfersWei())
	AddTotalBurnedGas(state, params.EconomyBaselineBurnedGasWei())
	AddTotalMinerBurnShare(state, params.EconomyBaselineMinerBurnShareWei())
	AddTotalDividendsMinted(state, params.EconomyBaselineDividendsMintedWei())
}

func MintDividendClaimTip(state vm.StateDB, coinbase common.Address, dividendReward *big.Int, now uint64) *big.Int {
	if state == nil || dividendReward == nil || dividendReward.Sign() == 0 {
		return new(big.Int)
	}
	burnRate := GetBurnRate(state)
	if burnRate == 0 {
		return new(big.Int)
	}
	virtualBurn := new(big.Int).Mul(dividendReward, new(big.Int).SetUint64(burnRate))
	virtualBurn.Div(virtualBurn, big.NewInt(10000))
	if virtualBurn.Sign() == 0 {
		return new(big.Int)
	}
	tip := new(big.Int).Mul(virtualBurn, big.NewInt(int64(MinerBurnShareBps)))
	tip.Div(tip, big.NewInt(10000))
	if tip.Sign() == 0 {
		return new(big.Int)
	}

	minted := GetTotalMinted(state)
	remaining := new(big.Int).Sub(params.MaxSupply(), minted)
	if remaining.Sign() <= 0 {
		return new(big.Int)
	}
	if tip.Cmp(remaining) > 0 {
		tip = remaining
	}
	if tip.Sign() == 0 {
		return new(big.Int)
	}
	SetTotalMinted(state, new(big.Int).Add(minted, tip))
	state.AddBalance(coinbase, uint256.MustFromBig(tip))
	AddHolding(state, coinbase, tip, now)
	return tip
}
