package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
)

var hopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

func newRequestID() string {
	b := make([]byte, 16)

	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}

	return hex.EncodeToString(b)
}

func clientIP(r *http.Request) string {
	host, _, err :=
		net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		return r.RemoteAddr
	}

	return host
}

func setForwardHeaders(
	req *http.Request,
	original *http.Request,
	requestID string,
) {
	req.Header.Set(
		"X-Request-ID",
		requestID,
	)

	ip := clientIP(original)

	if existing :=
		original.Header.Get(
			"X-Forwarded-For",
		); existing != "" {

		ip = existing + ", " + ip
	}

	req.Header.Set(
		"X-Forwarded-For",
		ip,
	)

	req.Header.Set(
		"X-Forwarded-Host",
		original.Host,
	)

	proto := "http"

	if original.TLS != nil {
		proto = "https"
	}

	req.Header.Set(
		"X-Forwarded-Proto",
		proto,
	)
}

func removeHopHeaders(
	header http.Header,
) {
	for _, name := range strings.Split(
		header.Get("Connection"),
		",",
	) {
		header.Del(
			strings.TrimSpace(name),
		)
	}

	for _, name := range hopHeaders {
		header.Del(name)
	}
}

func copyHeaders(
	dst,
	src http.Header,
) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
