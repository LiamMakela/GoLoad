package balancer

import (
	"testing"

	"github.com/LiamMakela/GoLoad/internal/backend"
)

func newTestBackend(t *testing.T, rawURL string) *backend.Backend {
	t.Helper()

	b, err := backend.New(rawURL)
	if err != nil {
		t.Fatalf(
			"failed to create backend %s: %v",
			rawURL,
			err,
		)
	}

	return b
}

func newTestBalancer(
	t *testing.T,
	backends []*backend.Backend,
	strategy Strategy,
) *Balancer {
	t.Helper()

	b, err := New(
		backends,
		string(strategy),
	)

	if err != nil {
		t.Fatalf(
			"failed to create balancer: %v",
			err,
		)
	}

	return b
}

func TestRoundRobin(t *testing.T) {
	b1 := newTestBackend(
		t,
		"http://backend1:8000",
	)

	b2 := newTestBackend(
		t,
		"http://backend2:8000",
	)

	b3 := newTestBackend(
		t,
		"http://backend3:8000",
	)

	balancer := newTestBalancer(
		t,
		[]*backend.Backend{
			b1,
			b2,
			b3,
		},
		RoundRobin,
	)

	/*
		Current round-robin implementation uses:

			counter.Add(1) % n

		so with a starting counter of zero,
		the first selected index is 1.

		Therefore the current sequence is:

			b2 -> b3 -> b1 -> b2
	*/

	first := balancer.Next()
	second := balancer.Next()
	third := balancer.Next()
	fourth := balancer.Next()

	if first != b1 {
		t.Fatalf("expected b1, got %v", first.URL)
	}

	if second != b2 {
		t.Fatalf("expected b2, got %v", second.URL)
	}

	if third != b3 {
		t.Fatalf("expected b3, got %v", third.URL)
	}

	if fourth != b1 {
		t.Fatalf("expected b1, got %v", fourth.URL)
	}
}

func TestRoundRobinSkipsUnhealthyBackend(
	t *testing.T,
) {
	b1 := newTestBackend(
		t,
		"http://backend1:8000",
	)

	b2 := newTestBackend(
		t,
		"http://backend2:8000",
	)

	b2.Alive.Store(false)

	balancer := newTestBalancer(
		t,
		[]*backend.Backend{
			b1,
			b2,
		},
		RoundRobin,
	)

	for i := 0; i < 5; i++ {
		selected := balancer.Next()

		if selected == nil {
			t.Fatal(
				"expected healthy backend, got nil",
			)
		}

		if selected != b1 {
			t.Fatalf(
				"expected only healthy backend b1, got %v",
				selected.URL,
			)
		}
	}
}

func TestLeastConnections(t *testing.T) {
	b1 := newTestBackend(
		t,
		"http://backend1:8000",
	)

	b2 := newTestBackend(
		t,
		"http://backend2:8000",
	)

	b3 := newTestBackend(
		t,
		"http://backend3:8000",
	)

	b1.ActiveConnections.Store(10)
	b2.ActiveConnections.Store(2)
	b3.ActiveConnections.Store(5)

	balancer := newTestBalancer(
		t,
		[]*backend.Backend{
			b1,
			b2,
			b3,
		},
		LeastConnections,
	)

	selected := balancer.Next()

	if selected == nil {
		t.Fatal(
			"expected backend, got nil",
		)
	}

	if selected != b2 {
		t.Fatalf(
			"expected backend with fewest connections, got %v",
			selected.URL,
		)
	}
}

func TestLeastConnectionsSkipsUnhealthyBackend(
	t *testing.T,
) {
	b1 := newTestBackend(
		t,
		"http://backend1:8000",
	)

	b2 := newTestBackend(
		t,
		"http://backend2:8000",
	)

	b1.ActiveConnections.Store(1)
	b2.ActiveConnections.Store(10)

	// Even though b1 has fewer connections,
	// it must not be selected while unhealthy.
	b1.Alive.Store(false)

	balancer := newTestBalancer(
		t,
		[]*backend.Backend{
			b1,
			b2,
		},
		LeastConnections,
	)

	selected := balancer.Next()

	if selected == nil {
		t.Fatal(
			"expected healthy backend, got nil",
		)
	}

	if selected != b2 {
		t.Fatalf(
			"expected healthy backend b2, got %v",
			selected.URL,
		)
	}
}

func TestConsistentHashSameKey(
	t *testing.T,
) {
	b1 := newTestBackend(
		t,
		"http://backend1:8000",
	)

	b2 := newTestBackend(
		t,
		"http://backend2:8000",
	)

	b3 := newTestBackend(
		t,
		"http://backend3:8000",
	)

	balancer := newTestBalancer(
		t,
		[]*backend.Backend{
			b1,
			b2,
			b3,
		},
		LeastConnections,
	)

	const key = "ABC123"

	first := balancer.ConsistentHash(key)

	if first == nil {
		t.Fatal(
			"expected a backend",
		)
	}

	for i := 0; i < 100; i++ {
		selected :=
			balancer.ConsistentHash(key)

		if selected != first {
			t.Fatalf(
				"expected consistent hash to return %v, got %v",
				first.URL,
				selected.URL,
			)
		}
	}
}

func TestConsistentHashSkipsUnhealthyBackends(
	t *testing.T,
) {
	b1 := newTestBackend(
		t,
		"http://backend1:8000",
	)

	b2 := newTestBackend(
		t,
		"http://backend2:8000",
	)

	b1.Alive.Store(false)

	balancer := newTestBalancer(
		t,
		[]*backend.Backend{
			b1,
			b2,
		},
		LeastConnections,
	)

	keys := []string{
		"GAME01",
		"GAME02",
		"GAME03",
		"GAME04",
	}

	for _, key := range keys {
		selected :=
			balancer.ConsistentHash(key)

		if selected == nil {
			t.Fatalf(
				"expected healthy backend for key %s, got nil",
				key,
			)
		}

		if selected != b2 {
			t.Fatalf(
				"expected healthy backend b2 for key %s, got %v",
				key,
				selected.URL,
			)
		}
	}
}

func TestNoHealthyBackends(
	t *testing.T,
) {
	b1 := newTestBackend(
		t,
		"http://backend1:8000",
	)

	b2 := newTestBackend(
		t,
		"http://backend2:8000",
	)

	b1.Alive.Store(false)
	b2.Alive.Store(false)

	balancer := newTestBalancer(
		t,
		[]*backend.Backend{
			b1,
			b2,
		},
		LeastConnections,
	)

	if selected := balancer.Next(); selected != nil {

		t.Fatalf(
			"expected nil when no backend is healthy, got %v",
			selected.URL,
		)
	}

	if selected :=
		balancer.ConsistentHash("ABC123"); selected != nil {

		t.Fatalf(
			"expected nil hash result when no backend is healthy, got %v",
			selected.URL,
		)
	}
}

func TestNewRejectsUnknownStrategy(
	t *testing.T,
) {
	b1 := newTestBackend(
		t,
		"http://backend1:8000",
	)

	_, err := New(
		[]*backend.Backend{b1},
		"definitely_not_a_strategy",
	)

	if err == nil {
		t.Fatal(
			"expected invalid strategy to return an error",
		)
	}
}
