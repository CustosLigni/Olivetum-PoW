package params

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params/vars"
)

var (
	// GasLimitAdmin is the privileged account that can update the block gas
	// limit and block period at runtime. It defaults to the same address used
	// for other on-chain administration like the burn and dividend modules.
	GasLimitAdmin    = common.HexToAddress("0x17a96Ab66c971e72bb1F9D355792eCEaeaf59af5")
	GasLimitContract = common.HexToAddress("0x0000000000000000000000000000000000000b01")
	PeriodAdmin      = GasLimitAdmin
	PeriodContract   = common.HexToAddress("0x0000000000000000000000000000000000000b02")

	currentGasLimit = vars.GenesisGasLimit
	// default to the clique genesis period so blocks continue sealing even
	// without runtime configuration transactions
	currentPeriod uint64 = 15

	// MaxReorgDepth defines the maximum number of blocks a canonical reorg is
	// allowed to roll back for Olivetum. Reorgs deeper than this are rejected.
	// This is an Olivetum-only consensus rule.
	MaxReorgDepth uint64 = 75

	// MaxForwardGap defines the maximum number of blocks the external chain
	// tip may be ahead of the local tip while still considering a reorg.
	// Olivetum-only rule to prevent large "forward jumps" from hidden mining.
	MaxForwardGap uint64 = 75

	// ReorgGuardDisableBlock is the block height after which custom Olivetum
	// reorg guards (depth/forward-gap/finality/MESS) are disabled to allow
	// unrestricted fork-choice.
	// Guards remain active before this height.
	ReorgGuardDisableBlock uint64 = 1400
)

func GetGasLimit() uint64      { return currentGasLimit }
func SetGasLimit(limit uint64) { currentGasLimit = limit }
func DecodeGasLimit(data []byte) (uint64, bool) {
	if len(data) != 1 {
		return 0, false
	}
	return uint64(data[0]) * 1000000, true
}

func GetBlockPeriod() uint64       { return currentPeriod }
func SetBlockPeriod(period uint64) { currentPeriod = period }
func DecodeBlockPeriod(data []byte) (uint64, bool) {
	if len(data) != 1 {
		return 0, false
	}
	period := uint64(data[0])
	if period == 0 || period > 60 {
		return 0, false
	}
	return period, true
}
