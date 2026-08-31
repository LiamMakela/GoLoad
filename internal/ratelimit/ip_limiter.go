package ratelimit

import (
	"sync"
	"time"
)

type visitor struct {
	limiter  *Limiter
	lastSeen time.Time
}

type IPLimiter struct {
	mu sync.Mutex

	rate  float64
	burst int

	visitors map[string]*visitor
}

func NewIPLimiter(rate float64, burst int) *IPLimiter {
	l := &IPLimiter{
		rate:     rate,
		burst:    burst,
		visitors: make(map[string]*visitor),
	}

	go l.cleanupVisitors()

	return l
}

func (l *IPLimiter) Allow(ip string) bool {
	l.mu.Lock()

	v, exists := l.visitors[ip]

	if !exists {
		v = &visitor{
			limiter:  New(l.rate, l.burst),
			lastSeen: time.Now(),
		}

		l.visitors[ip] = v
	} else {
		v.lastSeen = time.Now()
	}

	limiter := v.limiter

	l.mu.Unlock()

	return limiter.Allow()
}

func (l *IPLimiter) cleanupVisitors() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()

		for ip, v := range l.visitors {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(l.visitors, ip)
			}
		}

		l.mu.Unlock()
	}
}
