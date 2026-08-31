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
	"github.com/LiamMakela/GoLoad/internal/middleware"
	"github.com/LiamMakela/GoLoad/internal/proxy"
	"github.com/LiamMakela/GoLoad/internal/ratelimit"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	var backends []*backend.Backend

	for _, backendConfig := range cfg.Backends {
		b, err := backend.New(backendConfig.URL)
		if err != nil {
			log.Fatalf(
				"invalid backend %s: %v",
				backendConfig.URL,
				err,
			)
		}

		backends = append(backends, b)
	}

	lb, err := balancer.New(
		backends,
		cfg.LoadBalancer.Strategy,
	)

	if err != nil {
		log.Fatalf(
			"failed to create load balancer: %v",
			err,
		)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	checker := health.New(
		backends,
		cfg.Health.Interval,
		cfg.Health.Timeout,
	)

	checker.Start(ctx)

	m := metrics.New(backends)

	proxyHandler := proxy.New(
		lb,
		m,
		cfg.Proxy.Timeout,
		cfg.Proxy.Retries,
		ctx,
	)

	mux := http.NewServeMux()

	mux.Handle("/metrics", m)

	var publicHandler http.Handler = proxyHandler

	if cfg.RateLimit.Enabled {
		limiter := ratelimit.NewIPLimiter(
			cfg.RateLimit.RequestsPerSecond,
			cfg.RateLimit.Burst,
		)

		publicHandler = middleware.RateLimit(
			publicHandler,
			limiter,
		)
	}

	mux.Handle("/", publicHandler)

	address := fmt.Sprintf(
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

		err := server.ListenAndServe()

		if err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatalf(
				"server error: %v",
				err,
			)
		}
	}()

	<-ctx.Done()

	log.Println("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	defer cancel()

	log.Println("waiting for active connections to close...")

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf(
			"graceful shutdown timed out: %v",
			err,
		)

		if err := server.Close(); err != nil {
			log.Printf(
				"forced server close failed: %v",
				err,
			)
		}
	}

	log.Println("GoLoad stopped")
}
