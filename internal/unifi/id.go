package unifi

import (
	"encoding/hex"
	"time"
)

// IDTime returns the creation time embedded in a controller object ID. IDs are
// MongoDB ObjectIDs, whose first four bytes are the creation time in seconds
// since the Unix epoch, taken from the controller's clock. It reports false
// for anything that isn't a 24-character hex ID.
func IDTime(id string) (time.Time, bool) {
	if len(id) != 24 {
		return time.Time{}, false
	}
	b, err := hex.DecodeString(id)
	if err != nil {
		return time.Time{}, false
	}
	secs := int64(b[0])<<24 | int64(b[1])<<16 | int64(b[2])<<8 | int64(b[3])
	return time.Unix(secs, 0), true
}
