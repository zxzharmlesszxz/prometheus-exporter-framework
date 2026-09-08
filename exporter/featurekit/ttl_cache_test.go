package featurekit

import (
	"sync"
	"testing"
	"time"
)

func TestTTLCacheGetSetAndExpiry(t *testing.T) {
	clock := newTestClock(time.Unix(100, 0))
	cache := newTTLCacheWithClock[string, int](10*time.Second, clock.Now)

	cache.Set("answer", 42)
	value, ok := cache.Get("answer")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if value != 42 {
		t.Fatalf("value = %d, want 42", value)
	}

	clock.Advance(10 * time.Second)
	value, ok = cache.Get("answer")
	if ok {
		t.Fatalf("expected expired cache miss, got %d", value)
	}
	if size := cache.Len(); size != 0 {
		t.Fatalf("Len() = %d, want 0", size)
	}
}

func TestTTLCacheDeleteAndClear(t *testing.T) {
	clock := newTestClock(time.Unix(100, 0))
	cache := newTTLCacheWithClock[string, int](time.Minute, clock.Now)

	cache.Set("one", 1)
	cache.Set("two", 2)
	cache.Delete("one")

	if _, ok := cache.Get("one"); ok {
		t.Fatal("expected deleted key to miss")
	}
	if size := cache.Len(); size != 1 {
		t.Fatalf("Len() after Delete = %d, want 1", size)
	}

	cache.Clear()
	if size := cache.Len(); size != 0 {
		t.Fatalf("Len() after Clear = %d, want 0", size)
	}
}

func TestTTLCacheSetRenewsExpiry(t *testing.T) {
	clock := newTestClock(time.Unix(100, 0))
	cache := newTTLCacheWithClock[string, int](10*time.Second, clock.Now)

	cache.Set("answer", 1)
	clock.Advance(9 * time.Second)
	cache.Set("answer", 2)
	clock.Advance(2 * time.Second)

	value, ok := cache.Get("answer")
	if !ok {
		t.Fatal("expected refreshed cache hit")
	}
	if value != 2 {
		t.Fatalf("value = %d, want 2", value)
	}
}

func TestTTLCacheSetWithTTLUsesEntryTTL(t *testing.T) {
	clock := newTestClock(time.Unix(100, 0))
	cache := newTTLCacheWithClock[string, int](0, clock.Now)

	cache.Set("default-disabled", 1)
	if value, ok := cache.Get("default-disabled"); ok {
		t.Fatalf("expected default disabled cache miss, got %d", value)
	}

	cache.SetWithTTL("entry", 2, 10*time.Second)
	clock.Advance(9 * time.Second)
	if value, ok := cache.Get("entry"); !ok || value != 2 {
		t.Fatalf("Get(entry) = %d, %v, want 2, true", value, ok)
	}

	clock.Advance(time.Second)
	if value, ok := cache.Get("entry"); ok {
		t.Fatalf("expected expired entry miss, got %d", value)
	}
}

func TestTTLCacheWithNilClockUsesTimeNow(t *testing.T) {
	cache := newTTLCacheWithClock[string, int](time.Minute, nil)
	cache.Set("answer", 42)

	value, ok := cache.Get("answer")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if value != 42 {
		t.Fatalf("value = %d, want 42", value)
	}
}

func TestTTLCacheDisabledWhenTTLIsNonPositive(t *testing.T) {
	for _, ttl := range []time.Duration{0, -time.Second} {
		cache := NewTTLCache[string, int](ttl)
		cache.Set("answer", 42)

		if value, ok := cache.Get("answer"); ok {
			t.Fatalf("ttl %s: expected disabled cache miss, got %d", ttl, value)
		}
		if size := cache.Len(); size != 0 {
			t.Fatalf("ttl %s: Len() = %d, want 0", ttl, size)
		}
	}
}

func TestTTLCacheSetWithTTLDisabledWhenTTLIsNonPositive(t *testing.T) {
	cache := NewTTLCache[string, int](time.Minute)
	cache.SetWithTTL("zero", 1, 0)
	cache.SetWithTTL("negative", 2, -time.Second)

	if value, ok := cache.Get("zero"); ok {
		t.Fatalf("expected zero ttl miss, got %d", value)
	}
	if value, ok := cache.Get("negative"); ok {
		t.Fatalf("expected negative ttl miss, got %d", value)
	}
}

func TestTTLCacheNilReceiver(t *testing.T) {
	var cache *TTLCache[string, int]

	if value, ok := cache.Get("answer"); ok {
		t.Fatalf("expected nil cache miss, got %d", value)
	}
	cache.Set("answer", 42)
	cache.Delete("answer")
	cache.Clear()
	if size := cache.Len(); size != 0 {
		t.Fatalf("Len() = %d, want 0", size)
	}
}

func TestTTLCacheConcurrentUse(t *testing.T) {
	cache := NewTTLCache[int, int](time.Minute)

	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				key := worker*1000 + i
				cache.Set(key, i)
				if value, ok := cache.Get(key); ok && value != i {
					t.Errorf("Get(%d) = %d, want %d", key, value, i)
				}
				if i%3 == 0 {
					cache.Delete(key)
				}
				if i%100 == 0 {
					_ = cache.Len()
				}
			}
		}()
	}
	wg.Wait()
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock(now time.Time) *testClock {
	return &testClock{now: now}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	c.mu.Unlock()
}
