package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/LiamMakela/GoLoad/internal/ratelimit"
)

func clientIP(r *http.Request) string {
	// If GoLoad is behind another trusted proxy later,
	// this header may contain the original client IP.
	forwarded := r.Header.Get("X-Forwarded-For")

	if forwarded != "" {
		parts := strings.Split(forwarded, ",")

		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		return r.RemoteAddr
	}

	return host
}

func RateLimit(
	next http.Handler,
	limiter *ratelimit.IPLimiter,
) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)

			if !limiter.Allow(ip) {
				w.Header().Set("Retry-After", "1")

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
