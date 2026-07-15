package metric

import (
	"math"
	"testing"
	"time"
)

func TestBoolFloat(t *testing.T) {
	if got := BoolFloat(true); got != 1 {
		t.Fatalf("BoolFloat(true) = %v, want 1", got)
	}
	if got := BoolFloat(false); got != 0 {
		t.Fatalf("BoolFloat(false) = %v, want 0", got)
	}
}

func TestUnixTimestamp(t *testing.T) {
	if got, want := UnixTimestamp(time.Unix(1_700_000_000, 123_000_000)), 1_700_000_000.123; math.Abs(got-want) > 0.000001 {
		t.Fatalf("UnixTimestamp() = %v, want %v", got, want)
	}
	if got := UnixTimestamp(time.Time{}); got != 0 {
		t.Fatalf("UnixTimestamp(zero) = %v, want 0", got)
	}
}

func TestNormalizeDuration(t *testing.T) {
	if got := NormalizeDuration(time.Second, time.Minute); got != time.Second {
		t.Fatalf("NormalizeDuration() = %v, want %v", got, time.Second)
	}
	if got := NormalizeDuration(0, time.Minute); got != time.Minute {
		t.Fatalf("NormalizeDuration(0) = %v, want %v", got, time.Minute)
	}
	if got := NormalizeDuration(-time.Second, time.Minute); got != time.Minute {
		t.Fatalf("NormalizeDuration(negative) = %v, want %v", got, time.Minute)
	}
}
