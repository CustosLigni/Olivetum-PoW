package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

var totalMintedSlot = common.Hash{0: 0x0c}

// GetTotalMinted reads the total minted amount tracked in state. Returns 0 if
// state is nil.
func GetTotalMinted(state vm.StateDB) *big.Int {
	if state == nil {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(state.GetState(params.RewardVault, totalMintedSlot).Bytes())
}

// SetTotalMinted writes the cumulative minted amount into state.
func SetTotalMinted(state vm.StateDB, amount *big.Int) {
	if state == nil {
		return
	}
	if state.GetNonce(params.RewardVault) == 0 {
		state.SetNonce(params.RewardVault, 1)
	}
	state.SetState(params.RewardVault, totalMintedSlot, common.BigToHash(amount))
}
