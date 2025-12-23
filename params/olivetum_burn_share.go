package params

import "math/big"

// Burn share fork height. At and after this block, a portion of the computed
// burn is redirected to the miner.
var burnShareForkBlock = big.NewInt(0)

func SetBurnShareForkBlock(block *big.Int) {
	if block == nil {
		burnShareForkBlock = big.NewInt(0)
		return
	}
	burnShareForkBlock = new(big.Int).Set(block)
}

func GetBurnShareForkBlock() *big.Int {
	return new(big.Int).Set(burnShareForkBlock)
}
