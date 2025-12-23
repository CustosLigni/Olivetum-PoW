package core

import (
	"encoding/binary"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
)

var (
	DividendAdmin            = BurnAdmin
	DividendContract         = common.HexToAddress("0x000000000000000000000000000000000000d1e1")
	dividendOptions          = []uint64{50, 100, 150, 200, 300}
	currentDividend          = dividendOptions[0]
	dividendQualify   uint64 = 30 * 24 * 60 * 60  // 30 days atleast to qualify
	dividendInterval  uint64 = 364 * 24 * 60 * 60 // 364 days interval
	claimWindow       uint64 = 24 * 60 * 60       // 24 hours window to claim
	maxTimestampDrift uint64 = 15                 // 15 second tolerance for block timestamp

	lastDividendSlot = common.Hash{0: 1}
	roundRateSlot    = common.Hash{0: 2}
	roundStartSlot   = common.Hash{0: 3}
	roundIDSlot      = common.Hash{0: 4}
	// DividendClaimedTopic is keccak256("DividendClaimed(address,uint256)")
	DividendClaimedTopic = common.HexToHash("0x5efa67896a23b651b741b525caacba039c00ca7853be3de8eb1f4269e8669c56")
)

func holdingSlot(addr common.Address) common.Hash {
	var b [32]byte
	b[0] = 0x05
	copy(b[12:], addr.Bytes())
	return common.BytesToHash(b[:])
}

func claimedSlot(addr common.Address) common.Hash {
	var b [32]byte
	b[0] = 0x06
	copy(b[12:], addr.Bytes())
	return common.BytesToHash(b[:])
}

func heldAmountSlot(addr common.Address) common.Hash {
	var b [32]byte
	b[0] = 0x07
	copy(b[12:], addr.Bytes())
	return common.BytesToHash(b[:])
}

func recentTimeSlot(addr common.Address) common.Hash {
	var b [32]byte
	b[0] = 0x08
	copy(b[12:], addr.Bytes())
	return common.BytesToHash(b[:])
}

func recentAmountSlot(addr common.Address) common.Hash {
	var b [32]byte
	b[0] = 0x09
	copy(b[12:], addr.Bytes())
	return common.BytesToHash(b[:])
}

func recentHeadSlot(addr common.Address) common.Hash {
	var b [32]byte
	b[0] = 0x0a
	copy(b[12:], addr.Bytes())
	return common.BytesToHash(b[:])
}

func recentTailSlot(addr common.Address) common.Hash {
	var b [32]byte
	b[0] = 0x0b
	copy(b[12:], addr.Bytes())
	return common.BytesToHash(b[:])
}

func recentAmountIndexSlot(addr common.Address, idx uint64) common.Hash {
	var b [32]byte
	b[0] = 0x0c
	copy(b[12:], addr.Bytes())
	binary.BigEndian.PutUint64(b[24:], idx)
	return common.BytesToHash(b[:])
}

func recentTimeIndexSlot(addr common.Address, idx uint64) common.Hash {
	var b [32]byte
	b[0] = 0x0d
	copy(b[12:], addr.Bytes())
	binary.BigEndian.PutUint64(b[24:], idx)
	return common.BytesToHash(b[:])
}

func getRecentHead(s vm.StateDB, addr common.Address) uint64 {
	b := s.GetState(DividendContract, recentHeadSlot(addr))
	return binary.BigEndian.Uint64(b[24:])
}

func getRecentTail(s vm.StateDB, addr common.Address) uint64 {
	b := s.GetState(DividendContract, recentTailSlot(addr))
	return binary.BigEndian.Uint64(b[24:])
}

func setRecentHead(s vm.StateDB, addr common.Address, idx uint64) {
	var buf [32]byte
	binary.BigEndian.PutUint64(buf[24:], idx)
	s.SetState(DividendContract, recentHeadSlot(addr), common.BytesToHash(buf[:]))
}

func setRecentTail(s vm.StateDB, addr common.Address, idx uint64) {
	var buf [32]byte
	binary.BigEndian.PutUint64(buf[24:], idx)
	s.SetState(DividendContract, recentTailSlot(addr), common.BytesToHash(buf[:]))
}

