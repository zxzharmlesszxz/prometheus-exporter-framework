package __FEATURE_NAME__check

import (
	"context"
	"testing"
	"time"
)

func TestCheckerSnapshot(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshot := NewChecker().Snapshot(context.Background(), now)
	if !snapshot.AttemptTime.Equal(now) {
		t.Fatalf("AttemptTime = %v, want %v", snapshot.AttemptTime, now)
	}
	if !snapshot.Success {
		t.Fatal("Success = false, want true")
	}
	if snapshot.Value != 1 {
		t.Fatalf("Value = %v, want 1", snapshot.Value)
	}
	if snapshot.Err != nil {
		t.Fatalf("Err = %v, want nil", snapshot.Err)
	}
}
