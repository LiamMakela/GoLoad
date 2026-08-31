package health

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/LiamMakela/GoLoad/internal/backend"
)

type Checker struct {
	backends []*backend.Backend
	client   *http.Client
	interval time.Duration
}

func New(
	backends []*backend.Backend,
	interval time.Duration,
	timeout time.Duration,
) *Checker {
	return &Checker{
		backends: backends,
		interval: interval,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Checker) Start(ctx context.Context) {
	go func() {
		c.checkAll()

		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.checkAll()

			case <-ctx.Done():
				return
			}
		}
	}()
}

func (c *Checker) checkAll() {
	for _, b := range c.backends {
		go c.check(b)
	}
}

func (c *Checker) check(b *backend.Backend) {
	resp, err := c.client.Get(
		b.URL.String() + "/health",
	)

	healthy := err == nil &&
		resp != nil &&
		resp.StatusCode >= 200 &&
		resp.StatusCode < 300

	if resp != nil {
		resp.Body.Close()
	}

	wasHealthy := b.Alive.Swap(healthy)

	if healthy == wasHealthy {
		return
	}

	state := "DOWN"

	if healthy {
		state = "UP"
	}

	log.Printf(
		"backend %s is %s",
		b.URL,
		state,
	)
}
