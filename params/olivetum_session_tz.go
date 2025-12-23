package params

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
)

var (
	// Admin for session time offset is the same network admin.
	SessionTzAdmin = TxRateLimitAdmin

	// Session time offset management address. Value is stored as signed
	// 32-bit seconds offset applied to block timestamp when determining
	// session windows. Positive = east of UTC.
	SessionTzContract = common.HexToAddress("0x0000000000000000000000000000000000000b07")

	// Current session time offset in seconds. Default 0 (UTC).
	currentSessionTzOffsetSeconds int32 = 0
)

func GetSessionTzOffsetSeconds() int32  { return currentSessionTzOffsetSeconds }
func SetSessionTzOffsetSeconds(v int32) { currentSessionTzOffsetSeconds = v }

// DecodeSessionTzOffset expects exactly 4 bytes, big-endian signed int32 seconds.
// A loose sanity bound of +/- 24h is enforced.
func DecodeSessionTzOffset(data []byte) (int32, bool) {
	if len(data) != 4 {
		return 0, false
	}
	v := int32(binary.BigEndian.Uint32(data))
	if v < -86400 || v > 86400 {
		return 0, false
	}
	return v, true
}
