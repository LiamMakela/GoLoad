package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu sync.Mutex

	rate       float64
	capacity   float64
	tokens     float64
	lastRefill time.Time
}

func New(rate float64, burst int) *Limiter {
	return &Limiter{
		rate:       rate,
		capacity:   float64(burst),
		tokens:     float64(burst),
		lastRefill: time.Now(),
	}
}

func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	elapsed := now.Sub(l.lastRefill).Seconds()

	l.tokens += elapsed * l.rate

	if l.tokens > l.capacity {
		l.tokens = l.capacity
	}

	l.lastRefill = now

	if l.tokens < 1 {
		return false
	}

	l.tokens--

	return true
}
