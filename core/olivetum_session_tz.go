package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

var sessionTzSlot = common.Hash{}

func LoadSessionTzOffset(s vm.StateDB) int32 {
	stored := s.GetState(params.SessionTzContract, sessionTzSlot).Big()
	if stored.Sign() != 0 {
		v := int32(stored.Uint64())
		params.SetSessionTzOffsetSeconds(v)
		return v
	}
	return params.GetSessionTzOffsetSeconds()
}

func SetSessionTzOffset(s vm.StateDB, offset int32) {
	ensureMgmtAccountExists(s, params.SessionTzContract)
	s.SetState(params.SessionTzContract, sessionTzSlot, common.BigToHash(new(big.Int).SetUint64(uint64(uint32(offset)))))
	params.SetSessionTzOffsetSeconds(offset)
}
