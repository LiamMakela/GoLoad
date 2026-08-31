package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
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
	requestID :=
		r.Header.Get("X-Request-ID")

	if requestID == "" {
		requestID = newRequestID()
	}

	w.Header().Set(
		"X-Request-ID",
		requestID,
	)

	p.metrics.TotalRequests.Add(1)
	p.metrics.ActiveRequests.Add(1)

	defer p.metrics.ActiveRequests.Add(-1)

	if isWebSocket(r) {
		target :=
			p.balancer.Next()

		if key := websocketKey(r); key != "" {

			target =
				p.balancer.ConsistentHash(
					key,
				)
		}

		if target == nil {
			p.fail(
				w,
				http.StatusServiceUnavailable,
				"no healthy backends available",
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

	// Game-scoped HTTP traffic follows the same
	// backend affinity as its WebSocket connections.
	if key := gameKey(r); key != "" {
		target :=
			p.balancer.ConsistentHash(
				key,
			)

		if target == nil {
			p.fail(
				w,
				http.StatusServiceUnavailable,
				"no healthy backends available",
			)

			return
		}

		if err := p.tryBackend(
			w,
			r,
			target,
			1,
			requestID,
		); err != nil {

			p.fail(
				w,
				http.StatusBadGateway,
				"backend unavailable",
			)
		}

		return
	}

	p.serveBalanced(
		w,
		r,
		requestID,
	)
}

func (p *Proxy) serveBalanced(
	w http.ResponseWriter,
	r *http.Request,
	requestID string,
) {
	attempts := 1

	if r.Method == http.MethodGet ||
		r.Method == http.MethodHead {

		attempts += p.retries
	}

	excluded := make(map[string]bool)

	for attempt := 1; attempt <= attempts; attempt++ {

		target :=
			p.balancer.NextExcluding(
				excluded,
			)

		if target == nil {
			break
		}

		excluded[target.URL.String()] = true

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
			"request_id=%s retry=%d backend=%s error=%v",
			requestID,
			attempt,
			target.URL,
			err,
		)
	}

	p.fail(
		w,
		http.StatusBadGateway,
		"all backend attempts failed",
	)
}

func (p *Proxy) tryBackend(
	w http.ResponseWriter,
	r *http.Request,
	target *backend.Backend,
	attempt int,
	requestID string,
) error {
	start := time.Now()
	success := false

	target.BeginRequest()

	defer func() {
		target.FinishRequest(
			success,
			time.Since(start),
		)
	}()

	ctx, cancel :=
		context.WithTimeout(
			r.Context(),
			p.timeout,
		)

	defer cancel()

	req := r.Clone(ctx)

	req.URL.Scheme =
		target.URL.Scheme

	req.URL.Host =
		target.URL.Host

	req.RequestURI = ""

	setForwardHeaders(
		req,
		r,
		requestID,
	)

	removeHopHeaders(req.Header)

	resp, err :=
		http.DefaultTransport.RoundTrip(
			req,
		)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		_, _ = io.Copy(
			io.Discard,
			resp.Body,
		)

		return fmt.Errorf(
			"backend returned %d",
			resp.StatusCode,
		)
	}

	removeHopHeaders(resp.Header)
	copyHeaders(
		w.Header(),
		resp.Header,
	)

	w.WriteHeader(resp.StatusCode)

	// At this point the backend returned a valid
	// response. A client disconnect should not cause
	// a retry against another backend.
	success = true
	p.metrics.Successful.Add(1)

	if _, err := io.Copy(
		w,
		resp.Body,
	); err != nil {

		log.Printf(
			"request_id=%s client_write_error=%v",
			requestID,
			err,
		)
	}

	log.Printf(
		"method=%s path=%s status=%d backend=%s attempt=%d duration=%s",
		r.Method,
		r.URL.Path,
		resp.StatusCode,
		target.URL,
		attempt,
		time.Since(start),
	)

	return nil
}

func (p *Proxy) fail(
	w http.ResponseWriter,
	status int,
	message string,
) {
	p.metrics.Failed.Add(1)

	http.Error(
		w,
		message,
		status,
	)
}
