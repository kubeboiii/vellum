// Package main is the entrypoint for the IMS backend.
//
// Phase 1 (Foundation): start an empty HTTP server on :8080 with a single
// liveness probe at /health that returns 200. Subsequent phases wire in the
// ingestion pipeline, workers, persistence, workflow, and alerting per the
// dependency rule in 01-architecture §10.1.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultHTTPAddr = ":8080"
	shutdownTimeout = 10 * time.Second
)

func main() {
	addr := os.Getenv("IMS_HTTP_ADDR")
	if addr == "" {
		addr = defaultHTTPAddr
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Phase 1 placeholder. Phase 2 replaces this with the real dependency
	// roll-up described in 00-master-prd FR-8.1 / 01-architecture §11.1.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "phase": 1})
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Run the HTTP server and the signal handler concurrently. On SIGINT or
	// SIGTERM, give in-flight requests `shutdownTimeout` to drain.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("ims-backend listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("http server failed: %v", err)
		}
	case <-ctx.Done():
		log.Printf("shutdown signal received, draining for %s", shutdownTimeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("graceful shutdown failed: %v", err)
		}
	}
	log.Print("ims-backend stopped")
}