func getRecentEntry(s vm.StateDB, addr common.Address, idx uint64) (*big.Int, uint64) {
	amt := new(big.Int).SetBytes(s.GetState(DividendContract, recentAmountIndexSlot(addr, idx)).Bytes())
	t := binary.BigEndian.Uint64(s.GetState(DividendContract, recentTimeIndexSlot(addr, idx)).Bytes()[24:])
	return amt, t
}

func setRecentEntry(s vm.StateDB, addr common.Address, idx uint64, amt *big.Int, t uint64) {
	var buf [32]byte
	if amt.Sign() > 0 {
		ab := amt.Bytes()
		copy(buf[32-len(ab):], ab)
	}
	s.SetState(DividendContract, recentAmountIndexSlot(addr, idx), common.BytesToHash(buf[:]))
	binary.BigEndian.PutUint64(buf[24:], t)
	s.SetState(DividendContract, recentTimeIndexSlot(addr, idx), common.BytesToHash(buf[:]))
}

func clearRecentEntry(s vm.StateDB, addr common.Address, idx uint64) {
	zero := common.Hash{}
	s.SetState(DividendContract, recentAmountIndexSlot(addr, idx), zero)
	s.SetState(DividendContract, recentTimeIndexSlot(addr, idx), zero)
}

func ensureDividendAccount(s vm.StateDB) {
	if s.GetNonce(DividendContract) == 0 {
		s.SetNonce(DividendContract, 1)
	}
}

// bootstrapHolding seeds the dividend tracking slots for an account that holds
// a balance but has never interacted with the dividend logic.
func bootstrapHolding(s vm.StateDB, addr common.Address) {
	if getHeldAmount(s, addr).Sign() > 0 || getRecentHead(s, addr) != getRecentTail(s, addr) {
		return
	}
	bal := s.GetBalance(addr).ToBig()
	if bal.Sign() > 0 {
		setHeldAmount(s, addr, bal)
		setHoldingTime(s, addr, 0)
	}
}

func getHoldingTime(s vm.StateDB, addr common.Address) uint64 {
	b := s.GetState(DividendContract, holdingSlot(addr))
	return binary.BigEndian.Uint64(b[24:])
}

func setHoldingTime(s vm.StateDB, addr common.Address, t uint64) {
	var b [32]byte
	binary.BigEndian.PutUint64(b[24:], t)
	s.SetState(DividendContract, holdingSlot(addr), common.BytesToHash(b[:]))
}

func getClaimedRound(s vm.StateDB, addr common.Address) uint64 {
	b := s.GetState(DividendContract, claimedSlot(addr))
	return binary.BigEndian.Uint64(b[24:])
}

func setClaimedRound(s vm.StateDB, addr common.Address, id uint64) {
	var b [32]byte
	binary.BigEndian.PutUint64(b[24:], id)
	s.SetState(DividendContract, claimedSlot(addr), common.BytesToHash(b[:]))
}

func matureRecent(s vm.StateDB, addr common.Address, now uint64) {
	head := getRecentHead(s, addr)
	tail := getRecentTail(s, addr)
	updatedHead := head
	for head < tail {
		amt, t := getRecentEntry(s, addr, head)
		if amt.Sign() == 0 {
			clearRecentEntry(s, addr, head)
			head++
			continue
		}
		if now-t < dividendQualify {
			break
		}
		held := getHeldAmount(s, addr)
		if held.Sign() == 0 {
			setHoldingTime(s, addr, t)
		}
		held.Add(held, amt)
		setHeldAmount(s, addr, held)
		clearRecentEntry(s, addr, head)
		head++
		updatedHead = head
	}
	if updatedHead != getRecentHead(s, addr) {
		setRecentHead(s, addr, updatedHead)
	}
}

func getLastDividend(s vm.StateDB) uint64 {
	b := s.GetState(DividendContract, lastDividendSlot)
	return binary.BigEndian.Uint64(b[24:])
}

func setLastDividend(s vm.StateDB, t uint64) {
	var b [32]byte
	binary.BigEndian.PutUint64(b[24:], t)
	s.SetState(DividendContract, lastDividendSlot, common.BytesToHash(b[:]))
}

