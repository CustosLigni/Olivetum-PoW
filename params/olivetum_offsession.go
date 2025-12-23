package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params/vars"
)

// Off-session (market closed) dynamic parameters.
//
// The administrator can update these by sending zero-value transactions to the
// dedicated management addresses below. Values are stored in state under those
// accounts and mirrored into runtime globals for fast access (same pattern as
// MinTxAmount and TxRateLimit).

var (
	// Admin is the same administrator used for other management actions.
	OffSessionAdmin = TxRateLimitAdmin

	// Contract addresses for off-session controls.
	OffSessionTxRateContract   = common.HexToAddress("0x0000000000000000000000000000000000000b05")
	OffSessionMaxPerTxContract = common.HexToAddress("0x0000000000000000000000000000000000000b06")

	// Off-session tx/h bounds and default.
	OffSessionTxRateMin uint64 = 1
	OffSessionTxRateMax uint64 = 100
	offSessionTxRate           = uint64(2) // default 2 tx/h off-session

	// Off-session per-transaction maximum amount bounds and default.
	// Units in wei. Bounds are 0.0001 .. 10000 Olivo.
	OffSessionMaxPerTxMin = new(big.Int).Div(big.NewInt(vars.Ether), big.NewInt(10000)) // 0.0001 Olivo
	OffSessionMaxPerTxMax = new(big.Int).Mul(big.NewInt(10000), big.NewInt(vars.Ether)) // 10000 Olivo
	offSessionMaxPerTx    = new(big.Int).Set(OffSessionMaxPerTxMax)                     // default 10000 Olivo
)

func GetOffSessionTxRate() uint64      { return offSessionTxRate }
func SetOffSessionTxRate(limit uint64) { offSessionTxRate = limit }

// DecodeOffSessionTxRate expects a single byte (uint8) representing the tx/h
// limit. Valid range: [1, 100].
func DecodeOffSessionTxRate(data []byte) (uint64, bool) {
	if len(data) != 1 {
		return 0, false
	}
	v := uint64(data[0])
	if v < OffSessionTxRateMin || v > OffSessionTxRateMax {
		return 0, false
	}
	return v, true
}

func GetOffSessionMaxPerTx() *big.Int       { return new(big.Int).Set(offSessionMaxPerTx) }
func SetOffSessionMaxPerTx(amount *big.Int) { offSessionMaxPerTx.Set(amount) }

// DecodeOffSessionMaxPerTx expects an 8-byte big-endian unsigned integer N,
// where value = N * 0.0001 Olivo. The resulting value must be within
// [0.0001, 10000] Olivo.
func DecodeOffSessionMaxPerTx(data []byte) (*big.Int, bool) {
	if len(data) != 8 {
		return nil, false
	}
	val := new(big.Int).SetBytes(data)
	// Scale by 0.0001 Olivo (Ether/10000)
	val.Mul(val, OffSessionMaxPerTxMin)
	if val.Cmp(OffSessionMaxPerTxMin) < 0 || val.Cmp(OffSessionMaxPerTxMax) > 0 {
		return nil, false
	}
	return val, true
}
