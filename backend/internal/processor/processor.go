// Package processor is the per-signal orchestrator that replaces
// pipeline.NoopProcessor in Phase 3. For each signal pulled off the
// pipeline queue, it:
//
//  1. Asks the debouncer (Redis Lua) for the work_item_id + action.
//  2. Fans out to four sinks independently — NOT a distributed
//     transaction (01-architecture §6.2):
//     - Postgres (work_items insert or signal_count increment)
//     - MongoDB (raw signal audit log)
//     - TimescaleDB (signal_metrics row)
//  3. Wraps each write in retry-with-backoff (cenkalti/backoff/v4).
//  4. Dead-letters the failed payload to Mongo on exhaustion.
//
// The package depends ONLY on `internal/model` and the per-sink repo
// interfaces defined here — concrete repos are injected via
// constructor. This keeps the processor testable with fakes and
// enforces the inward dependency rule (01-architecture §10.1).
package processor

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"

	"github.com/kubeboiii/ims/internal/debounce"
	"github.com/kubeboiii/ims/internal/model"
)

// ---- Interfaces consumed (defined where used, per CLAUDE.md) ----

// Debouncer is the narrow surface we need from internal/debounce. We
// don't import the concrete *debounce.Debouncer because that would
// pull go-redis into our tests — defining the interface here lets us
// stub.
type Debouncer interface {
	Process(ctx context.Context, componentID string) (debounce.Result, error)
}

// WorkItemRepo is the narrow surface we need from pg.WorkItemRepository.
type WorkItemRepo interface {
	Insert(ctx context.Context, wi model.WorkItem) error
	IncrementSignalCount(ctx context.Context, id uuid.UUID, signalTS time.Time) error
}

// SignalRepo is the narrow surface we need from mongo.SignalRepository.
type SignalRepo interface {
	Insert(ctx context.Context, sig model.Signal, workItemID uuid.UUID) error
}

// MetricsWriter is the narrow surface we need from
// timescale.MetricsWriter.
type MetricsWriter interface {
	Insert(ctx context.Context, sig model.Signal, workItemID uuid.UUID) error
}

// DeadLetter is the narrow surface we need from
// mongo.DeadLetterRepository. We use a different name from the package
// (DeadLetter vs DeadLetterRepository) so the import-site code reads
// more naturally.
type DeadLetter interface {
	Insert(ctx context.Context, sink string, payload any, err error) error
}

// ---- Config + constructor ----

// Config bundles the retry knobs from 01-architecture §6.3 and the
// per-sink timeout that bounds total wall time of a doomed signal.
type Config struct {
	MaxAttempts    int           // total attempts including the first try
	InitialBackoff time.Duration // base delay; doubles each retry
	PerSinkTimeout time.Duration // total budget per sink (initial + retries)
}

// DefaultConfig returns the PRD-defined values: 3 attempts (100ms,
// 200ms, 400ms exponential), 2-second total budget per sink.
func DefaultConfig() Config {
	return Config{
		MaxAttempts:    3,
		InitialBackoff: 100 * time.Millisecond,
		PerSinkTimeout: 2 * time.Second,
	}
}

// Processor is the wired orchestrator. Built once at startup; the
// `Process` method plugs into `pipeline.Pipeline`'s Processor func type.
type Processor struct {
	cfg        Config
	debouncer  Debouncer
	workItems  WorkItemRepo
	signals    SignalRepo
	metrics    MetricsWriter
	deadLetter DeadLetter

	// degradedLogged guards us from spamming "redis degraded" logs on
	// every signal — we want one log per degradation episode, not
	// thousands. (Phase 6 could replace this with a proper rate-limited
	// logger; for v1 a once-token is fine.)
	degradedLogged bool
}

// New constructs the processor. All collaborators are required.
func New(cfg Config, d Debouncer, wi WorkItemRepo, sig SignalRepo, mw MetricsWriter, dl DeadLetter) *Processor {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 100 * time.Millisecond
	}
	if cfg.PerSinkTimeout <= 0 {
		cfg.PerSinkTimeout = 2 * time.Second
	}
	return &Processor{
		cfg:        cfg,
		debouncer:  d,
		workItems:  wi,
		signals:    sig,
		metrics:    mw,
		deadLetter: dl,
	}
}

// ---- The hot path ----