func getRoundRate(s vm.StateDB) uint64 {
	b := s.GetState(DividendContract, roundRateSlot)
	return binary.BigEndian.Uint64(b[24:])
}

func setRoundRate(s vm.StateDB, v uint64) {
	var b [32]byte
	binary.BigEndian.PutUint64(b[24:], v)
	s.SetState(DividendContract, roundRateSlot, common.BytesToHash(b[:]))
}

func getRoundStart(s vm.StateDB) uint64 {
	b := s.GetState(DividendContract, roundStartSlot)
	return binary.BigEndian.Uint64(b[24:])
}

func setRoundStart(s vm.StateDB, v uint64) {
	var b [32]byte
	binary.BigEndian.PutUint64(b[24:], v)
	s.SetState(DividendContract, roundStartSlot, common.BytesToHash(b[:]))
}

func getRoundID(s vm.StateDB) uint64 {
	b := s.GetState(DividendContract, roundIDSlot)
	return binary.BigEndian.Uint64(b[24:])
}

func setRoundID(s vm.StateDB, v uint64) {
	var b [32]byte
	binary.BigEndian.PutUint64(b[24:], v)
	s.SetState(DividendContract, roundIDSlot, common.BytesToHash(b[:]))
}

// DividendStatus represents the current dividend round parameters along with
// whether a given address has already claimed the payout.
type DividendStatus struct {
	Rate    uint64
	Start   uint64
	Qualify uint64
	Window  uint64
	Claimed bool
}

func GetDividendStatus(s vm.StateDB, addr common.Address) DividendStatus {
	return DividendStatus{
		Rate:    getRoundRate(s),
		Start:   getRoundStart(s),
		Qualify: dividendQualify,
		Window:  claimWindow,
		Claimed: getClaimedRound(s, addr) == getRoundID(s),
	}
}

func DecodeDividendRate(data []byte) (uint64, bool) {
	if len(data) != 1 {
		return 0, false
	}
	idx := int(data[0])
	if idx < 0 || idx >= len(dividendOptions) {
		return 0, false
	}
	return dividendOptions[idx], true
}

func SetDividendRate(rate uint64) { currentDividend = rate }

// GetDividendRate returns the currently active dividend rate. If state is
// provided, it prefers the on-chain configured value, otherwise it falls back
// to the in-memory default.
func GetDividendRate(s vm.StateDB) uint64 {
	if s != nil {
		if rate := getRoundRate(s); rate != 0 {
			return rate
		}
	}
	return currentDividend
}

func getHeldAmount(s vm.StateDB, addr common.Address) *big.Int {
	b := s.GetState(DividendContract, heldAmountSlot(addr))
	return new(big.Int).SetBytes(b.Bytes())
}

func setHeldAmount(s vm.StateDB, addr common.Address, amt *big.Int) {
	var b [32]byte
	if amt.Sign() > 0 {
		ab := amt.Bytes()
		copy(b[32-len(ab):], ab)
	}
	s.SetState(DividendContract, heldAmountSlot(addr), common.BytesToHash(b[:]))
}

func AddHolding(s vm.StateDB, addr common.Address, amt *big.Int, now uint64) {
	if amt == nil || amt.Sign() == 0 {
		return
	}
	ensureDividendAccount(s)
	matureRecent(s, addr, now)
	tail := getRecentTail(s, addr)
	setRecentEntry(s, addr, tail, amt, now)
	setRecentTail(s, addr, tail+1)
}

func RemoveHolding(s vm.StateDB, addr common.Address, amt *big.Int, now uint64) {
	if amt == nil || amt.Sign() == 0 {
		return
	}
	ensureDividendAccount(s)
	matureRecent(s, addr, now)
	remaining := new(big.Int).Set(amt)
	for {
		head := getRecentHead(s, addr)
		tail := getRecentTail(s, addr)
		if remaining.Sign() == 0 || head >= tail {
			break
		}
		idx := tail - 1
		entryAmt, entryTime := getRecentEntry(s, addr, idx)
		if entryAmt.Sign() == 0 {
			clearRecentEntry(s, addr, idx)
			setRecentTail(s, addr, idx)
			continue
		}
		if entryAmt.Cmp(remaining) <= 0 {
			remaining.Sub(remaining, entryAmt)
			clearRecentEntry(s, addr, idx)
			setRecentTail(s, addr, idx)
			continue
		}
		entryAmt.Sub(entryAmt, remaining)
		setRecentEntry(s, addr, idx, entryAmt, entryTime)
		remaining.SetInt64(0)
	}
	held := getHeldAmount(s, addr)
	if held.Cmp(remaining) <= 0 {
		setHeldAmount(s, addr, new(big.Int))
		setHoldingTime(s, addr, now)
	} else {
		held.Sub(held, remaining)
		setHeldAmount(s, addr, held)
	}
}

