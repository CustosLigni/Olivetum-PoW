package params

import "math/big"

var economyForkBlock = big.NewInt(260000)

func SetEconomyForkBlock(block *big.Int) {
	if block == nil {
		economyForkBlock = big.NewInt(0)
		return
	}
	economyForkBlock = new(big.Int).Set(block)
}

func GetEconomyForkBlock() *big.Int {
	return new(big.Int).Set(economyForkBlock)
}
