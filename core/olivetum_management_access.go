package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

var publicManagementTargets = map[common.Address]struct{}{
	DividendContract: {},
}

func managementAdminFor(addr common.Address) (common.Address, bool) {
	switch addr {
	case BurnContract:
		return BurnAdmin, true
	case params.GasLimitContract:
		return params.GasLimitAdmin, true
	case params.PeriodContract:
		return params.PeriodAdmin, true
	case params.MinTxAmountContract:
		return params.MinTxAmountAdmin, true
	case params.TxRateLimitContract:
		return params.TxRateLimitAdmin, true
	case params.OffSessionTxRateContract:
		return params.OffSessionAdmin, true
	case params.OffSessionMaxPerTxContract:
		return params.OffSessionAdmin, true
	case params.SessionTzContract:
		return params.SessionTzAdmin, true
	default:
		return common.Address{}, false
	}
}

func IsAuthorizedManagementTx(from common.Address, to common.Address) bool {
	if _, ok := publicManagementTargets[to]; ok {
		return true
	}
	if admin, ok := managementAdminFor(to); ok {
		return from == admin
	}
	return true
}
