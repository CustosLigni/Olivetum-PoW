package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

func ValidateOlivetumTxPayload(from common.Address, to common.Address, value *big.Int, data []byte, accessList types.AccessList, economyForkActive bool) error {
	if !economyForkActive {
		return nil
	}
	if len(accessList) > 0 {
		return ErrTxAccessListNotAllowed
	}

	switch to {
	case DividendContract:
		if value.Sign() != 0 {
			return ErrTxValueNotAllowed
		}
		if len(data) == 0 {
			return nil
		}
		if len(data) == 1 && from == DividendAdmin {
			return nil
		}
		return ErrTxDataNotAllowed
	case BurnContract:
		if value.Sign() != 0 {
			return ErrTxValueNotAllowed
		}
		if len(data) != 1 {
			return ErrTxDataLengthInvalid
		}
		return nil
	case params.GasLimitContract, params.PeriodContract, params.TxRateLimitContract, params.OffSessionTxRateContract:
		if value.Sign() != 0 {
			return ErrTxValueNotAllowed
		}
		if len(data) != 1 {
			return ErrTxDataLengthInvalid
		}
		return nil
	case params.MinTxAmountContract, params.OffSessionMaxPerTxContract:
		if value.Sign() != 0 {
			return ErrTxValueNotAllowed
		}
		if len(data) != 8 {
			return ErrTxDataLengthInvalid
		}
		return nil
	case params.SessionTzContract:
		if value.Sign() != 0 {
			return ErrTxValueNotAllowed
		}
		if len(data) != 4 {
			return ErrTxDataLengthInvalid
		}
		return nil
	default:
		if len(data) != 0 {
			return ErrTxDataNotAllowed
		}
		return nil
	}
}
