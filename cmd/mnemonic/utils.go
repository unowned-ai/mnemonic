package main

import (
	"time"
)

// formatTimestamp converts a Unix timestamp (float64, seconds since epoch)
// to a human-readable string in RFC3339 format.
func formatTimestamp(timestamp float64) string {
	timeObj := time.Unix(int64(timestamp), 0)
	return timeObj.Format(time.RFC3339)
}
