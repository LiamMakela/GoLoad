package proxy

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/LiamMakela/GoLoad/internal/backend"
)

func isWebSocket(r *http.Request) bool {
	connection := strings.ToLower(r.Header.Get("Connection"))
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))

	return strings.Contains(connection, "upgrade") &&
		upgrade == "websocket"
}

func (p *Proxy) serveWebSocket(
	w http.ResponseWriter,
	r *http.Request,
	target *backend.Backend,
	requestID string,
) {
	start := time.Now()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		select {
		case <-p.ctx.Done():
			cancel()

		case <-ctx.Done():
		}
	}()

	r = r.Clone(ctx)

	target.ActiveConnections.Add(1)
	target.TotalRequests.Add(1)

	defer target.ActiveConnections.Add(-1)

	proxy := httputil.NewSingleHostReverseProxy(target.URL)

	proxy.ErrorHandler = func(
		w http.ResponseWriter,
		r *http.Request,
		err error,
	) {
		target.Failed.Add(1)
		target.CompletedRequests.Add(1)

		log.Printf(
			"request_id=%s websocket=true backend=%s error=%q",
			requestID,
			target.URL,
			err,
		)

		http.Error(
			w,
			"websocket backend unavailable",
			http.StatusBadGateway,
		)
	}

	originalDirector := proxy.Director

	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.Header.Set(
			"X-Request-ID",
			requestID,
		)

		req.Header.Set(
			"X-Forwarded-For",
			clientIP(r),
		)

		req.Header.Set(
			"X-Forwarded-Host",
			r.Host,
		)

		if r.TLS != nil {
			req.Header.Set(
				"X-Forwarded-Proto",
				"https",
			)
		} else {
			req.Header.Set(
				"X-Forwarded-Proto",
				"http",
			)
		}
	}

	proxy.ServeHTTP(w, r)

	duration := time.Since(start)

	target.Successful.Add(1)
	target.CompletedRequests.Add(1)
	target.TotalLatencyNs.Add(
		uint64(duration.Nanoseconds()),
	)

	log.Printf(
		"request_id=%s websocket=true backend=%s duration=%s",
		requestID,
		target.URL,
		duration,
	)
}

func websocketKey(r *http.Request) string {
	const prefix = "/ws/game/"

	if strings.HasPrefix(r.URL.Path, prefix) {
		return strings.TrimPrefix(r.URL.Path, prefix)
	}

	return ""
}
