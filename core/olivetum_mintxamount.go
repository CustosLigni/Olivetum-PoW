package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

var minTxAmountSlot = common.Hash{}

// LoadMinTxAmount sets the current minimum transaction amount from state storage
// and returns the active value. If no value is stored, the existing minimum is kept.
func LoadMinTxAmount(s vm.StateDB) *big.Int {
	stored := s.GetState(params.MinTxAmountContract, minTxAmountSlot).Big()
	if stored.Sign() > 0 {
		params.SetMinTxAmount(stored)
		return stored
	}
	return params.GetMinTxAmount()
}

// SetMinTxAmount writes the minimum transaction amount into state storage and
// updates the runtime value used by validation.
func SetMinTxAmount(s vm.StateDB, amount *big.Int) {
	s.SetState(params.MinTxAmountContract, minTxAmountSlot, common.BigToHash(amount))
	params.SetMinTxAmount(amount)
}
