package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params/types/coregeth"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
	"github.com/ethereum/go-ethereum/params/types/goethereum"
)

var (
	defaultOlivetumhashBlock = big.NewInt(0)
	defaultOlivetumhashCfg   = &ctypes.OlivetumhashConfig{
		EpochLength:        11520,
		DatasetInitBytes:   16 * 1024 * 1024,
		DatasetGrowthBytes: 2 * 1024 * 1024,
		MixRounds:          64,
	}
	// Burn-share was active from genesis on the existing chain, so default fork
	// height is 0 for compatibility.
	defaultBurnShareForkBlock = big.NewInt(0)
)

// ApplyOlivetumDefaults patches the configuration for the Olivetum network so it
// activates Olivetumhash even if the genesis file predates the fork parameters.
func ApplyOlivetumDefaults(cfg ctypes.ChainConfigurator) {
	if cfg == nil {
		return
	}
	chainID := cfg.GetChainID()
	if chainID == nil || chainID.Cmp(olivetumChainID) != 0 {
		return
	}
	if cfg.GetConsensusEngineType() != ctypes.ConsensusEngineT_Olivetumhash {
		_ = cfg.MustSetConsensusEngineType(ctypes.ConsensusEngineT_Olivetumhash)
	}
	ensureOlivetumhashConfig(cfg)
	if cfg.GetOlivetumhashTransition() == nil {
		block := defaultOlivetumhashBlock.Uint64()
		cfg.SetOlivetumhashTransition(&block)
	}
	// Ensure burn-share set for Olivetum.
	if GetBurnShareForkBlock().Sign() == 0 {
		SetBurnShareForkBlock(defaultBurnShareForkBlock)
	}
}

func ensureOlivetumhashConfig(cfg ctypes.ChainConfigurator) {
	switch c := cfg.(type) {
	case *coregeth.CoreGethChainConfig:
		if c.Olivetumhash == nil {
			clone := *defaultOlivetumhashCfg
			c.Olivetumhash = &clone
		}
	case *goethereum.ChainConfig:
		if c.Olivetumhash == nil {
			clone := *defaultOlivetumhashCfg
			c.Olivetumhash = &clone
		}
	}
}
