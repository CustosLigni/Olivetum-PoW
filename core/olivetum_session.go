package core

import (
	"time"

	"github.com/ethereum/go-ethereum/params"
)

// IsSession reports whether the given Unix timestamp (seconds), after applying
// the configured session time offset, falls into the session window:
// Monday–Saturday from 12:00 to 24:00 (local = UTC + offset). Sundays are closed.
func IsSession(ts uint64) bool {
	off := int64(params.GetSessionTzOffsetSeconds())
	t := time.Unix(int64(ts)+off, 0).UTC()
	if t.Weekday() == time.Sunday {
		return false
	}
	hour := t.Hour()
	return hour >= 12 && hour < 24
}
