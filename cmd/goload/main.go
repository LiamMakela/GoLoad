package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LiamMakela/GoLoad/internal/backend"
	"github.com/LiamMakela/GoLoad/internal/balancer"
	"github.com/LiamMakela/GoLoad/internal/config"
	"github.com/LiamMakela/GoLoad/internal/health"
	"github.com/LiamMakela/GoLoad/internal/metrics"
	"github.com/LiamMakela/GoLoad/internal/proxy"
	"github.com/LiamMakela/GoLoad/internal/ratelimit"
)

func main() {
	cfg, err :=
		config.Load(
			"configs/config.yaml",
		)

	if err != nil {
		log.Fatal(err)
	}

	backends, err :=
		createBackends(cfg.Backends)

	if err != nil {
		log.Fatal(err)
	}

	lb, err := balancer.New(
		backends,
		cfg.LoadBalancer.Strategy,
	)

	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	health.New(
		backends,
		cfg.Health.Interval,
		cfg.Health.Timeout,
	).Start(ctx)

	m := metrics.New(backends)

	var handler http.Handler = proxy.New(
		lb,
		m,
		cfg.Proxy.Timeout,
		cfg.Proxy.Retries,
		ctx,
	)

	if cfg.RateLimit.Enabled {
		handler =
			ratelimit.NewIPLimiter(
				cfg.RateLimit.RequestsPerSecond,
				cfg.RateLimit.Burst,
			).Middleware(handler)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", m)
	mux.Handle("/", handler)

	address :=
		fmt.Sprintf(
			":%d",
			cfg.Server.Port,
		)

	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf(
			"GoLoad listening on %s using %s",
			address,
			cfg.LoadBalancer.Strategy,
		)

		if err :=
			server.ListenAndServe(); err != nil &&
			!errors.Is(
				err,
				http.ErrServerClosed,
			) {

			log.Fatal(err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel :=
		context.WithTimeout(
			context.Background(),
			10*time.Second,
		)

	defer cancel()

	if err :=
		server.Shutdown(
			shutdownCtx,
		); err != nil {

		log.Printf(
			"graceful shutdown failed: %v",
			err,
		)

		_ = server.Close()
	}
}

func createBackends(
	configs []config.BackendConfig,
) ([]*backend.Backend, error) {
	backends := make(
		[]*backend.Backend,
		0,
		len(configs),
	)

	for _, cfg := range configs {
		b, err := backend.New(cfg.URL)

		if err != nil {
			return nil, fmt.Errorf(
				"invalid backend %q: %w",
				cfg.URL,
				err,
			)
		}

		backends = append(
			backends,
			b,
		)
	}

	return backends, nil
}
