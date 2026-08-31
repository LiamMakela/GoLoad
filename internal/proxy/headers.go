package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
)

func newRequestID() string {
	b := make([]byte, 16)

	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}

	return hex.EncodeToString(b)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		return r.RemoteAddr
	}

	return host
}
