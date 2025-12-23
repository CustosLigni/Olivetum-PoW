package params

import "github.com/ethereum/go-ethereum/common"

var (
	TxRateLimitAdmin    = common.HexToAddress("0x17a96Ab66c971e72bb1F9D355792eCEaeaf59af5")
	TxRateLimitContract = common.HexToAddress("0x0000000000000000000000000000000000000b04")

	TxRateLimitDefault = uint64(5)
	TxRateLimitMin     = uint64(1)
	TxRateLimitMax     = uint64(100)

	currentTxRateLimit = TxRateLimitDefault
)

func GetTxRateLimit() uint64      { return currentTxRateLimit }
func SetTxRateLimit(limit uint64) { currentTxRateLimit = limit }
func DecodeTxRateLimit(data []byte) (uint64, bool) {
	if len(data) != 1 {
		return 0, false
	}
	limit := uint64(data[0])
	if limit < TxRateLimitMin || limit > TxRateLimitMax {
		return 0, false
	}
	return limit, true
}
