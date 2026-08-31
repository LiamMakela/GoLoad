package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGameKeyJoinRoute(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/games/ABC123/join",
		nil,
	)

	key := gameKey(request)

	if key != "ABC123" {
		t.Fatalf(
			"expected ABC123, got %q",
			key,
		)
	}
}

func TestGameKeyStateRoute(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/games/XYZ789",
		nil,
	)

	key := gameKey(request)

	if key != "XYZ789" {
		t.Fatalf(
			"expected XYZ789, got %q",
			key,
		)
	}
}

func TestGameKeyWebSocketRoute(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/games/GAME42/ws/player-123",
		nil,
	)

	key := gameKey(request)

	if key != "GAME42" {
		t.Fatalf(
			"expected GAME42, got %q",
			key,
		)
	}
}

func TestGameKeyCreateGameReturnsEmpty(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/games",
		nil,
	)

	key := gameKey(request)

	if key != "" {
		t.Fatalf(
			"expected empty key, got %q",
			key,
		)
	}
}

func TestGameKeyUnrelatedRouteReturnsEmpty(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/health",
		nil,
	)

	key := gameKey(request)

	if key != "" {
		t.Fatalf(
			"expected empty key, got %q",
			key,
		)
	}
}

func TestWebSocketKey(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/games/ABC123/ws/player-456",
		nil,
	)

	key := websocketKey(request)

	if key != "ABC123" {
		t.Fatalf(
			"expected ABC123, got %q",
			key,
		)
	}
}

func TestWebSocketKeyRejectsNormalGameRoute(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/games/ABC123",
		nil,
	)

	key := websocketKey(request)

	if key != "" {
		t.Fatalf(
			"expected empty key, got %q",
			key,
		)
	}
}

func TestWebSocketDetection(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/games/ABC123/ws/player-456",
		nil,
	)

	request.Header.Set(
		"Connection",
		"Upgrade",
	)

	request.Header.Set(
		"Upgrade",
		"websocket",
	)

	if !isWebSocket(request) {
		t.Fatal(
			"expected request to be detected as websocket",
		)
	}
}

func TestNormalHTTPRequestIsNotWebSocket(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/health",
		nil,
	)

	if isWebSocket(request) {
		t.Fatal(
			"expected regular HTTP request not to be websocket",
		)
	}
}
