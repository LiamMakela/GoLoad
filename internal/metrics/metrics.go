package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/LiamMakela/GoLoad/internal/backend"
)

type Metrics struct {
	TotalRequests  atomic.Uint64
	Successful     atomic.Uint64
	Failed         atomic.Uint64
	ActiveRequests atomic.Int64

	backends []*backend.Backend
}

func New(backends []*backend.Backend) *Metrics {
	return &Metrics{
		backends: backends,
	}
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	fmt.Fprintf(w, "goload_requests_total %d\n", m.TotalRequests.Load())
	fmt.Fprintf(w, "goload_requests_successful %d\n", m.Successful.Load())
	fmt.Fprintf(w, "goload_requests_failed %d\n", m.Failed.Load())
	fmt.Fprintf(w, "goload_requests_active %d\n", m.ActiveRequests.Load())

	for _, b := range m.backends {
		fmt.Fprintf(
			w,
			"goload_backend_alive{backend=%q} %t\n",
			b.URL.String(),
			b.Alive.Load(),
		)

		fmt.Fprintf(
			w,
			"goload_backend_requests_total{backend=%q} %d\n",
			b.URL.String(),
			b.TotalRequests.Load(),
		)

		fmt.Fprintf(
			w,
			"goload_backend_requests_successful{backend=%q} %d\n",
			b.URL.String(),
			b.Successful.Load(),
		)

		fmt.Fprintf(
			w,
			"goload_backend_requests_failed{backend=%q} %d\n",
			b.URL.String(),
			b.Failed.Load(),
		)

		fmt.Fprintf(
			w,
			"goload_backend_active_connections{backend=%q} %d\n",
			b.URL.String(),
			b.ActiveConnections.Load(),
		)

		completed := b.CompletedRequests.Load()

		if completed > 0 {
			avgNs := b.TotalLatencyNs.Load() / completed

			fmt.Fprintf(
				w,
				"goload_backend_average_latency_ns{backend=%q} %d\n",
				b.URL.String(),
				avgNs,
			)
		}
	}
}
