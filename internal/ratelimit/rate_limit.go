package ratelimit

import (
	"net"
	"net/http"
	"strings"
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

func newBucket(rate float64, burst int) *bucket {
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

	b.tokens += now.Sub(b.lastRefill).Seconds() * b.rate

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

func NewIPLimiter(rate float64, burst int) *IPLimiter {
	limiter := &IPLimiter{
		rate:     rate,
		burst:    burst,
		visitors: make(map[string]*visitor),
	}

	go limiter.cleanup()

	return limiter
}

func (l *IPLimiter) Allow(ip string) bool {
	l.mu.Lock()

	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{
			bucket: newBucket(l.rate, l.burst),
		}

		l.visitors[ip] = v
	}

	v.lastSeen = time.Now()
	b := v.bucket

	l.mu.Unlock()

	return b.allow()
}

func (l *IPLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(clientIP(r)) {
			rateLimitResponse(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *IPLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)

		l.mu.Lock()

		for ip, v := range l.visitors {
			if v.lastSeen.Before(cutoff) {
				delete(l.visitors, ip)
			}
		}

		l.mu.Unlock()
	}
}

// RouteLimiter applies different per-IP limits depending on the request.
//
// Create game:  10/minute
// Join game:    20/minute
// WebSocket:    10 connection attempts/minute
// General HTTP: 30/second, burst 60
type RouteLimiter struct {
	createGame *IPLimiter
	joinGame   *IPLimiter
	webSocket  *IPLimiter
	general    *IPLimiter
}

func NewRouteLimiter() *RouteLimiter {
	return &RouteLimiter{
		createGame: NewIPLimiter(10.0/60.0, 10),
		joinGame:   NewIPLimiter(20.0/60.0, 20),
		webSocket:  NewIPLimiter(10.0/60.0, 10),
		general:    NewIPLimiter(30, 60),
	}
}

func (l *RouteLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)

		var allowed bool

		switch {
		case isWebSocket(r):
			allowed = l.webSocket.Allow(ip)

		case isCreateGame(r):
			allowed = l.createGame.Allow(ip)

		case isJoinGame(r):
			allowed = l.joinGame.Allow(ip)

		default:
			allowed = l.general.Allow(ip)
		}

		if !allowed {
			rateLimitResponse(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isCreateGame(r *http.Request) bool {
	return r.Method == http.MethodPost &&
		r.URL.Path == "/games"
}

func isJoinGame(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}

	parts := strings.Split(
		strings.Trim(r.URL.Path, "/"),
		"/",
	)

	return len(parts) == 3 &&
		parts[0] == "games" &&
		parts[2] == "join"
}

func isWebSocket(r *http.Request) bool {
	return strings.EqualFold(
		r.Header.Get("Upgrade"),
		"websocket",
	) &&
		strings.Contains(
			strings.ToLower(r.Header.Get("Connection")),
			"upgrade",
		)
}

func rateLimitResponse(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "1")

	http.Error(
		w,
		"rate limit exceeded",
		http.StatusTooManyRequests,
	)
}

func clientIP(r *http.Request) string {
	// Caddy sets X-Forwarded-For when proxying requests.
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip := strings.TrimSpace(
			strings.Split(forwarded, ",")[0],
		)

		if ip != "" {
			return ip
		}
	}

	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
