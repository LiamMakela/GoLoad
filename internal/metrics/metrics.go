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

func New(
	backends []*backend.Backend,
) *Metrics {
	return &Metrics{
		backends: backends,
	}
}

func (m *Metrics) ServeHTTP(
	w http.ResponseWriter,
	_ *http.Request,
) {
	w.Header().Set(
		"Content-Type",
		"text/plain",
	)

	fmt.Fprintf(
		w,
		"goload_requests_total %d\n",
		m.TotalRequests.Load(),
	)

	fmt.Fprintf(
		w,
		"goload_requests_successful %d\n",
		m.Successful.Load(),
	)

	fmt.Fprintf(
		w,
		"goload_requests_failed %d\n",
		m.Failed.Load(),
	)

	fmt.Fprintf(
		w,
		"goload_requests_active %d\n",
		m.ActiveRequests.Load(),
	)

	for _, b := range m.backends {
		label := fmt.Sprintf(
			"{backend=%q}",
			b.URL.String(),
		)

		fmt.Fprintf(
			w,
			"goload_backend_alive%s %t\n",
			label,
			b.Alive.Load(),
		)

		fmt.Fprintf(
			w,
			"goload_backend_requests_total%s %d\n",
			label,
			b.TotalRequests.Load(),
		)

		fmt.Fprintf(
			w,
			"goload_backend_requests_successful%s %d\n",
			label,
			b.Successful.Load(),
		)

		fmt.Fprintf(
			w,
			"goload_backend_requests_failed%s %d\n",
			label,
			b.Failed.Load(),
		)

		fmt.Fprintf(
			w,
			"goload_backend_active_connections%s %d\n",
			label,
			b.ActiveConnections.Load(),
		)

		completed :=
			b.CompletedRequests.Load()

		if completed > 0 {
			average :=
				b.TotalLatencyNs.Load() /
					completed

			fmt.Fprintf(
				w,
				"goload_backend_average_latency_ns%s %d\n",
				label,
				average,
			)
		}
	}
}
