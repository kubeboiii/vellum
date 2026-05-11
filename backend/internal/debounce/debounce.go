// Package debounce decides, for each incoming signal, whether it should
// join an existing Work Item's debounce window or kick off a fresh one.
// This is FR-3 in 00-master-prd and the most interesting subsystem in
// the project (01-architecture §5 calls it out as the "single most
// interesting subsystem" — internalize how it works).
//
// The check-then-act is implemented as a single Redis Lua script
// (script.lua in this package), guaranteed atomic by Redis's
// single-threaded scripting engine. If Redis is unreachable we fall
// through to "always CREATED" — noisier but no signal loss (FR-3.6).
package debounce

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// scriptBody embeds the Lua source so the production binary doesn't
// need filesystem access at runtime. CLAUDE.md design rule 4 says the
// script lives in this exact file.
//
//go:embed script.lua
var scriptBody string

// ScriptBody exposes the embedded Lua source for callers (the redis
// package) that need to SCRIPT LOAD it at startup.
func ScriptBody() string { return scriptBody }

// Action is what the Lua script decided: a new Work Item was opened, or
// the signal was attached to an existing one.
type Action string

const (
	ActionCreated Action = "CREATED"
	ActionJoined  Action = "JOINED"
)

// Result is what Process returns for one signal.
type Result struct {
	WorkItemID uuid.UUID
	Action     Action
	Count      int  // signal count in the window AFTER this signal
	Degraded   bool // true if Redis was unavailable and we used the fallback
}

// Config bundles the debounce window parameters from FR-3.1.
type Config struct {
	WindowSeconds int
	MaxSignals    int
}

// DefaultConfig returns the spec values: 10s window, 100 signals max.
func DefaultConfig() Config {
	return Config{WindowSeconds: 10, MaxSignals: 100}
}

// Debouncer holds the state needed to call the script: the client,
// the *redis.Script helper, and the window config.
//
// We use go-redis's *redis.Script.Run instead of raw EVALSHA because
// it handles the NOSCRIPT-on-evict case transparently:
//   - Fast path: EVALSHA on the cached SHA (one round-trip).
//   - If Redis evicted the script (FLUSHDB, restart, etc.), it
//     automatically re-uploads via EVAL and retries.
//
// That means a Redis restart doesn't permanently break debounce —
// the first call after the restart pays one extra round-trip.
type Debouncer struct {
	client redis.Scripter
	script *redis.Script
	cfg    Config
}

// New builds a Debouncer. scriptSHA is accepted for symmetry with the
// startup flow (`persist/redis.LoadScript` returns a SHA) but is not
// stored — *redis.Script computes the SHA itself from the body.
func New(client redis.Scripter, scriptSHA string, cfg Config) *Debouncer {
	_ = scriptSHA // intentionally unused — kept in the signature so
	//             callers can log the SHA without us holding it twice.
	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = 10
	}
	if cfg.MaxSignals <= 0 {
		cfg.MaxSignals = 100
	}
	return &Debouncer{
		client: client,
		script: redis.NewScript(scriptBody),
		cfg:    cfg,
	}
}

// ErrRedisDegraded is the sentinel returned when Redis is unreachable
// AND we fell through to the always-CREATED fallback. The caller can
// log it once and still get a usable Result.
var ErrRedisDegraded = errors.New("debounce: redis unavailable, used fallback")

// Process runs the Lua script for the given component_id, returning
// the work_item_id (existing or new), the action taken, and the count.
//
// On Redis error, returns a CREATED result with a fresh UUID and
// `Degraded: true`. The error itself is ErrRedisDegraded — callers
// should log once-per-degradation and keep processing.
func (d *Debouncer) Process(ctx context.Context, componentID string) (Result, error) {
	candidateID := uuid.New()

	keys := []string{
		"debounce:" + componentID + ":work_item",
		"debounce:" + componentID + ":count",
	}
	args := []any{
		candidateID.String(),
		strconv.Itoa(d.cfg.WindowSeconds),
		strconv.Itoa(d.cfg.MaxSignals),
	}

	// script.Run handles the EVALSHA → EVAL fallback transparently if
	// Redis evicted the script cache (FLUSHDB, restart, etc.). That
	// means a Redis restart doesn't permanently break debounce — the
	// first call after the restart pays one extra round-trip to
	// re-cache the script, then everything is back to fast-path.
	raw, err := d.script.Run(ctx, d.client, keys, args...).Result()
	if err != nil {
		// Fallback path. Per FR-3.6 we treat every signal as a fresh
		// window when Redis is down. The system stays up; we just get
		// more work items than we'd otherwise have.
		return Result{
			WorkItemID: candidateID,
			Action:     ActionCreated,
			Count:      1,
			Degraded:   true,
		}, ErrRedisDegraded
	}

	return parseScriptResult(raw, candidateID)
}

// parseScriptResult unpacks the {work_item_id, action, count} array
// the Lua script returns. Done in a helper so tests can target the
// parsing logic directly without spinning up Redis.
func parseScriptResult(raw any, candidate uuid.UUID) (Result, error) {
	arr, ok := raw.([]any)
	if !ok || len(arr) != 3 {
		return Result{}, fmt.Errorf("debounce: unexpected script result type %T", raw)
	}

	idStr, ok := arr[0].(string)
	if !ok {
		return Result{}, fmt.Errorf("debounce: result[0] not a string: %T", arr[0])
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return Result{}, fmt.Errorf("debounce: parse work_item_id: %w", err)
	}

	actionStr, ok := arr[1].(string)
	if !ok {
		return Result{}, fmt.Errorf("debounce: result[1] not a string: %T", arr[1])
	}

	// Redis returns Lua integers as int64.
	count, ok := arr[2].(int64)
	if !ok {
		return Result{}, fmt.Errorf("debounce: result[2] not an int64: %T", arr[2])
	}

	res := Result{
		WorkItemID: id,
		Action:     Action(actionStr),
		Count:      int(count),
	}
	// Sanity check: if action == CREATED, the script picked our
	// candidate UUID, so the returned ID should match it. If action
	// == JOINED, ID is the *existing* work_item_id (different from
	// our candidate).
	if res.Action == ActionCreated && res.WorkItemID != candidate {
		return Result{}, fmt.Errorf("debounce: CREATED but id mismatch (%v vs %v)", res.WorkItemID, candidate)
	}
	return res, nil
}

// WindowDuration is the configured window as a Duration. Useful when
// the caller wants to log "window expires in N seconds".
func (d *Debouncer) WindowDuration() time.Duration {
	return time.Duration(d.cfg.WindowSeconds) * time.Second
}
