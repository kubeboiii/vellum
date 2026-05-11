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

	"github.com/kubeboiii/vellum/internal/alert"
	"github.com/kubeboiii/vellum/internal/api"
	"github.com/kubeboiii/vellum/internal/debounce"
	"github.com/kubeboiii/vellum/internal/ingest"
	"github.com/kubeboiii/vellum/internal/model"
	"github.com/kubeboiii/vellum/internal/obs"
	"github.com/kubeboiii/vellum/internal/persist/mongo"
	"github.com/kubeboiii/vellum/internal/persist/pg"
	"github.com/kubeboiii/vellum/internal/persist/redis"
	"github.com/kubeboiii/vellum/internal/persist/timescale"
	"github.com/kubeboiii/vellum/internal/pipeline"
	"github.com/kubeboiii/vellum/internal/processor"
	"github.com/kubeboiii/vellum/internal/workflow"
	vellumv1 "github.com/kubeboiii/vellum/proto/vellum/v1"
)

const (
	defaultHTTPAddr        = ":8080"
	defaultQueueCapacity   = 50_000
	defaultRateLimitRPS    = 1000.0
	defaultRateLimitBurst  = 2000
	defaultMetricsInterval = 5 * time.Second
	defaultShutdownTimeout = 30 * time.Second
	defaultIdleBucketTTL   = 5 * time.Minute

	defaultDatabaseURL = "postgres://vellum:vellum@localhost:5432/vellum?sslmode=disable"
	defaultMongoURI    = "mongodb://vellum:vellum@localhost:27017/vellum?authSource=admin"
	defaultMongoDB     = "vellum"
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

	debouncer := debounce.New(redisClient, scriptSHA, debounce.Config{
		WindowSeconds: cfg.debounceWindow,
		MaxSignals:    cfg.debounceMaxSignals,
	})

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

	rcaRepo := pg.NewRCARepository(pgPool)
	transitionReader := pg.NewTransitionReader(pgPool)
	workflowEngine := workflow.NewEngine(pg.NewWorkflowTxRunner(workItems))

	pipe := pipeline.New(pipeline.Config{
		Capacity:        cfg.queueCapacity,
		Workers:         cfg.workerCount,
		ShutdownTimeout: cfg.shutdownTimeout,
	}, proc.Process)
	pipe.Start(rootCtx)

	limiter := ingest.NewRateLimiter(cfg.rateLimitRPS, cfg.rateLimitBurst, defaultIdleBucketTTL)
	go limiter.RunSweeper(rootCtx)

	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(corsMiddleware(cfg.corsOrigins))

	v1 := r.Group("/v1")
	if err := ingest.RegisterRoutes(v1, pipe, limiter.Middleware()); err != nil {
		log.Fatalf("ingest routes: %v", err)
	}

	api.RegisterRoutes(v1, &api.Handlers{
		WorkItems:   workItems,
		RCA:         rcaRepo,
		Signals:     signals,
		Transitions: transitionReader,
		Engine:      workflowEngine,
	})

	health := obs.NewHealth(pipe, obs.HealthConfig{
		Deps: []obs.Pinger{
			workItems,
			signals,
			redis.PingChecker{Client: redisClient},
		},
		PingTimeout: cfg.depPingTimeout,
	})
	r.GET("/health", health.Handler())

	go obs.NewMetricsTicker(pipe, cfg.metricsInterval).Run(rootCtx)

	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("vellum-backend HTTP listening on %s (workers=%d, queue=%d, debounce=%ds/%dmax)",
			cfg.httpAddr, cfg.workerCount, cfg.queueCapacity, cfg.debounceWindow, cfg.debounceMaxSignals)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	grpcLis, err := net.Listen("tcp", cfg.grpcAddr)
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}
	grpcSrv := grpc.NewServer()
	vellumv1.RegisterSignalServiceServer(grpcSrv, ingest.NewSignalServiceServer(pipe))
	grpcErr := make(chan error, 1)
	go func() {
		log.Printf("vellum-backend gRPC listening on %s", cfg.grpcAddr)
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

	log.Printf("shutting down: servers first, then pipeline drain (timeout=%s)", cfg.shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer cancel()

	go grpcSrv.GracefulStop()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}
	grpcSrv.Stop()
	pipe.Stop()
	<-pipe.Done()
	log.Print("vellum-backend stopped")
}

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
		httpAddr:        envOr("VELLUM_HTTP_ADDR", defaultHTTPAddr),
		queueCapacity:   envInt("VELLUM_QUEUE_CAPACITY", defaultQueueCapacity),
		workerCount:     envInt("VELLUM_WORKER_COUNT", runtime.NumCPU()*2),
		rateLimitRPS:    envFloat("VELLUM_RATE_LIMIT_RPS", defaultRateLimitRPS),
		rateLimitBurst:  envInt("VELLUM_RATE_LIMIT_BURST", defaultRateLimitBurst),
		metricsInterval: envDur("VELLUM_METRICS_INTERVAL", defaultMetricsInterval),
		shutdownTimeout: envDur("VELLUM_SHUTDOWN_TIMEOUT", defaultShutdownTimeout),

		databaseURL: envOr("DATABASE_URL", defaultDatabaseURL),
		mongoURI:    envOr("MONGO_URI", defaultMongoURI),
		mongoDB:     envOr("MONGO_DATABASE", defaultMongoDB),
		redisAddr:   envOr("REDIS_ADDR", defaultRedisAddr),

		debounceWindow:     envInt("VELLUM_DEBOUNCE_WINDOW_SECONDS", defaultDebounceWindowSeconds),
		debounceMaxSignals: envInt("VELLUM_DEBOUNCE_MAX_SIGNALS", defaultDebounceMaxSignals),
		depPingTimeout:     envDur("VELLUM_DEP_PING_TIMEOUT", defaultDepPingTimeout),

		slackWebhookURL: envOr("SLACK_WEBHOOK_URL", ""),
		alertTimeout:    envDur("VELLUM_ALERTER_TIMEOUT", defaultAlertTimeout),

		grpcAddr:    envOr("VELLUM_GRPC_ADDR", defaultGRPCAddr),
		corsOrigins: parseCSV(envOr("VELLUM_CORS_ORIGINS", defaultCORSOrigins)),
	}
}

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
