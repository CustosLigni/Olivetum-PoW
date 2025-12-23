package rawdb

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var olivetumPeriodPrefix = []byte("olivetum-period-")
var olivetumFinalizedHeightKey = []byte("olivetum-finalized-height")

func olivetumPeriodKey(hash common.Hash) []byte {
	return append(olivetumPeriodPrefix, hash.Bytes()...)
}

// WriteOlivetumBlockPeriod persists the runtime block-period (in seconds) that
// was in effect after finishing the specified block. This allows Olivetum
// difficulty calculation and runtime guards to survive restarts and rewinds.
func WriteOlivetumBlockPeriod(db ethdb.KeyValueWriter, hash common.Hash, period uint64) {
	var enc [8]byte
	binary.BigEndian.PutUint64(enc[:], period)
	if err := db.Put(olivetumPeriodKey(hash), enc[:]); err != nil {
		log.Crit("Failed to store Olivetum block period", "hash", hash, "err", err)
	}
}

// ReadOlivetumBlockPeriod loads the stored block-period value for the provided
// block hash.
func ReadOlivetumBlockPeriod(db ethdb.KeyValueReader, hash common.Hash) (uint64, bool) {
	data, err := db.Get(olivetumPeriodKey(hash))
	if err != nil || len(data) == 0 {
		return 0, false
	}
	return binary.BigEndian.Uint64(data), true
}

// WriteOlivetumFinalizedHeight persists the finalized height watermark.
func WriteOlivetumFinalizedHeight(db ethdb.KeyValueWriter, height uint64) {
	var enc [8]byte
	binary.BigEndian.PutUint64(enc[:], height)
	if err := db.Put(olivetumFinalizedHeightKey, enc[:]); err != nil {
		log.Crit("Failed to store Olivetum finalized height", "height", height, "err", err)
	}
}

// ReadOlivetumFinalizedHeight loads the finalized height watermark.
func ReadOlivetumFinalizedHeight(db ethdb.KeyValueReader) (uint64, bool) {
	data, err := db.Get(olivetumFinalizedHeightKey)
	if err != nil || len(data) == 0 {
		return 0, false
	}
	return binary.BigEndian.Uint64(data), true
}