// DividendView summarizes current holdings for informational queries without
// mutating state (useful for RPC). EligibleNow includes held plus any
// recent entries that would mature at the provided timestamp; Pending is
// the sum of recent entries younger than the qualification window.
type DividendView struct {
	EligibleNow *big.Int
	Pending     *big.Int
}

// GetDividendView computes the dividend view at a given timestamp without
// changing state.
func GetDividendView(s vm.StateDB, addr common.Address, now uint64) DividendView {
	eligible := getHeldAmount(s, addr)
	pending := new(big.Int)

	head := getRecentHead(s, addr)
	tail := getRecentTail(s, addr)
	for idx := head; idx < tail; idx++ {
		amt, t := getRecentEntry(s, addr, idx)
		if amt.Sign() == 0 {
			continue
		}
		if now-t >= dividendQualify {
			eligible = new(big.Int).Add(eligible, amt)
		} else {
			pending.Add(pending, amt)
		}
	}
	return DividendView{
		EligibleNow: eligible,
		Pending:     pending,
	}
}

func ClaimDividend(s vm.StateDB, addr common.Address, now uint64) bool {
	ensureDividendAccount(s)
	bootstrapHolding(s, addr)
	matureRecent(s, addr, now)
	real := uint64(time.Now().Unix())
	if now > real+maxTimestampDrift {
		return false
	}
	rate := getRoundRate(s)
	if rate == 0 {
		return false
	}
	start := getRoundStart(s)
	if now < start || now-start > claimWindow {
		return false
	}
	if now-getHoldingTime(s, addr) < dividendQualify {
		return false
	}
	roundID := getRoundID(s)
	if getClaimedRound(s, addr) == roundID {
		return false
	}
	held := getHeldAmount(s, addr)
	if held.Sign() == 0 {
		return false
	}
	reward := new(big.Int).Mul(held, big.NewInt(int64(rate)))
	reward.Div(reward, big.NewInt(10000))
	if reward.Sign() == 0 {
		return false
	}
	s.AddBalance(addr, uint256.MustFromBig(reward))

	held.Add(held, reward)
	setHeldAmount(s, addr, held)

	s.AddLog(&types.Log{
		Address: DividendContract,
		Topics: []common.Hash{
			DividendClaimedTopic,
			common.BytesToHash(addr[:]),
		},
		Data: reward.FillBytes(make([]byte, 32)),
	})

	setClaimedRound(s, addr, roundID)
	return true
}

func TriggerDividend(s vm.StateDB, now uint64) bool {
	ensureDividendAccount(s)
	real := uint64(time.Now().Unix())
	if now > real+maxTimestampDrift {
		return false
	}
	last := getLastDividend(s)
	if last != 0 && now-last < dividendInterval {
		return false
	}
	rate := getRoundRate(s)
	if rate != 0 && now-getRoundStart(s) <= claimWindow {
		return false
	}
	setRoundRate(s, currentDividend)
	setRoundStart(s, now)
	setRoundID(s, getRoundID(s)+1)
	setLastDividend(s, now)
	return true
}

func CanTriggerDividend(s vm.StateDB, now uint64) bool {
	ensureDividendAccount(s)
	real := uint64(time.Now().Unix())
	if now > real+maxTimestampDrift {
		return false
	}
	last := getLastDividend(s)
	if last != 0 && now-last < dividendInterval {
		return false
	}
	rate := getRoundRate(s)
	if rate != 0 && now-getRoundStart(s) <= claimWindow {
		return false
	}
	return true
}
