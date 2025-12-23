package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

var blockPeriodSlot = common.Hash{}

func LoadBlockPeriod(s vm.StateDB) uint64 {
	stored := s.GetState(params.PeriodContract, blockPeriodSlot).Big().Uint64()
	if stored != 0 {
		params.SetBlockPeriod(stored)
		return stored
	}
	return params.GetBlockPeriod()
}

func SetBlockPeriod(s vm.StateDB, period uint64) {
	s.SetState(params.PeriodContract, blockPeriodSlot, common.BigToHash(new(big.Int).SetUint64(period)))
	params.SetBlockPeriod(period)
}
