package balancer

import (
	"fmt"
	"hash/fnv"
	"sync/atomic"

	"github.com/LiamMakela/GoLoad/internal/backend"
)

type Strategy string

const (
	RoundRobin       Strategy = "round_robin"
	LeastConnections Strategy = "least_connections"
)

type Balancer struct {
	backends []*backend.Backend
	strategy Strategy
	counter  atomic.Uint64
}

func New(backends []*backend.Backend, strategy string) (*Balancer, error) {
	s := Strategy(strategy)

	if s != RoundRobin && s != LeastConnections {
		return nil, fmt.Errorf(
			"unknown load balancing strategy: %s",
			strategy,
		)
	}

	return &Balancer{
		backends: backends,
		strategy: s,
	}, nil
}

func (b *Balancer) Next() *backend.Backend {
	return b.NextExcluding(nil)
}

func (b *Balancer) NextExcluding(
	excluded map[string]bool,
) *backend.Backend {
	switch b.strategy {
	case RoundRobin:
		return b.roundRobin(excluded)

	case LeastConnections:
		return b.leastConnections(excluded)

	default:
		return nil
	}
}

func (b *Balancer) roundRobin(
	excluded map[string]bool,
) *backend.Backend {
	n := len(b.backends)

	if n == 0 {
		return nil
	}

	start := b.counter.Add(1) - 1

	for i := 0; i < n; i++ {
		index := int(
			(start + uint64(i)) % uint64(n),
		)

		candidate := b.backends[index]

		if available(candidate, excluded) {
			return candidate
		}
	}

	return nil
}

func (b *Balancer) leastConnections(
	excluded map[string]bool,
) *backend.Backend {
	n := len(b.backends)

	if n == 0 {
		return nil
	}

	start := int(
		(b.counter.Add(1) - 1) % uint64(n),
	)

	var selected *backend.Backend
	var fewest int64

	for i := 0; i < n; i++ {
		candidate := b.backends[(start+i)%n]

		if !available(candidate, excluded) {
			continue
		}

		connections :=
			candidate.ActiveConnections.Load()

		if selected == nil ||
			connections < fewest {

			selected = candidate
			fewest = connections
		}
	}

	return selected
}

func available(
	b *backend.Backend,
	excluded map[string]bool,
) bool {
	if !b.Alive.Load() {
		return false
	}

	return excluded == nil ||
		!excluded[b.URL.String()]
}

// ConsistentHash uses rendezvous hashing so the same key
// consistently prefers the same healthy backend.
func (b *Balancer) ConsistentHash(
	key string,
) *backend.Backend {
	var selected *backend.Backend
	var highestScore uint64

	for _, candidate := range b.backends {
		if !candidate.Alive.Load() {
			continue
		}

		score := hash(
			key,
			candidate.URL.String(),
		)

		if selected == nil ||
			score > highestScore {

			selected = candidate
			highestScore = score
		}
	}

	return selected
}

func hash(key, backendURL string) uint64 {
	h := fnv.New64a()

	_, _ = h.Write([]byte(key))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(backendURL))

	return h.Sum64()
}
