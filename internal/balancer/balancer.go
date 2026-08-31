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
	counter  atomic.Uint64
	strategy Strategy
}

func New(backends []*backend.Backend, strategy string) (*Balancer, error) {
	s := Strategy(strategy)

	switch s {
	case RoundRobin, LeastConnections:
	default:
		return nil, fmt.Errorf("unknown load balancing strategy: %s", strategy)
	}

	return &Balancer{
		backends: backends,
		strategy: s,
	}, nil
}

func (b *Balancer) Next() *backend.Backend {
	switch b.strategy {
	case RoundRobin:
		return b.roundRobin()

	case LeastConnections:
		return b.leastConnections()

	default:
		return nil
	}
}

func (b *Balancer) roundRobin() *backend.Backend {
	n := len(b.backends)

	if n == 0 {
		return nil
	}

	for i := 0; i < n; i++ {
		index := b.counter.Add(1) % uint64(n)
		target := b.backends[index]

		if target.Alive.Load() {
			return target
		}
	}

	return nil
}

func (b *Balancer) leastConnections() *backend.Backend {
	n := len(b.backends)

	if n == 0 {
		return nil
	}

	start := int(b.counter.Add(1) % uint64(n))

	var selected *backend.Backend
	var minConnections int64

	for i := 0; i < n; i++ {
		index := (start + i) % n
		candidate := b.backends[index]

		if !candidate.Alive.Load() {
			continue
		}

		connections := candidate.ActiveConnections.Load()

		if selected == nil || connections < minConnections {
			selected = candidate
			minConnections = connections
		}
	}

	return selected
}

func (b *Balancer) NextExcluding(
	excluded map[string]bool,
) *backend.Backend {
	switch b.strategy {
	case RoundRobin:
		return b.roundRobinExcluding(excluded)

	case LeastConnections:
		return b.leastConnectionsExcluding(excluded)

	default:
		return nil
	}
}

func (b *Balancer) roundRobinExcluding(
	excluded map[string]bool,
) *backend.Backend {
	n := len(b.backends)

	if n == 0 {
		return nil
	}

	for i := 0; i < n; i++ {
		index := b.counter.Add(1) % uint64(n)
		target := b.backends[index]

		if !target.Alive.Load() {
			continue
		}

		if excluded[target.URL.String()] {
			continue
		}

		return target
	}

	return nil
}

func (b *Balancer) leastConnectionsExcluding(
	excluded map[string]bool,
) *backend.Backend {
	n := len(b.backends)

	if n == 0 {
		return nil
	}

	start := int(b.counter.Add(1) % uint64(n))

	var selected *backend.Backend
	var minConnections int64

	for i := 0; i < n; i++ {
		index := (start + i) % n
		candidate := b.backends[index]

		if !candidate.Alive.Load() {
			continue
		}

		if excluded[candidate.URL.String()] {
			continue
		}

		connections := candidate.ActiveConnections.Load()

		if selected == nil ||
			connections < minConnections {
			selected = candidate
			minConnections = connections
		}
	}

	return selected
}

func (b *Balancer) ConsistentHash(key string) *backend.Backend {
	healthy := make([]*backend.Backend, 0)

	for _, candidate := range b.backends {
		if candidate.Alive.Load() {
			healthy = append(healthy, candidate)
		}
	}

	if len(healthy) == 0 {
		return nil
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(key))

	index := int(h.Sum32()) % len(healthy)

	return healthy[index]
}
