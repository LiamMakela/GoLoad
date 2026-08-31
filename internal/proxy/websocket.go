package proxy

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/LiamMakela/GoLoad/internal/backend"
)

func isWebSocket(r *http.Request) bool {
	return strings.Contains(
		strings.ToLower(
			r.Header.Get("Connection"),
		),
		"upgrade",
	) &&
		strings.EqualFold(
			r.Header.Get("Upgrade"),
			"websocket",
		)
}

func (p *Proxy) serveWebSocket(
	w http.ResponseWriter,
	r *http.Request,
	target *backend.Backend,
	requestID string,
) {
	start := time.Now()
	success := true

	ctx, cancel :=
		context.WithCancel(
			r.Context(),
		)

	defer cancel()

	stopShutdown :=
		context.AfterFunc(
			p.ctx,
			cancel,
		)

	defer stopShutdown()

	r = r.Clone(ctx)

	target.BeginRequest()

	defer func() {
		target.FinishRequest(
			success,
			time.Since(start),
		)

		if success {
			p.metrics.Successful.Add(1)
		} else {
			p.metrics.Failed.Add(1)
		}
	}()

	reverseProxy :=
		httputil.NewSingleHostReverseProxy(
			target.URL,
		)

	director := reverseProxy.Director

	reverseProxy.Director =
		func(req *http.Request) {
			director(req)

			setForwardHeaders(
				req,
				r,
				requestID,
			)
		}

	var proxyErr error

	reverseProxy.ErrorHandler =
		func(
			w http.ResponseWriter,
			_ *http.Request,
			err error,
		) {
			proxyErr = err

			// Client disconnects and server shutdowns
			// are normal for long-lived WebSockets.
			if errors.Is(
				err,
				context.Canceled,
			) {
				return
			}

			http.Error(
				w,
				"websocket backend unavailable",
				http.StatusBadGateway,
			)
		}

	reverseProxy.ServeHTTP(w, r)

	if proxyErr != nil &&
		!errors.Is(
			proxyErr,
			context.Canceled,
		) {

		success = false

		log.Printf(
			"request_id=%s websocket_backend=%s error=%v",
			requestID,
			target.URL,
			proxyErr,
		)

		return
	}

	log.Printf(
		"request_id=%s websocket_backend=%s duration=%s",
		requestID,
		target.URL,
		time.Since(start),
	)
}

func websocketKey(
	r *http.Request,
) string {
	parts :=
		strings.Split(
			strings.Trim(
				r.URL.Path,
				"/",
			),
			"/",
		)

	if len(parts) >= 4 &&
		parts[0] == "games" &&
		parts[2] == "ws" {

		return parts[1]
	}

	return ""
}

func gameKey(
	r *http.Request,
) string {
	parts :=
		strings.Split(
			strings.Trim(
				r.URL.Path,
				"/",
			),
			"/",
		)

	if len(parts) >= 2 &&
		parts[0] == "games" {

		return parts[1]
	}

	return ""
}
