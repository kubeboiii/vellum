// Package main is the entrypoint for the IMS backend.
//
// Phase 3 wires the full pipeline:
//
//   - Postgres pool + WorkItemRepository (internal/persist/pg)
//   - Mongo client + SignalRepository + DeadLetterRepository (internal/persist/mongo)
//   - Redis client + Lua SCRIPT LOAD for debounce (internal/persist/redis + internal/debounce)
//   - TimescaleDB writer reusing the pg pool (internal/persist/timescale)
//   - Processor: debounce → fan-out + retry/backoff + dead-letter (internal/processor)
//   - Bounded-channel pipeline + worker pool from Phase 2 (internal/pipeline)
//   - /v1/signals + rate limiter from Phase 2 (internal/ingest)
//   - /health with per-dep Pinger roll-up (internal/obs)
//   - 5s metrics ticker (internal/obs)
//
// Shutdown ordering: HTTP listener first → pipeline drain → close all
// pools/clients. Same pattern as Phase 2, just more cleanups.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"github.com/kubeboiii/ims/internal/alert"
	"github.com/kubeboiii/ims/internal/api"
	"github.com/kubeboiii/ims/internal/debounce"
	"github.com/kubeboiii/ims/internal/ingest"
	"github.com/kubeboiii/ims/internal/model"
	"github.com/kubeboiii/ims/internal/obs"
	"github.com/kubeboiii/ims/internal/persist/mongo"
	"github.com/kubeboiii/ims/internal/persist/pg"
	"github.com/kubeboiii/ims/internal/persist/redis"
	"github.com/kubeboiii/ims/internal/persist/timescale"
	"github.com/kubeboiii/ims/internal/pipeline"
	"github.com/kubeboiii/ims/internal/processor"
	"github.com/kubeboiii/ims/internal/workflow"
	imsv1 "github.com/kubeboiii/ims/proto/ims/v1"
)

const (
	defaultHTTPAddr        = ":8080"
	defaultQueueCapacity   = 50_000
	defaultRateLimitRPS    = 1000.0
	defaultRateLimitBurst  = 2000
	defaultMetricsInterval = 5 * time.Second
	defaultShutdownTimeout = 30 * time.Second
	defaultIdleBucketTTL   = 5 * time.Minute

	defaultDatabaseURL = "postgres://ims:ims@localhost:5432/ims?sslmode=disable"
	defaultMongoURI    = "mongodb://ims:ims@localhost:27017/ims?authSource=admin"
	defaultMongoDB     = "ims"
	defaultRedisAddr   = "localhost:6379"

	defaultDebounceWindowSeconds = 10
	defaultDebounceMaxSignals    = 100
	defaultDepPingTimeout        = 500 * time.Millisecond
	defaultAlertTimeout          = 5 * time.Second

	defaultGRPCAddr    = ":9090"
	defaultCORSOrigins = "http://localhost:3000"
)

