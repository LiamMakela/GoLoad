package backend

import (
	"net/url"
	"sync/atomic"
	"time"
)

type Backend struct {
	URL *url.URL

	Alive             atomic.Bool
	ActiveConnections atomic.Int64
	TotalRequests     atomic.Uint64
	CompletedRequests atomic.Uint64
	Successful        atomic.Uint64
	Failed            atomic.Uint64
	TotalLatencyNs    atomic.Uint64
}

func New(rawURL string) (*Backend, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	b := &Backend{URL: u}
	b.Alive.Store(true)

	return b, nil
}

func (b *Backend) BeginRequest() {
	b.ActiveConnections.Add(1)
	b.TotalRequests.Add(1)
}

func (b *Backend) FinishRequest(success bool, duration time.Duration) {
	b.ActiveConnections.Add(-1)
	b.CompletedRequests.Add(1)
	b.TotalLatencyNs.Add(uint64(duration.Nanoseconds()))

	if success {
		b.Successful.Add(1)
	} else {
		b.Failed.Add(1)
	}
}
