package util

import "time"

// NowUTC exposes time.Now for deterministic testing.
func NowUTC() time.Time {
	return time.Now().UTC()
}
