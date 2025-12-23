package params

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params/vars"
)

var (
	MinTxAmountAdmin    = common.HexToAddress("0x17a96Ab66c971e72bb1F9D355792eCEaeaf59af5")
	MinTxAmountContract = common.HexToAddress("0x0000000000000000000000000000000000000b03")
	DividendAddress     = common.HexToAddress("0x000000000000000000000000000000000000d1e1")

	MinTxAmountDefault = new(big.Int).Mul(big.NewInt(10), big.NewInt(vars.Ether))

	MinTxAmountMin = new(big.Int).Div(big.NewInt(vars.Ether), big.NewInt(1000))
	MinTxAmountMax = new(big.Int).Mul(big.NewInt(100), big.NewInt(vars.Ether))

	currentMinTxAmount = new(big.Int).Set(MinTxAmountDefault)

	minTxAmountExempt = map[common.Address]struct{}{
		MinTxAmountAdmin: {},
		DividendAddress:  {},
	}
)

func GetMinTxAmount() *big.Int {
	return new(big.Int).Set(currentMinTxAmount)
}

func SetMinTxAmount(amount *big.Int) {
	currentMinTxAmount.Set(amount)
}

func DecodeMinTxAmount(data []byte) (*big.Int, bool) {
	if len(data) != 8 {
		return nil, false
	}
	val := new(big.Int).SetUint64(binary.BigEndian.Uint64(data))
	val.Mul(val, MinTxAmountMin)
	if val.Cmp(MinTxAmountMin) < 0 || val.Cmp(MinTxAmountMax) > 0 {
		return nil, false
	}
	return val, true
}

func IsMinTxAmountExempt(addr common.Address) bool {
	_, ok := minTxAmountExempt[addr]
	return ok
}
