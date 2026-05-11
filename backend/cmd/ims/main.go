// Package main is the entrypoint for the IMS backend.
//
// Phase 2 wires:
//
//   - bounded-channel pipeline + worker pool (internal/pipeline)
//   - POST /v1/signals handler with non-blocking enqueue (internal/ingest)
//   - per-source token-bucket rate limiter (FR-1.6, internal/ingest)
//   - /health with queue stats (internal/obs)
//   - 5s metrics ticker on stdout (FR-8.2, internal/obs)
//   - graceful shutdown: HTTP listener first, then drain the queue, then exit
//
// Phase 3 will swap the no-op processor for the debounce + persistence
// fan-out, and Phase 5 will add the gRPC listener sharing the same
// pipeline.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubeboiii/ims/internal/ingest"
	"github.com/kubeboiii/ims/internal/obs"
	"github.com/kubeboiii/ims/internal/pipeline"
)

// Configuration knobs (see docs/phases/phase-2-ingestion.md §2.3).
const (
	defaultHTTPAddr        = ":8080"
	defaultQueueCapacity   = 50_000
	defaultRateLimitRPS    = 1000.0
	defaultRateLimitBurst  = 2000
	defaultMetricsInterval = 5 * time.Second
	defaultShutdownTimeout = 30 * time.Second
	defaultIdleBucketTTL   = 5 * time.Minute
)

func main() {
	cfg := loadConfig()

	// 1. Pipeline first — it owns the channel that the handler will write
	//    to. Workers do nothing yet beyond counting (Phase 2 = no
	//    persistence). The rootCtx is the lifecycle for everything below.
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pipe := pipeline.New(pipeline.Config{
		Capacity:        cfg.queueCapacity,
		Workers:         cfg.workerCount,
		ShutdownTimeout: cfg.shutdownTimeout,
	}, pipeline.NoopProcessor)
	pipe.Start(rootCtx)

	// 2. Rate limiter — its sweeper runs for the lifetime of the process.
	limiter := ingest.NewRateLimiter(cfg.rateLimitRPS, cfg.rateLimitBurst, defaultIdleBucketTTL)
	go limiter.RunSweeper(rootCtx)

	// 3. HTTP router.
	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/v1")
	if err := ingest.RegisterRoutes(v1, pipe, limiter.Middleware()); err != nil {
		log.Fatalf("ingest routes: %v", err)
	}

	health := obs.NewHealth(pipe)
	r.GET("/health", health.Handler())

	// 4. Metrics ticker.
	go obs.NewMetricsTicker(pipe, cfg.metricsInterval).Run(rootCtx)

	// 5. HTTP server with explicit timeouts. ReadHeaderTimeout in
	//    particular defends against Slowloris on the ingestion endpoint.
	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("ims-backend listening on %s (workers=%d, queue=%d)",
			cfg.httpAddr, cfg.workerCount, cfg.queueCapacity)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	// 6. Wait for either a server error or a shutdown signal, then drain.
	select {
	case err := <-serverErr:
		if err != nil {
			log.Printf("http server failed: %v", err)
			pipe.Stop()
			os.Exit(1)
		}
	case <-rootCtx.Done():
		log.Printf("shutdown signal received")
	}

	// Shutdown ordering matters:
	// (a) Stop accepting new HTTP requests (close listeners, finish
	//     in-flight handlers).
	// (b) THEN stop the pipeline (the handler can no longer enqueue,
	//     so closing the channel is race-free).
	// (c) Workers drain whatever they had buffered.
	log.Printf("shutting down: HTTP first, then pipeline drain (timeout=%s)", cfg.shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}
	pipe.Stop()
	<-pipe.Done()
	log.Print("ims-backend stopped")
}

// config bundles the env-driven knobs in one place so loadConfig is
// straightforward and testable (though we don't test main).
type config struct {
	httpAddr        string
	queueCapacity   int
	workerCount     int
	rateLimitRPS    float64
	rateLimitBurst  int
	metricsInterval time.Duration
	shutdownTimeout time.Duration
}

func loadConfig() config {
	return config{
		httpAddr:        envOr("IMS_HTTP_ADDR", defaultHTTPAddr),
		queueCapacity:   envInt("IMS_QUEUE_CAPACITY", defaultQueueCapacity),
		workerCount:     envInt("IMS_WORKER_COUNT", runtime.NumCPU()*2),
		rateLimitRPS:    envFloat("IMS_RATE_LIMIT_RPS", defaultRateLimitRPS),
		rateLimitBurst:  envInt("IMS_RATE_LIMIT_BURST", defaultRateLimitBurst),
		metricsInterval: envDur("IMS_METRICS_INTERVAL", defaultMetricsInterval),
		shutdownTimeout: envDur("IMS_SHUTDOWN_TIMEOUT", defaultShutdownTimeout),
	}
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("bad %s=%q: %v (using default %d)", key, v, err, fallback)
		return fallback
	}
	return n
}

func envFloat(key string, fallback float64) float64 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		log.Printf("bad %s=%q: %v (using default %v)", key, v, err, fallback)
		return fallback
	}
	return f
}

func envDur(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("bad %s=%q: %v (using default %s)", key, v, err, fallback)
		return fallback
	}
	return d
}
