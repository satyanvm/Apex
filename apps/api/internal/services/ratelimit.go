package services

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimiter allows each key `limit` actions per rolling window. Counts live in
// memory: the deployment runs one API instance, and a restart resetting them is
// acceptable. A limit of 0 or less disables it.
type RateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	now       func() time.Time
	hits      map[string][]time.Time
	nextSweep time.Time
}

func NewRateLimiter(limit int, window time.Duration, now func() time.Time) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, now: now, hits: map[string][]time.Time{}}
}

// Limit is the number of actions allowed per window (0 = unlimited).
func (l *RateLimiter) Limit() int { return l.limit }

// Take spends one action for key. When none is left it returns false and how
// long until the oldest one frees up.
func (l *RateLimiter) Take(key string) (bool, time.Duration) {
	if l.limit <= 0 {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	live := l.live(l.hits[key], now)
	if len(live) >= l.limit {
		l.hits[key] = live
		return false, live[0].Add(l.window).Sub(now)
	}
	l.hits[key] = append(live, now)
	return true, 0
}

// Refund gives back the latest Take, for an action refused before it did
// anything (for example, a launch whose accounts still need connecting).
func (l *RateLimiter) Refund(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if hits := l.hits[key]; len(hits) > 0 {
		l.hits[key] = hits[:len(hits)-1]
	}
}

// live drops hits older than the window; hits are in time order.
func (l *RateLimiter) live(hits []time.Time, now time.Time) []time.Time {
	cutoff := now.Add(-l.window)
	for i, at := range hits {
		if at.After(cutoff) {
			return hits[i:]
		}
	}
	return nil
}

// sweep forgets keys with no live hits, at most once per window.
func (l *RateLimiter) sweep(now time.Time) {
	if now.Before(l.nextSweep) {
		return
	}
	l.nextSweep = now.Add(l.window)
	for key, hits := range l.hits {
		if len(l.live(hits, now)) == 0 {
			delete(l.hits, key)
		}
	}
}

// WaitText renders a wait as "3h12m" or "4m", rounded up to the minute.
func WaitText(d time.Duration) string {
	d = d.Truncate(time.Minute) + time.Minute
	return strings.TrimSuffix(d.String(), "0s")
}

// limitFromEnv reads a non-negative per-day limit. Unset or unparsable keeps
// the default rather than removing the cap; 0 turns the limit off.
func limitFromEnv(name string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || n < 0 {
		return def
	}
	return n
}
