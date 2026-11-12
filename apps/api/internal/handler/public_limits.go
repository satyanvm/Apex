package handler

// public_limits.go is the abuse protection for anonymous compiles. Every compile
// costs real model spend, so an unauthenticated caller is bounded per client, per
// day across all clients, and in concurrency. State is in memory: the deployment
// runs one API instance, and a restart resetting the counters is acceptable.

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type publicLimits struct {
	mu          sync.Mutex
	now         func() time.Time
	perClient   int
	window      time.Duration
	dailyBudget int
	maxClients  int

	hits     map[string][]time.Time
	day      string
	spent    int
	inFlight int
	maxInFly int
}

type limitVerdict int

const (
	limitAllowed limitVerdict = iota
	limitClient
	limitDaily
	limitBusy
)

func newPublicLimits() *publicLimits {
	return &publicLimits{
		now:         time.Now,
		perClient:   envPositive("PUBLIC_COMPILE_PER_HOUR", 6),
		window:      time.Hour,
		dailyBudget: envPositive("PUBLIC_COMPILE_DAILY_BUDGET", 200),
		maxInFly:    envPositive("PUBLIC_COMPILE_MAX_CONCURRENT", 3),
		maxClients:  100_000,
		hits:        map[string][]time.Time{},
	}
}

func envPositive(name string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name))); err == nil && v > 0 {
		return v
	}
	return def
}

// acquire reserves one compile for client. A compile counts against the limits
// the moment it starts, whether or not it succeeds, because a failed compile has
// already spent money. On limitAllowed the caller must call release.
func (l *publicLimits) acquire(client string) (limitVerdict, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	if day := now.UTC().Format("2006-01-02"); day != l.day {
		l.day, l.spent = day, 0
	}
	if l.spent >= l.dailyBudget {
		tomorrow := now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
		return limitDaily, tomorrow.Sub(now)
	}

	cutoff := now.Add(-l.window)
	recent := l.hits[client][:0:0]
	for _, t := range l.hits[client] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.perClient {
		l.hits[client] = recent
		return limitClient, recent[0].Add(l.window).Sub(now)
	}
	if l.inFlight >= l.maxInFly {
		return limitBusy, 15 * time.Second
	}

	if len(l.hits) >= l.maxClients {
		l.sweep(cutoff)
	}
	l.hits[client] = append(recent, now)
	l.spent++
	l.inFlight++
	return limitAllowed, 0
}

func (l *publicLimits) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.inFlight--
}

// sweep drops clients with no hit inside the window; if the table is still full
// of live clients it starts over rather than grow without bound.
func (l *publicLimits) sweep(cutoff time.Time) {
	for client, times := range l.hits {
		if len(times) == 0 || !times[len(times)-1].After(cutoff) {
			delete(l.hits, client)
		}
	}
	if len(l.hits) >= l.maxClients {
		l.hits = map[string][]time.Time{}
	}
}

// clientKey identifies the caller for rate limiting. The address comes from the
// TCP peer unless PUBLIC_CLIENT_IP_HEADER names a header that the deployment's
// edge proxy always overwrites. That header must never be trusted when the API
// is reachable directly, so it is opt-in. IPv6 clients are grouped by /64, since
// one customer controls a whole /64 and could otherwise rotate past the limit.
func clientKey(r *http.Request) string {
	raw := ""
	if header := strings.TrimSpace(os.Getenv("PUBLIC_CLIENT_IP_HEADER")); header != "" {
		raw = strings.TrimSpace(r.Header.Get(header))
	}
	if raw == "" {
		raw = r.RemoteAddr
		if host, _, err := net.SplitHostPort(raw); err == nil {
			raw = host
		}
	}
	ip := net.ParseIP(raw)
	if ip == nil {
		return raw
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.Mask(net.CIDRMask(64, 128)).String()
}
