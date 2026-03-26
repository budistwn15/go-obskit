package httplog

import "time"

func DurationMS(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	return d.Milliseconds()
}

func IsSlowRequest(d time.Duration, threshold time.Duration) bool {
	if threshold <= 0 {
		return false
	}
	return d >= threshold
}

func Since(start time.Time) time.Duration {
	if start.IsZero() {
		return 0
	}
	return time.Since(start)
}
