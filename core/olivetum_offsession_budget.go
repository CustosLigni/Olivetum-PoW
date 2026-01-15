package core

import (
	"encoding/binary"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

func offSessionBudgetWindow(ts uint64) uint64 {
	off := int64(params.GetSessionTzOffsetSeconds())
	local := int64(ts) + off
	if local < 0 {
		return 0
	}
	t := time.Unix(local, 0).UTC()
	if t.Weekday() != time.Sunday && t.Hour() >= 12 && t.Hour() < 24 {
		return 0
	}

	day := int64(24 * time.Hour / time.Second)
	window := (local / day) * day
	if t.Weekday() == time.Monday && t.Hour() < 12 {
		window -= day
		if window < 0 {
			window = 0
		}
	}
	return uint64(window)
}

func offSessionBudgetWindowSlot(addr common.Address) common.Hash {
	var b [32]byte
	b[0] = 0x01
	copy(b[12:], addr.Bytes())
	return common.BytesToHash(b[:])
}

func offSessionBudgetSpentSlot(addr common.Address) common.Hash {
	var b [32]byte
	b[0] = 0x02
	copy(b[12:], addr.Bytes())
	return common.BytesToHash(b[:])
}

func getOffSessionBudgetWindow(s vm.StateDB, addr common.Address) uint64 {
	b := s.GetState(params.OffSessionMaxPerTxContract, offSessionBudgetWindowSlot(addr))
	return binary.BigEndian.Uint64(b[24:])
}

func setOffSessionBudgetWindow(s vm.StateDB, addr common.Address, window uint64) {
	var b [32]byte
	binary.BigEndian.PutUint64(b[24:], window)
	s.SetState(params.OffSessionMaxPerTxContract, offSessionBudgetWindowSlot(addr), common.BytesToHash(b[:]))
}

func getOffSessionBudgetSpent(s vm.StateDB, addr common.Address) *big.Int {
	return new(big.Int).SetBytes(s.GetState(params.OffSessionMaxPerTxContract, offSessionBudgetSpentSlot(addr)).Bytes())
}

func setOffSessionBudgetSpent(s vm.StateDB, addr common.Address, spent *big.Int) {
	s.SetState(params.OffSessionMaxPerTxContract, offSessionBudgetSpentSlot(addr), common.BigToHash(spent))
}

func UpdateOffSessionBudget(s vm.StateDB, addr common.Address, amount *big.Int, now uint64) error {
	if s == nil || amount == nil || amount.Sign() == 0 {
		return nil
	}
	limit := params.GetOffSessionMaxPerTx()
	if limit.Sign() == 0 {
		return nil
	}

	window := offSessionBudgetWindow(now)
	prevWindow := getOffSessionBudgetWindow(s, addr)
	spent := getOffSessionBudgetSpent(s, addr)
	if prevWindow != window {
		spent = new(big.Int)
	}
	next := new(big.Int).Add(spent, amount)
	if next.Cmp(limit) > 0 {
		return ErrOverMaxOffSessionBudget
	}

	ensureMgmtAccountExists(s, params.OffSessionMaxPerTxContract)
	setOffSessionBudgetWindow(s, addr, window)
	setOffSessionBudgetSpent(s, addr, next)
	return nil
}

func GetOffSessionBudgetSpent(s vm.StateDB, addr common.Address, now uint64) *big.Int {
	if s == nil {
		return new(big.Int)
	}
	window := offSessionBudgetWindow(now)
	if getOffSessionBudgetWindow(s, addr) != window {
		return new(big.Int)
	}
	return getOffSessionBudgetSpent(s, addr)
}
