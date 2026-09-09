package featurekit

import (
	"sync"
	"time"
)

// LastKnownGood keeps the last successful value per key while tracking failed
// refresh attempts separately from data availability.
type LastKnownGood[K comparable, V any] struct {
	mu         sync.RWMutex
	staleAfter time.Duration
	entries    map[K]lastKnownGoodEntry[V]
}

type lastKnownGoodEntry[V any] struct {
	value               V
	available           bool
	lastSuccess         time.Time
	lastAttempt         time.Time
	consecutiveFailures uint64
}

type LastKnownGoodResult[V any] struct {
	Value               V
	Available           bool
	Stale               bool
	LastSuccess         time.Time
	LastAttempt         time.Time
	ConsecutiveFailures uint64
}

type LastKnownGoodEntry[K comparable, V any] struct {
	Key K
	LastKnownGoodResult[V]
}

func NewLastKnownGood[K comparable, V any](staleAfter time.Duration) *LastKnownGood[K, V] {
	return &LastKnownGood[K, V]{
		staleAfter: staleAfter,
		entries:    make(map[K]lastKnownGoodEntry[V]),
	}
}

func (l *LastKnownGood[K, V]) Success(key K, value V, now time.Time) {
	if l == nil {
		return
	}

	l.mu.Lock()
	l.entries[key] = lastKnownGoodEntry[V]{
		value:       value,
		available:   true,
		lastSuccess: now,
		lastAttempt: now,
	}
	l.mu.Unlock()
}

func (l *LastKnownGood[K, V]) Failure(key K, now time.Time) {
	if l == nil {
		return
	}

	l.mu.Lock()
	entry := l.entries[key]
	entry.lastAttempt = now
	entry.consecutiveFailures++
	l.entries[key] = entry
	l.mu.Unlock()
}

func (l *LastKnownGood[K, V]) Delete(key K) {
	if l == nil {
		return
	}

	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

func (l *LastKnownGood[K, V]) Get(key K, now time.Time) LastKnownGoodResult[V] {
	if l == nil {
		return LastKnownGoodResult[V]{}
	}

	l.mu.RLock()
	entry, ok := l.entries[key]
	l.mu.RUnlock()
	if !ok {
		return LastKnownGoodResult[V]{}
	}
	return l.result(entry, now)
}

func (l *LastKnownGood[K, V]) Snapshot(now time.Time) []LastKnownGoodEntry[K, V] {
	if l == nil {
		return nil
	}

	l.mu.RLock()
	entries := make([]LastKnownGoodEntry[K, V], 0, len(l.entries))
	for key, entry := range l.entries {
		entries = append(entries, LastKnownGoodEntry[K, V]{
			Key:                 key,
			LastKnownGoodResult: l.result(entry, now),
		})
	}
	l.mu.RUnlock()
	return entries
}

func (l *LastKnownGood[K, V]) Len() int {
	if l == nil {
		return 0
	}

	l.mu.RLock()
	size := len(l.entries)
	l.mu.RUnlock()
	return size
}

func (l *LastKnownGood[K, V]) result(entry lastKnownGoodEntry[V], now time.Time) LastKnownGoodResult[V] {
	return LastKnownGoodResult[V]{
		Value:               entry.value,
		Available:           entry.available,
		Stale:               entry.available && l.staleAfter > 0 && !now.Before(entry.lastSuccess.Add(l.staleAfter)),
		LastSuccess:         entry.lastSuccess,
		LastAttempt:         entry.lastAttempt,
		ConsecutiveFailures: entry.consecutiveFailures,
	}
}