// Process is what the pipeline calls for each signal. Signature
// matches `pipeline.Processor` (a Go type alias). Returns an error
// only when the *control flow* is broken (e.g., bad input we can't
// debounce); individual sink failures are dead-lettered, NOT returned,
// so one bad sink doesn't block the worker.
func (p *Processor) Process(ctx context.Context, sig model.Signal) error {
	// 1. Debounce. On Redis failure we get a CREATED fallback +
	//    ErrRedisDegraded — note it but keep going (FR-3.6).
	res, err := p.debouncer.Process(ctx, sig.ComponentID)
	if err != nil {
		if errors.Is(err, debounce.ErrRedisDegraded) {
			if !p.degradedLogged {
				log.Printf("processor: redis debounce unavailable, falling through to always-CREATED (logged once)")
				p.degradedLogged = true
			}
			// res is still usable (CREATED with a fresh UUID).
		} else {
			// Unknown debounce error — can't proceed without a work_item_id.
			return err
		}
	} else if p.degradedLogged && !res.Degraded {
		// Recovery: Redis came back. Reset the gate so next outage logs again.
		log.Print("processor: redis debounce recovered")
		p.degradedLogged = false
	}

	// 2. Fan-out. Each sink is independent; we don't bail on the
	//    first failure. Sequential keeps the code simple — running
	//    them in parallel would add goroutine overhead per signal,
	//    which at 10K/sec adds up (~10K extra goroutines/sec).
	p.writePostgres(ctx, sig, res)
	p.writeMongo(ctx, sig, res.WorkItemID)
	p.writeTimescale(ctx, sig, res.WorkItemID)
	return nil
}

// writePostgres writes either a new work_items row (CREATED) or bumps
// the signal_count on an existing row (JOINED). Wrapped in retry; on
// final failure, dead-letters and returns silently.
func (p *Processor) writePostgres(ctx context.Context, sig model.Signal, res debounce.Result) {
	var (
		op      func() error
		payload any
	)
	if res.Action == debounce.ActionCreated {
		wi := model.NewWorkItem(res.WorkItemID, sig)
		payload = wi
		op = func() error { return p.workItems.Insert(ctx, wi) }
	} else {
		payload = map[string]any{
			"work_item_id":   res.WorkItemID,
			"last_signal_ts": sig.Timestamp,
		}
		op = func() error { return p.workItems.IncrementSignalCount(ctx, res.WorkItemID, sig.Timestamp) }
	}
	p.runWithRetry(ctx, "postgres", op, payload)
}

func (p *Processor) writeMongo(ctx context.Context, sig model.Signal, wiID uuid.UUID) {
	op := func() error { return p.signals.Insert(ctx, sig, wiID) }
	p.runWithRetry(ctx, "mongo", op, map[string]any{
		"signal_id":    sig.SignalID,
		"work_item_id": wiID,
	})
}

func (p *Processor) writeTimescale(ctx context.Context, sig model.Signal, wiID uuid.UUID) {
	op := func() error { return p.metrics.Insert(ctx, sig, wiID) }
	p.runWithRetry(ctx, "timescale", op, map[string]any{
		"signal_id":    sig.SignalID,
		"work_item_id": wiID,
		"ts":           sig.Timestamp,
	})
}

// runWithRetry is the single retry policy applied to every sink write.
// Lives in one place so a future operator can tune base/maxAttempts
// without touching three call sites.
//
// On final failure: log + push the original payload to the
// dead_letter collection. We do NOT return the error — the worker's
// next signal must continue processing.
func (p *Processor) runWithRetry(parentCtx context.Context, sink string, op func() error, payload any) {
	// Per-sink budget — bounds the worst-case wall time of a single
	// signal even if all sinks are slow. At 100ms base × 2 × 3 = 700ms
	// of pure backoff, plus call latency, 2s is comfortable.
	ctx, cancel := context.WithTimeout(parentCtx, p.cfg.PerSinkTimeout)
	defer cancel()

	policy := backoff.NewExponentialBackOff()
	policy.InitialInterval = p.cfg.InitialBackoff
	policy.Multiplier = 2.0
	policy.MaxInterval = 1 * time.Second
	policy.MaxElapsedTime = p.cfg.PerSinkTimeout
	// Capping attempts: backoff/v4's WithMaxRetries wraps the policy
	// so the .Retry loop stops after N attempts. N-1 retries means N
	// total tries (initial + N-1).
	bo := backoff.WithMaxRetries(policy, uint64(p.cfg.MaxAttempts-1))

	err := backoff.Retry(op, backoff.WithContext(bo, ctx))
	if err == nil {
		return
	}
	// Final failure. Dead-letter — note we use a fresh context (the
	// parent might be timing out from the previous shutdown). If
	// dead-letter also fails, log to stderr and move on.
	log.Printf("processor: sink=%s exhausted retries: %v", sink, err)
	dlCtx, dlCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer dlCancel()
	if dlErr := p.deadLetter.Insert(dlCtx, sink, payload, err); dlErr != nil {
		log.Printf("processor: dead_letter insert ALSO failed: %v", dlErr)
	}
}
