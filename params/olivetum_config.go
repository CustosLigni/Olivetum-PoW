package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params/types/ctypes"
)

var olivetumChainID = big.NewInt(30216931)

func IsOlivetumConfig(cfg ctypes.ChainConfigurator) bool {
	if cfg == nil {
		return false
	}
	id := cfg.GetChainID()
	return id != nil && id.Cmp(olivetumChainID) == 0
}
