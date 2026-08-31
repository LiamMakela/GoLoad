package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"time"

	"github.com/LiamMakela/GoLoad/internal/backend"
	"github.com/LiamMakela/GoLoad/internal/balancer"
	"github.com/LiamMakela/GoLoad/internal/metrics"
)

type Proxy struct {
	balancer *balancer.Balancer
	metrics  *metrics.Metrics
	timeout  time.Duration
	retries  int
	ctx      context.Context
}

func New(
	b *balancer.Balancer,
	m *metrics.Metrics,
	timeout time.Duration,
	retries int,
	ctx context.Context,
) *Proxy {
	return &Proxy{
		balancer: b,
		metrics:  m,
		timeout:  timeout,
		retries:  retries,
		ctx:      ctx,
	}
}

func (p *Proxy) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	requestID := r.Header.Get("X-Request-ID")

	if requestID == "" {
		requestID = newRequestID()
	}

	w.Header().Set("X-Request-ID", requestID)

	p.metrics.TotalRequests.Add(1)
	p.metrics.ActiveRequests.Add(1)

	defer p.metrics.ActiveRequests.Add(-1)

	if isWebSocket(r) {
		key := websocketKey(r)

		var target *backend.Backend

		if key != "" {
			target = p.balancer.ConsistentHash(key)
		} else {
			target = p.balancer.Next()
		}

		if target == nil {
			p.metrics.Failed.Add(1)

			http.Error(
				w,
				"no healthy backends available",
				http.StatusServiceUnavailable,
			)

			return
		}

		p.serveWebSocket(
			w,
			r,
			target,
			requestID,
		)

		return
	}

	attempts := 1

	if r.Method == http.MethodGet ||
		r.Method == http.MethodHead {
		attempts += p.retries
	}

	attempted := make(map[string]bool)

	for attempt := 1; attempt <= attempts; attempt++ {
		target := p.balancer.NextExcluding(attempted)

		if target == nil {
			break
		}

		attempted[target.URL.String()] = true

		err := p.tryBackend(
			w,
			r,
			target,
			attempt,
			requestID,
		)

		if err == nil {
			return
		}

		log.Printf(
			"request_id=%s retrying method=%s path=%s attempt=%d error=%v",
			requestID,
			r.Method,
			r.URL.Path,
			attempt,
			err,
		)
	}

	p.metrics.Failed.Add(1)

	http.Error(
		w,
		"all backend attempts failed",
		http.StatusBadGateway,
	)
}

func (p *Proxy) tryBackend(
	w http.ResponseWriter,
	r *http.Request,
	target *backend.Backend,
	attempt int,
	requestID string,
) error {
	attemptStart := time.Now()
	target.ActiveConnections.Add(1)
	target.TotalRequests.Add(1)

	defer target.ActiveConnections.Add(-1)

	ctx, cancel := context.WithTimeout(
		r.Context(),
		p.timeout,
	)

	defer cancel()

	req := r.Clone(ctx)

	req.Header.Set("X-Request-ID", requestID)

	forwardedFor := r.Header.Get("X-Forwarded-For")
	currentIP := clientIP(r)

	if forwardedFor == "" {
		req.Header.Set("X-Forwarded-For", currentIP)
	} else {
		req.Header.Set(
			"X-Forwarded-For",
			forwardedFor+", "+currentIP,
		)
	}

	req.Header.Set(
		"X-Forwarded-Host",
		r.Host,
	)

	if r.TLS != nil {
		req.Header.Set("X-Forwarded-Proto", "https")
	} else {
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	recorder := httptest.NewRecorder()

	reverseProxy := httputil.NewSingleHostReverseProxy(
		target.URL,
	)

	var proxyErr error

	reverseProxy.ErrorHandler = func(
		rw http.ResponseWriter,
		req *http.Request,
		err error,
	) {
		proxyErr = err
	}

	reverseProxy.ServeHTTP(recorder, req)

	if proxyErr != nil {
		target.Failed.Add(1)
		target.CompletedRequests.Add(1)

		duration := time.Since(attemptStart)
		target.TotalLatencyNs.Add(
			uint64(duration.Nanoseconds()),
		)

		log.Printf(
			"request_id=%s method=%s path=%s attempt=%d backend=%s error=%q",
			requestID,
			r.Method,
			r.URL.Path,
			attempt,
			target.URL,
			proxyErr,
		)

		return proxyErr
	}

	result := recorder.Result()
	defer result.Body.Close()

	if result.StatusCode >= 500 {
		duration := time.Since(attemptStart)

		target.Failed.Add(1)
		target.CompletedRequests.Add(1)
		target.TotalLatencyNs.Add(
			uint64(duration.Nanoseconds()),
		)

		log.Printf(
			"request_id=%s method=%s path=%s status=%d duration=%s backend=%s attempt=%d",
			requestID,
			r.Method,
			r.URL.Path,
			result.StatusCode,
			duration,
			target.URL,
			attempt,
		)

		return fmt.Errorf(
			"backend returned status %d",
			result.StatusCode,
		)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		target.Failed.Add(1)
		target.CompletedRequests.Add(1)

		return err
	}

	for key, values := range result.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(result.StatusCode)

	if _, err := w.Write(body); err != nil {
		return err
	}

	duration := time.Since(attemptStart)

	target.Successful.Add(1)
	target.CompletedRequests.Add(1)
	target.TotalLatencyNs.Add(
		uint64(duration.Nanoseconds()),
	)

	p.metrics.Successful.Add(1)

	log.Printf(
		"method=%s path=%s status=%d duration=%s backend=%s attempt=%d",
		r.Method,
		r.URL.Path,
		result.StatusCode,
		duration,
		target.URL,
		attempt,
	)

	return nil
}
