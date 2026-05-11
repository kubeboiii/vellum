// Package redis wraps the go-redis v9 client and is responsible for
// loading the debounce Lua script at startup. The script lives in
// `backend/internal/debounce/script.lua` (per CLAUDE.md design rule 4)
// and is embedded via go:embed so the production binary needs no
// filesystem access to find it.
//
// We intentionally keep the surface of this package tiny: NewClient,
// LoadDebounceScript, Ping. The actual debounce *logic* (deciding
// whether the result means JOINED vs CREATED, fallback when Redis is
// down) lives in `internal/debounce`. This package only knows about
// the protocol.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ClientConfig is the per-instance knobs we need at startup.
type ClientConfig struct {
	Addr           string        // host:port, e.g. "localhost:6379"
	Password       string        // empty if no AUTH
	DB             int           // logical DB index, default 0
	ConnectTimeout time.Duration // bounds the initial PING
}

// NewClient opens a connection pool, pings to verify reachability, and
// returns the typed client. Caller closes via `client.Close()` on
// shutdown.
//
// We use a single pool (go-redis manages internal connection pooling)
// — no explicit MaxConns knob because go-redis defaults (10 * NumCPU)
// are tuned for high-throughput workloads exactly like ours.
func NewClient(ctx context.Context, cfg ClientConfig) (*redis.Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("redis: Addr is required")
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 3 * time.Second
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return client, nil
}

// LoadScript sends `SCRIPT LOAD <body>` to Redis and returns the SHA-1
// hash of the script. Subsequent calls can use EVALSHA <sha> instead of
// EVAL <body>, which avoids re-transmitting the whole script body on
// every call (~1KB) — saves bandwidth and one parse cycle.
//
// The result is deterministic: the same script always hashes to the
// same SHA. We could compute it client-side (sha1 of the body) instead
// of round-tripping, but doing it server-side ALSO confirms the script
// loaded without syntax errors — startup-time safety net.
//
// If the server forgets the script (FLUSHDB, restart), EVALSHA returns
// NOSCRIPT and the caller must reload. We don't auto-reload here
// because the debounce caller handles that error (and falls back to
// always-CREATED if Redis is truly down — FR-3.6).
func LoadScript(ctx context.Context, client *redis.Client, body string) (string, error) {
	sha, err := client.ScriptLoad(ctx, body).Result()
	if err != nil {
		return "", fmt.Errorf("redis: SCRIPT LOAD: %w", err)
	}
	return sha, nil
}

// PingChecker satisfies the obs.Pinger contract that /health will use.
type PingChecker struct{ Client *redis.Client }

func (p PingChecker) Ping(ctx context.Context) error { return p.Client.Ping(ctx).Err() }
func (PingChecker) Name() string                     { return "redis" }
