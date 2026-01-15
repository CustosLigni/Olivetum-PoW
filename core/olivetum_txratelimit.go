package core

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

var (
	txRateLimitSlot = common.Hash{}
	txRateEpochSlot = common.BigToHash(big.NewInt(1))
)

func txRateSlot(addr common.Address, index byte) common.Hash {
	var buf [64]byte
	copy(buf[12:32], addr.Bytes())
	buf[63] = index
	return crypto.Keccak256Hash(buf[:])
}

type TxRateUsage struct {
	Count uint64
	Start uint64
	Epoch uint64
}

func loadTxRateEpoch(s vm.StateDB) uint64 {
	return s.GetState(params.TxRateLimitContract, txRateEpochSlot).Big().Uint64()
}

func GetTxRateEpoch(s vm.StateDB) uint64 {
	return loadTxRateEpoch(s)
}

func setTxRateEpoch(s vm.StateDB, epoch uint64) {
	ensureTxRateAccount(s)
	s.SetState(params.TxRateLimitContract, txRateEpochSlot, common.BigToHash(new(big.Int).SetUint64(epoch)))
}

func GetTxRateUsage(s vm.StateDB, addr common.Address) TxRateUsage {
	count := s.GetState(params.TxRateLimitContract, txRateSlot(addr, 0)).Big().Uint64()
	start := s.GetState(params.TxRateLimitContract, txRateSlot(addr, 1)).Big().Uint64()
	epoch := s.GetState(params.TxRateLimitContract, txRateSlot(addr, 2)).Big().Uint64()
	return TxRateUsage{Count: count, Start: start, Epoch: epoch}
}

func SetTxRateUsage(s vm.StateDB, addr common.Address, u TxRateUsage) {
	ensureTxRateAccount(s)
	s.SetState(params.TxRateLimitContract, txRateSlot(addr, 0), common.BigToHash(new(big.Int).SetUint64(u.Count)))
	s.SetState(params.TxRateLimitContract, txRateSlot(addr, 1), common.BigToHash(new(big.Int).SetUint64(u.Start)))
	s.SetState(params.TxRateLimitContract, txRateSlot(addr, 2), common.BigToHash(new(big.Int).SetUint64(u.Epoch)))
}

func ClearTxRateUsage(s vm.StateDB, addr common.Address) {
	zero := common.Hash{}
	ensureTxRateAccount(s)
	s.SetState(params.TxRateLimitContract, txRateSlot(addr, 0), zero)
	s.SetState(params.TxRateLimitContract, txRateSlot(addr, 1), zero)
	s.SetState(params.TxRateLimitContract, txRateSlot(addr, 2), zero)
}

func GetTxAllowance(s vm.StateDB, addr common.Address, now uint64) uint64 {
	limit := params.GetTxRateLimit()
	if !IsSession(now) {
		limit = params.GetOffSessionTxRate()
	}
	epoch := loadTxRateEpoch(s)
	u := GetTxRateUsage(s, addr)
	if u.Epoch != epoch || now-u.Start >= uint64(time.Hour/time.Second) {
		return limit
	}
	if u.Count >= limit {
		return 0
	}
	return limit - u.Count
}

func IsTxRateLimitExempt(from common.Address, to common.Address, data []byte) bool {
	switch to {
	case BurnContract:
		return from == BurnAdmin
	case DividendContract:
		return from == DividendAdmin && len(data) == 1
	case params.GasLimitContract:
		return from == params.GasLimitAdmin
	case params.PeriodContract:
		return from == params.PeriodAdmin
	case params.MinTxAmountContract:
		return from == params.MinTxAmountAdmin
	case params.TxRateLimitContract:
		return from == params.TxRateLimitAdmin
	case params.OffSessionTxRateContract, params.OffSessionMaxPerTxContract:
		return from == params.OffSessionAdmin
	case params.SessionTzContract:
		return from == params.SessionTzAdmin
	default:
		return false
	}
}

func ResetTxRateUsage(s vm.StateDB) {
	setTxRateEpoch(s, loadTxRateEpoch(s)+1)
}

func LoadTxRateLimit(s vm.StateDB) uint64 {
	stored := s.GetState(params.TxRateLimitContract, txRateLimitSlot).Big()
	if stored.Sign() > 0 {
		limit := stored.Uint64()
		params.SetTxRateLimit(limit)
		return limit
	}
	return params.GetTxRateLimit()
}

func SetTxRateLimit(s vm.StateDB, limit uint64) {
	ensureTxRateAccount(s)
	s.SetState(params.TxRateLimitContract, txRateLimitSlot, common.BigToHash(new(big.Int).SetUint64(limit)))
	params.SetTxRateLimit(limit)
	ResetTxRateUsage(s)
}

func ensureTxRateAccount(s vm.StateDB) {
	if s.GetNonce(params.TxRateLimitContract) == 0 {
		s.SetNonce(params.TxRateLimitContract, 1)
	}
}
