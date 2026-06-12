package __FEATURE_NAME__check

import "time"

type Snapshot struct {
	AttemptTime time.Time
	Success     bool
	Value       float64
	Err         error
}
