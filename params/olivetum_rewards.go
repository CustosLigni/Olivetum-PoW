package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params/vars"
)

var (
	RewardVault = common.HexToAddress("0xA08b7722E58dFAB026C8FaFcfB1f826467f57cb6")

	rewardBaseWei  = new(big.Int).Mul(big.NewInt(12), big.NewInt(vars.Ether))
	rewardFloorWei = new(big.Int).Div(new(big.Int).Mul(big.NewInt(375), big.NewInt(vars.Ether)), big.NewInt(1000))
	maxSupplyWei   = new(big.Int).Mul(big.NewInt(500_000_000), big.NewInt(vars.Ether))

	rewardForkBlock = big.NewInt(0)
)

const RewardHalvingInterval = uint64(8_409_600)

func RewardBase() *big.Int {
	return new(big.Int).Set(rewardBaseWei)
}

func RewardFloor() *big.Int {
	return new(big.Int).Set(rewardFloorWei)
}

func MaxSupply() *big.Int {
	return new(big.Int).Set(maxSupplyWei)
}

func SetRewardForkBlock(block *big.Int) {
	if block == nil {
		rewardForkBlock = big.NewInt(0)
		return
	}
	rewardForkBlock = new(big.Int).Set(block)
}

func GetRewardForkBlock() *big.Int {
	return new(big.Int).Set(rewardForkBlock)
}
