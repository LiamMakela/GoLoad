package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	mu sync.Mutex

	rate       float64
	capacity   float64
	tokens     float64
	lastRefill time.Time
}

func newBucket(
	rate float64,
	burst int,
) *bucket {
	return &bucket{
		rate:       rate,
		capacity:   float64(burst),
		tokens:     float64(burst),
		lastRefill: time.Now(),
	}
}

func (b *bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()

	b.tokens +=
		now.Sub(b.lastRefill).Seconds() *
			b.rate

	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}

	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}

	b.tokens--

	return true
}

type visitor struct {
	bucket   *bucket
	lastSeen time.Time
}

type IPLimiter struct {
	mu sync.Mutex

	rate     float64
	burst    int
	visitors map[string]*visitor
}

func NewIPLimiter(
	rate float64,
	burst int,
) *IPLimiter {
	l := &IPLimiter{
		rate:     rate,
		burst:    burst,
		visitors: make(map[string]*visitor),
	}

	go l.cleanup()

	return l
}

func (l *IPLimiter) Allow(ip string) bool {
	l.mu.Lock()

	v, ok := l.visitors[ip]

	if !ok {
		v = &visitor{
			bucket: newBucket(
				l.rate,
				l.burst,
			),
		}

		l.visitors[ip] = v
	}

	v.lastSeen = time.Now()
	b := v.bucket

	l.mu.Unlock()

	return b.allow()
}

func (l *IPLimiter) Middleware(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if !l.Allow(clientIP(r)) {
				w.Header().Set(
					"Retry-After",
					"1",
				)

				http.Error(
					w,
					"rate limit exceeded",
					http.StatusTooManyRequests,
				)

				return
			}

			next.ServeHTTP(w, r)
		},
	)
}

func (l *IPLimiter) cleanup() {
	ticker := time.NewTicker(
		5 * time.Minute,
	)

	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(
			-10 * time.Minute,
		)

		l.mu.Lock()

		for ip, v := range l.visitors {
			if v.lastSeen.Before(cutoff) {
				delete(l.visitors, ip)
			}
		}

		l.mu.Unlock()
	}
}

func clientIP(r *http.Request) string {
	host, _, err :=
		net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		return r.RemoteAddr
	}

	return host
}
