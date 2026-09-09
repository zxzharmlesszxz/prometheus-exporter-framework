package featurekit

import (
	"sync"
	"testing"
	"time"
)

func TestLastKnownGoodSuccessFailureAndStale(t *testing.T) {
	store := NewLastKnownGood[string, int](time.Minute)
	now := time.Unix(100, 0)

	store.Failure("domain", now)
	result := store.Get("domain", now)
	if result.Available {
		t.Fatal("expected unavailable data after failure without success")
	}
	if result.ConsecutiveFailures != 1 {
		t.Fatalf("ConsecutiveFailures = %d, want 1", result.ConsecutiveFailures)
	}

	store.Success("domain", 42, now.Add(time.Second))
	result = store.Get("domain", now.Add(30*time.Second))
	if !result.Available || result.Stale || result.Value != 42 {
		t.Fatalf("Get() = %#v, want available fresh value", result)
	}
	if result.ConsecutiveFailures != 0 {
		t.Fatalf("ConsecutiveFailures after success = %d, want 0", result.ConsecutiveFailures)
	}

	store.Failure("domain", now.Add(40*time.Second))
	result = store.Get("domain", now.Add(2*time.Minute))
	if !result.Available || !result.Stale || result.Value != 42 {
		t.Fatalf("Get() = %#v, want stale last-known-good value", result)
	}
	if result.ConsecutiveFailures != 1 {
		t.Fatalf("ConsecutiveFailures = %d, want 1", result.ConsecutiveFailures)
	}
}

func TestLastKnownGoodDeleteLenSnapshotAndNilReceiver(t *testing.T) {
	now := time.Unix(100, 0)
	store := NewLastKnownGood[string, int](0)
	store.Success("one", 1, now)
	store.Failure("two", now)

	if size := store.Len(); size != 2 {
		t.Fatalf("Len() = %d, want 2", size)
	}
	entries := store.Snapshot(now)
	if len(entries) != 2 {
		t.Fatalf("Snapshot() returned %d entries, want 2", len(entries))
	}

	store.Delete("one")
	if result := store.Get("one", now); result.Available {
		t.Fatalf("expected deleted entry to be unavailable, got %#v", result)
	}

	var nilStore *LastKnownGood[string, int]
	nilStore.Success("x", 1, now)
	nilStore.Failure("x", now)
	nilStore.Delete("x")
	if size := nilStore.Len(); size != 0 {
		t.Fatalf("nil Len() = %d, want 0", size)
	}
	if result := nilStore.Get("x", now); result.Available || result.ConsecutiveFailures != 0 {
		t.Fatalf("nil Get() = %#v, want zero result", result)
	}
	if entries := nilStore.Snapshot(now); entries != nil {
		t.Fatalf("nil Snapshot() = %#v, want nil", entries)
	}
}

func TestLastKnownGoodConcurrentUse(t *testing.T) {
	store := NewLastKnownGood[int, int](time.Minute)
	now := time.Unix(100, 0)

	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				key := worker*1000 + i
				store.Success(key, i, now)
				store.Failure(key, now.Add(time.Second))
				result := store.Get(key, now.Add(2*time.Second))
				if !result.Available || result.Value != i {
					t.Errorf("Get(%d) = %#v, want available value %d", key, result, i)
				}
				if i%100 == 0 {
					_ = store.Snapshot(now)
				}
			}
		}()
	}
	wg.Wait()
}
