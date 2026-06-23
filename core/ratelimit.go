package core

import (
	"sync"
	"time"
)

// Rate limiter: best-effort per-IP token bucket sitting at the very front of
// the serve pipeline (before file lookup / kill switch). When a scanner storms
// the server from one address, we 429 every request past the budget instead of
// burning DB lookups, filter evaluations, and (worst case) GeoIP fetches on
// every hit.
//
// Per-IP, in-process only — a multi-instance deployment would need a shared
// store, but pwndrop is typically a single VPS so this is the right altitude.
// Buckets that have been idle long enough are reaped by the regular cleanup
// loop (RateLimitSweepIdle).

const (
	rateLimitDefaultPerMin   = 60
	rateLimitIdleEvictAfter  = 10 * time.Minute
	rateLimitSweepMaxBuckets = 50000
)

type rlBucket struct {
	tokens float64
	last   time.Time
}

var (
	rlMu      sync.Mutex
	rlBuckets = map[string]*rlBucket{}
)

// RateLimitTake returns true when the request from ip is within budget. perMin
// <= 0 falls back to the default. The caller is responsible for checking the
// master switch before calling.
func RateLimitTake(ip string, perMin int) bool {
	if perMin <= 0 {
		perMin = rateLimitDefaultPerMin
	}
	cap := float64(perMin)
	rate := cap / 60.0 // tokens per second

	rlMu.Lock()
	defer rlMu.Unlock()

	now := time.Now()
	b, ok := rlBuckets[ip]
	if !ok {
		// New buckets start full so a one-off legit request from a fresh IP
		// is not penalised by zero accrued tokens.
		b = &rlBucket{tokens: cap, last: now}
		rlBuckets[ip] = b
	} else {
		elapsed := now.Sub(b.last).Seconds()
		if elapsed > 0 {
			b.tokens += elapsed * rate
			if b.tokens > cap {
				b.tokens = cap
			}
			b.last = now
		}
	}
	if b.tokens < 1.0 {
		return false
	}
	b.tokens -= 1.0
	return true
}

// RateLimitSweepIdle drops buckets that haven't been touched recently. Called
// from the regular cleanup tick. Also enforces a hard cap on the map size to
// prevent a slow-trickle adversary from filling memory with sub-budget IPs.
func RateLimitSweepIdle() int {
	cutoff := time.Now().Add(-rateLimitIdleEvictAfter)
	rlMu.Lock()
	defer rlMu.Unlock()
	dropped := 0
	for ip, b := range rlBuckets {
		if b.last.Before(cutoff) {
			delete(rlBuckets, ip)
			dropped++
		}
	}
	// Hard-cap fallback: if we're still oversized after the idle sweep, evict
	// a random batch (map iteration order is randomised in Go).
	if len(rlBuckets) > rateLimitSweepMaxBuckets {
		excess := len(rlBuckets) - rateLimitSweepMaxBuckets
		for ip := range rlBuckets {
			if excess <= 0 {
				break
			}
			delete(rlBuckets, ip)
			excess--
			dropped++
		}
	}
	return dropped
}