func main() {
	cfg := loadConfig()

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ---- 1. Build the persistence layer.
	//
	// We fail-fast on startup: if any sink we *need* to talk to at
	// startup is unreachable, we exit rather than starting in a broken
	// state. (Phase 2's /health degradation is for runtime failures —
	// the system was reachable when we booted and lost a dep later.)

	pgPool, err := pg.NewPool(rootCtx, pg.PoolConfig{DSN: cfg.databaseURL})
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pgPool.Close()
	workItems := pg.NewWorkItemRepository(pgPool)

	mongoClient, err := mongo.NewClient(rootCtx, mongo.ClientConfig{
		URI: cfg.mongoURI, Database: cfg.mongoDB,
	})
	if err != nil {
		log.Fatalf("mongo: %v", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoClient.Disconnect(closeCtx)
	}()
	mongoDB := mongoClient.Database(cfg.mongoDB)
	signals := mongo.NewSignalRepository(mongoDB)
	deadLetter := mongo.NewDeadLetterRepository(mongoDB)
	// Create indexes once at startup. Idempotent — safe to re-run.
	if err := signals.EnsureIndexes(rootCtx); err != nil {
		log.Fatalf("mongo: ensure indexes: %v", err)
	}

	redisClient, err := redis.NewClient(rootCtx, redis.ClientConfig{Addr: cfg.redisAddr})
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer redisClient.Close()
	scriptSHA, err := redis.LoadScript(rootCtx, redisClient, debounce.ScriptBody())
	if err != nil {
		log.Fatalf("redis: load lua: %v", err)
	}
	log.Printf("debounce: lua script loaded (sha=%s)", scriptSHA)

	metrics := timescale.NewMetricsWriter(pgPool)

	// ---- 2. Build the orchestration layer.

	debouncer := debounce.New(redisClient, scriptSHA, debounce.Config{
		WindowSeconds: cfg.debounceWindow,
		MaxSignals:    cfg.debounceMaxSignals,
	})

	// Alerter registry per FR-6.2. Slack uses SLACK_WEBHOOK_URL if
	// set; falls back to console when empty. PagerDuty stub matches P0;
	// Slack matches P1/P2; everything else (P3 + fallback) lands on
	// the console alerter.
	var slackAlerter alert.Alerter = alert.NewSlackAlerter(cfg.slackWebhookURL, cfg.alertTimeout)
	if cfg.slackWebhookURL == "" {
		slackAlerter = alert.ConsoleAlerter{}
		log.Print("alert: SLACK_WEBHOOK_URL unset, P1/P2 alerts will log to console")
	}
	alerterRegistry := alert.NewRegistry(
		map[string]alert.Alerter{
			"pagerduty_stub": alert.PagerDutyStub{},
			"slack_webhook":  slackAlerter,
			"console":        alert.ConsoleAlerter{},
		},
		[]alert.Rule{
			alert.SeverityRule("p0", "pagerduty_stub", model.SeverityP0),
			alert.SeverityRule("p12", "slack_webhook", model.SeverityP1, model.SeverityP2),
		},
	)

	procCfg := processor.DefaultConfig()
	procCfg.AlertTimeout = cfg.alertTimeout
	proc := processor.New(procCfg, debouncer, workItems, signals, metrics, deadLetter, alerterRegistry)

	// Workflow engine (Phase 4): handles state transitions in a
	// SERIALIZABLE Postgres tx + the compound RCA-and-close flow.
	rcaRepo := pg.NewRCARepository(pgPool)
	transitionReader := pg.NewTransitionReader(pgPool)
	workflowEngine := workflow.NewEngine(pg.NewWorkflowTxRunner(workItems))

	// ---- 3. Pipeline (Phase 2). Plug in the real Processor.

	pipe := pipeline.New(pipeline.Config{
		Capacity:        cfg.queueCapacity,
		Workers:         cfg.workerCount,
		ShutdownTimeout: cfg.shutdownTimeout,
	}, proc.Process)
	pipe.Start(rootCtx)

	// ---- 4. Rate limiter (Phase 2 — unchanged).

	limiter := ingest.NewRateLimiter(cfg.rateLimitRPS, cfg.rateLimitBurst, defaultIdleBucketTTL)
	go limiter.RunSweeper(rootCtx)

	// ---- 5. HTTP server.

	r := gin.New()
	r.Use(gin.Recovery())
	// CORS middleware so the Next.js dashboard at localhost:3000 can
	// call this API in dev. Production would gate via a reverse proxy
	// or proper CORS lib; this small inline implementation covers our
	// needs without an extra dep.
	r.Use(corsMiddleware(cfg.corsOrigins))

	v1 := r.Group("/v1")
	if err := ingest.RegisterRoutes(v1, pipe, limiter.Middleware()); err != nil {
		log.Fatalf("ingest routes: %v", err)
	}
	// Phase 4+5 endpoints: incident list/detail + state machine + RCA
	// + paginated raw signals + state-transition audit timeline
	// + closed-incident history.
	api.RegisterRoutes(v1, &api.Handlers{
		WorkItems:   workItems,
		RCA:         rcaRepo,
		Signals:     signals,
		Transitions: transitionReader,
		Engine:      workflowEngine,
	})

	health := obs.NewHealth(pipe, obs.HealthConfig{
		Deps: []obs.Pinger{
			workItems, // satisfies Pinger via Name()/Ping()
			signals,   // ditto
			redis.PingChecker{Client: redisClient},
		},
		PingTimeout: cfg.depPingTimeout,
	})
	r.GET("/health", health.Handler())

	// ---- 6. Metrics ticker.

	go obs.NewMetricsTicker(pipe, cfg.metricsInterval).Run(rootCtx)

	// ---- 7. HTTP server with explicit timeouts.

	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("ims-backend HTTP listening on %s (workers=%d, queue=%d, debounce=%ds/%dmax)",
			cfg.httpAddr, cfg.workerCount, cfg.queueCapacity, cfg.debounceWindow, cfg.debounceMaxSignals)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	// ---- 7b. gRPC server (Phase 5). Listens on a separate port,
	// shares the same pipeline as HTTP per FR-1.3 ("exactly one
	// downstream path regardless of protocol").
	grpcLis, err := net.Listen("tcp", cfg.grpcAddr)
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}
	grpcSrv := grpc.NewServer()
	imsv1.RegisterSignalServiceServer(grpcSrv, ingest.NewSignalServiceServer(pipe))
	grpcErr := make(chan error, 1)
	go func() {
		log.Printf("ims-backend gRPC listening on %s", cfg.grpcAddr)
		if err := grpcSrv.Serve(grpcLis); err != nil {
			grpcErr <- err
		}
		close(grpcErr)
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Printf("http server failed: %v", err)
			grpcSrv.Stop()
			pipe.Stop()
			os.Exit(1)
		}
	case err := <-grpcErr:
		if err != nil {
			log.Printf("grpc server failed: %v", err)
			_ = srv.Close()
			pipe.Stop()
			os.Exit(1)
		}
	case <-rootCtx.Done():
		log.Printf("shutdown signal received")
	}

	// ---- 8. Ordered shutdown: stop accepting new requests (HTTP +
	// gRPC) first, then drain the pipeline. Pipeline.Stop closes the
	// channel, which would panic any in-flight handler trying to
	// Submit — so the network shutdowns MUST come first.
	log.Printf("shutting down: servers first, then pipeline drain (timeout=%s)", cfg.shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer cancel()

	// GracefulStop waits for in-flight RPCs but won't accept new ones.
	// We don't bound it by ctx because gRPC's Stop (the rude one)
	// fires automatically if the process exits — and our outer
	// shutdownTimeout caps total wall time via pipe drain anyway.
	go grpcSrv.GracefulStop()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}
	grpcSrv.Stop() // belt-and-braces: ensure gRPC is fully done.
	pipe.Stop()
	<-pipe.Done()
	log.Print("ims-backend stopped")
}

