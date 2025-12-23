package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

func TestBurnShareFork(t *testing.T) {
	var (
		from     = common.HexToAddress("0x1111111111111111111111111111111111111111")
		to       = common.HexToAddress("0x2222222222222222222222222222222222222222")
		coinbase = common.HexToAddress("0x3333333333333333333333333333333333333333")
		value    = etherBig(100)
		burnRate = uint64(100) // 1%
	)

	oldFork := params.GetBurnShareForkBlock()
	t.Cleanup(func() { params.SetBurnShareForkBlock(oldFork) })
	params.SetBurnShareForkBlock(big.NewInt(2))

	tests := []struct {
		name      string
		blockNum  uint64
		wantShare bool
	}{
		{"pre-fork no share", 1, false},
		{"post-fork with share", 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evm, state, gp := newOlivetumEnv(t, 1000)
			evm.Context.BlockNumber = new(big.Int).SetUint64(tt.blockNum)
			evm.Context.Coinbase = coinbase

			SetBurnRate(state, burnRate)
			fundAccount(state, from, etherBig(1000))

			msg := Message{
				From:      from,
				To:        &to,
				Value:     value,
				GasLimit:  21000,
				GasPrice:  big.NewInt(0),
				GasFeeCap: big.NewInt(0),
				GasTipCap: big.NewInt(0),
				Nonce:     0,
			}

			st := NewStateTransition(evm, &msg, gp)
			if _, err := st.TransitionDb(); err != nil {
				t.Fatalf("transition error: %v", err)
			}

			expectedBurn := new(big.Int).Mul(value, big.NewInt(int64(burnRate)))
			expectedBurn.Div(expectedBurn, big.NewInt(10000))
			expectedShare := new(big.Int)
			if tt.wantShare {
				expectedShare.Mul(expectedBurn, big.NewInt(int64(MinerBurnShareBps)))
				expectedShare.Div(expectedShare, big.NewInt(10000))
			}
			expectedTo := new(big.Int).Sub(value, expectedBurn)

			coinbaseBal := state.GetBalance(coinbase).ToBig()
			if coinbaseBal.Cmp(expectedShare) != 0 {
				t.Fatalf("coinbase balance mismatch: got %s want %s", coinbaseBal, expectedShare)
			}

			toBal := state.GetBalance(to).ToBig()
			if toBal.Cmp(expectedTo) != 0 {
				t.Fatalf("recipient balance mismatch: got %s want %s", toBal, expectedTo)
			}

			senderBal := state.GetBalance(from).ToBig()
			expectedSender := new(big.Int).Sub(etherBig(1000), value)
			if senderBal.Cmp(expectedSender) != 0 {
				t.Fatalf("sender balance mismatch: got %s want %s", senderBal, expectedSender)
			}

			qualify := GetDividendStatus(state, coinbase).Qualify
			_ = qualify // keep compiler quiet if qualify unused later

			coinbaseTail := getRecentTail(state, coinbase)
			toTail := getRecentTail(state, to)

			if expectedShare.Sign() > 0 {
				if coinbaseTail != 1 {
					t.Fatalf("expected one recent entry for coinbase, got tail %d (recipient tail %d)", coinbaseTail, toTail)
				}
				amt, _ := getRecentEntry(state, coinbase, coinbaseTail-1)
				if amt.Cmp(expectedShare) != 0 {
					recipientAmt, _ := getRecentEntry(state, to, toTail-1)
					t.Fatalf("coinbase recent entry mismatch: got %s want %s (recipient recent %s)", amt, expectedShare, recipientAmt)
				}
			} else if coinbaseTail != 0 {
				t.Fatalf("expected no recent entries for coinbase pre-fork, got tail %d", coinbaseTail)
			}
		})
	}
}
