package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

const MinerBurnShareBps = 25

var (
	BurnAdmin    = common.HexToAddress("0x17a96Ab66c971e72bb1F9D355792eCEaeaf59af5")
	BurnContract = common.HexToAddress("0x0000000000000000000000000000000000000b00")
	burnOptions  = []uint64{50, 100, 150, 200, 250, 300}
	burnSlot     = common.Hash{}
)

func GetBurnRate(s vm.StateDB) uint64 {
	stored := s.GetState(BurnContract, burnSlot).Big().Uint64()
	if stored == 0 {
		return burnOptions[0]
	}
	return stored
}

func SetBurnRate(s vm.StateDB, rate uint64) {
	ensureBurnAccount(s)
	s.SetState(BurnContract, burnSlot, common.BigToHash(new(big.Int).SetUint64(rate)))
}

func ensureBurnAccount(s vm.StateDB) {
	if s.GetNonce(BurnContract) == 0 {
		s.SetNonce(BurnContract, 1)
	}
}

func DecodeBurnRate(data []byte) (uint64, bool) {
	if len(data) != 1 {
		return 0, false
	}
	idx := int(data[0])
	if idx < 0 || idx >= len(burnOptions) {
		return 0, false
	}
	return burnOptions[idx], true
}