// ---- Config plumbing -------------------------------------------------

type config struct {
	httpAddr        string
	queueCapacity   int
	workerCount     int
	rateLimitRPS    float64
	rateLimitBurst  int
	metricsInterval time.Duration
	shutdownTimeout time.Duration

	databaseURL string
	mongoURI    string
	mongoDB     string
	redisAddr   string

	debounceWindow     int
	debounceMaxSignals int
	depPingTimeout     time.Duration

	slackWebhookURL string
	alertTimeout    time.Duration

	grpcAddr    string
	corsOrigins []string
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

		databaseURL: envOr("DATABASE_URL", defaultDatabaseURL),
		mongoURI:    envOr("MONGO_URI", defaultMongoURI),
		mongoDB:     envOr("MONGO_DATABASE", defaultMongoDB),
		redisAddr:   envOr("REDIS_ADDR", defaultRedisAddr),

		debounceWindow:     envInt("IMS_DEBOUNCE_WINDOW_SECONDS", defaultDebounceWindowSeconds),
		debounceMaxSignals: envInt("IMS_DEBOUNCE_MAX_SIGNALS", defaultDebounceMaxSignals),
		depPingTimeout:     envDur("IMS_DEP_PING_TIMEOUT", defaultDepPingTimeout),

		slackWebhookURL: envOr("SLACK_WEBHOOK_URL", ""),
		alertTimeout:    envDur("IMS_ALERTER_TIMEOUT", defaultAlertTimeout),

		grpcAddr:    envOr("IMS_GRPC_ADDR", defaultGRPCAddr),
		corsOrigins: parseCSV(envOr("IMS_CORS_ORIGINS", defaultCORSOrigins)),
	}
}

// parseCSV splits a comma-separated env value, trimming whitespace
// and skipping empty entries. Used for IMS_CORS_ORIGINS.
func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// corsMiddleware enforces a tiny CORS policy: each request's Origin
// must be in `allowed`, or the response carries no CORS headers (and
// browsers will block it). Handles preflight OPTIONS by short-circuiting
// with 204.
//
// Why inline rather than gin-contrib/cors:
//   - Our needs are tiny — one list of origins, the standard methods,
//     no credentials, no custom headers beyond Content-Type.
//   - Avoids a new dep just for ~30 lines of logic that we can review
//     in one sitting.
func corsMiddleware(allowed []string) gin.HandlerFunc {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		if _, ok := set[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
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
