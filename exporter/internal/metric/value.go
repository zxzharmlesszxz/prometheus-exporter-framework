package metric

import "time"

func BoolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func UnixTimestamp(value time.Time) float64 {
	if value.IsZero() {
		return 0
	}
	return float64(value.UnixNano()) / float64(time.Second)
}

func NormalizeDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
