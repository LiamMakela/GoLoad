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
	ticker := time.NewTicker(c.interval)

	go func() {
		defer ticker.Stop()

		// Run one check immediately.
		c.checkAll()

		for {
			select {
			case <-ticker.C:
				c.checkAll()

			case <-ctx.Done():
				log.Println("health checker stopped")
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
	resp, err := c.client.Get(b.URL.String() + "/health")

	if err != nil {
		if b.Alive.Swap(false) {
			log.Printf("backend %s is DOWN", b.URL)
		}

		return
	}

	defer resp.Body.Close()

	healthy := resp.StatusCode >= 200 &&
		resp.StatusCode < 300

	wasAlive := b.Alive.Swap(healthy)

	if healthy && !wasAlive {
		log.Printf("backend %s is UP", b.URL)
	}

	if !healthy && wasAlive {
		log.Printf("backend %s is DOWN", b.URL)
	}
}
