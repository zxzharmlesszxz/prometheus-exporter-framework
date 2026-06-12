package __FEATURE_NAME__check

import (
	"context"
	"time"
)

type Checker struct{}

func NewChecker() Checker {
	return Checker{}
}

func (c Checker) Snapshot(_ context.Context, now time.Time) Snapshot {
	return Snapshot{
		AttemptTime: now,
		Success:     true,
		Value:       1,
	}
}
