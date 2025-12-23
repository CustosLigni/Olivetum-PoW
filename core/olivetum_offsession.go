package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

var (
	offSessionTxRateSlot   = common.Hash{}
	offSessionMaxPerTxSlot = common.Hash{}
)

func LoadOffSessionTxRate(s vm.StateDB) uint64 {
	stored := s.GetState(params.OffSessionTxRateContract, offSessionTxRateSlot).Big()
	if stored.Sign() > 0 {
		limit := stored.Uint64()
		params.SetOffSessionTxRate(limit)
		return limit
	}
	return params.GetOffSessionTxRate()
}

func SetOffSessionTxRate(s vm.StateDB, limit uint64) {
	ensureMgmtAccountExists(s, params.OffSessionTxRateContract)
	s.SetState(params.OffSessionTxRateContract, offSessionTxRateSlot, common.BigToHash(new(big.Int).SetUint64(limit)))
	params.SetOffSessionTxRate(limit)
	ResetTxRateUsage(s)
}

func LoadOffSessionMaxPerTx(s vm.StateDB) *big.Int {
	stored := s.GetState(params.OffSessionMaxPerTxContract, offSessionMaxPerTxSlot).Big()
	if stored.Sign() > 0 {
		params.SetOffSessionMaxPerTx(stored)
		return stored
	}
	return params.GetOffSessionMaxPerTx()
}

func SetOffSessionMaxPerTx(s vm.StateDB, amount *big.Int) {
	ensureMgmtAccountExists(s, params.OffSessionMaxPerTxContract)
	s.SetState(params.OffSessionMaxPerTxContract, offSessionMaxPerTxSlot, common.BigToHash(amount))
	params.SetOffSessionMaxPerTx(amount)
}

func ensureMgmtAccountExists(s vm.StateDB, addr common.Address) {
	if s.GetNonce(addr) == 0 {
		s.SetNonce(addr, 1)
	}
}
