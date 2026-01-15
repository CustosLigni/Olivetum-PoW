package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

var (
	totalMintedSlot          = common.Hash{0: 0x0c}
	totalBurnedSlot          = common.Hash{0: 0x0d}
	totalDividendsMintedSlot = common.Hash{0: 0x0e}
	totalBurnedTransfersSlot = common.Hash{0: 0x0f}
	totalBurnedGasSlot       = common.Hash{0: 0x10}
	totalMinerBurnShareSlot  = common.Hash{0: 0x11}
)

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
	ensureRewardVaultAccount(state)
	state.SetState(params.RewardVault, totalMintedSlot, common.BigToHash(amount))
}

func GetTotalBurned(state vm.StateDB) *big.Int {
	if state == nil {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(state.GetState(params.RewardVault, totalBurnedSlot).Bytes())
}

func SetTotalBurned(state vm.StateDB, amount *big.Int) {
	if state == nil {
		return
	}
	ensureRewardVaultAccount(state)
	state.SetState(params.RewardVault, totalBurnedSlot, common.BigToHash(amount))
}

func AddTotalBurned(state vm.StateDB, amount *big.Int) {
	if state == nil || amount == nil || amount.Sign() == 0 {
		return
	}
	total := GetTotalBurned(state)
	total.Add(total, amount)
	SetTotalBurned(state, total)
}

func GetTotalBurnedTransfers(state vm.StateDB) *big.Int {
	if state == nil {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(state.GetState(params.RewardVault, totalBurnedTransfersSlot).Bytes())
}

func SetTotalBurnedTransfers(state vm.StateDB, amount *big.Int) {
	if state == nil {
		return
	}
	ensureRewardVaultAccount(state)
	state.SetState(params.RewardVault, totalBurnedTransfersSlot, common.BigToHash(amount))
}

func AddTotalBurnedTransfers(state vm.StateDB, amount *big.Int) {
	if state == nil || amount == nil || amount.Sign() == 0 {
		return
	}
	total := GetTotalBurnedTransfers(state)
	total.Add(total, amount)
	SetTotalBurnedTransfers(state, total)
}

func GetTotalBurnedGas(state vm.StateDB) *big.Int {
	if state == nil {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(state.GetState(params.RewardVault, totalBurnedGasSlot).Bytes())
}

func SetTotalBurnedGas(state vm.StateDB, amount *big.Int) {
	if state == nil {
		return
	}
	ensureRewardVaultAccount(state)
	state.SetState(params.RewardVault, totalBurnedGasSlot, common.BigToHash(amount))
}

func AddTotalBurnedGas(state vm.StateDB, amount *big.Int) {
	if state == nil || amount == nil || amount.Sign() == 0 {
		return
	}
	total := GetTotalBurnedGas(state)
	total.Add(total, amount)
	SetTotalBurnedGas(state, total)
}

func GetTotalMinerBurnShare(state vm.StateDB) *big.Int {
	if state == nil {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(state.GetState(params.RewardVault, totalMinerBurnShareSlot).Bytes())
}

func SetTotalMinerBurnShare(state vm.StateDB, amount *big.Int) {
	if state == nil {
		return
	}
	ensureRewardVaultAccount(state)
	state.SetState(params.RewardVault, totalMinerBurnShareSlot, common.BigToHash(amount))
}

func AddTotalMinerBurnShare(state vm.StateDB, amount *big.Int) {
	if state == nil || amount == nil || amount.Sign() == 0 {
		return
	}
	total := GetTotalMinerBurnShare(state)
	total.Add(total, amount)
	SetTotalMinerBurnShare(state, total)
}

func GetTotalDividendsMinted(state vm.StateDB) *big.Int {
	if state == nil {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(state.GetState(params.RewardVault, totalDividendsMintedSlot).Bytes())
}

func SetTotalDividendsMinted(state vm.StateDB, amount *big.Int) {
	if state == nil {
		return
	}
	ensureRewardVaultAccount(state)
	state.SetState(params.RewardVault, totalDividendsMintedSlot, common.BigToHash(amount))
}

func AddTotalDividendsMinted(state vm.StateDB, amount *big.Int) {
	if state == nil || amount == nil || amount.Sign() == 0 {
		return
	}
	total := GetTotalDividendsMinted(state)
	total.Add(total, amount)
	SetTotalDividendsMinted(state, total)
}

func ensureRewardVaultAccount(state vm.StateDB) {
	if state.GetNonce(params.RewardVault) == 0 {
		state.SetNonce(params.RewardVault, 1)
	}
}
