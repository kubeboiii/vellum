package processor

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"

	"github.com/kubeboiii/vellum/internal/alert"
	"github.com/kubeboiii/vellum/internal/debounce"
	"github.com/kubeboiii/vellum/internal/model"
)

type Debouncer interface {
	Process(ctx context.Context, componentID string) (debounce.Result, error)
}

type WorkItemRepo interface {
	Insert(ctx context.Context, wi model.WorkItem) error
	IncrementSignalCount(ctx context.Context, id uuid.UUID, signalTS time.Time) error
}

type SignalRepo interface {
	Insert(ctx context.Context, sig model.Signal, workItemID uuid.UUID) error
}

type MetricsWriter interface {
	Insert(ctx context.Context, sig model.Signal, workItemID uuid.UUID) error
}

type DeadLetter interface {
	Insert(ctx context.Context, sink string, payload any, err error) error
}

type AlerterPicker interface {
	ForWorkItem(wi model.WorkItem) alert.Alerter
}

type Config struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	PerSinkTimeout time.Duration

	AlertTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{
		MaxAttempts:    3,
		InitialBackoff: 100 * time.Millisecond,
		PerSinkTimeout: 2 * time.Second,
		AlertTimeout:   5 * time.Second,
	}
}

type Processor struct {
	cfg        Config
	debouncer  Debouncer
	workItems  WorkItemRepo
	signals    SignalRepo
	metrics    MetricsWriter
	deadLetter DeadLetter
	alerters   AlerterPicker

	degradedLogged bool
}

func New(cfg Config, d Debouncer, wi WorkItemRepo, sig SignalRepo, mw MetricsWriter, dl DeadLetter, alerters AlerterPicker) *Processor {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 100 * time.Millisecond
	}
	if cfg.PerSinkTimeout <= 0 {
		cfg.PerSinkTimeout = 2 * time.Second
	}
	if cfg.AlertTimeout <= 0 {
		cfg.AlertTimeout = 5 * time.Second
	}
	return &Processor{
		cfg:        cfg,
		debouncer:  d,
		workItems:  wi,
		signals:    sig,
		metrics:    mw,
		deadLetter: dl,
		alerters:   alerters,
	}
}

func (p *Processor) Process(ctx context.Context, sig model.Signal) error {

	res, err := p.debouncer.Process(ctx, sig.ComponentID)
	if err != nil {
		if errors.Is(err, debounce.ErrRedisDegraded) {
			if !p.degradedLogged {
				log.Printf("processor: redis debounce unavailable, falling through to always-CREATED (logged once)")
				p.degradedLogged = true
			}

		} else {

			return err
		}
	} else if p.degradedLogged && !res.Degraded {

		log.Print("processor: redis debounce recovered")
		p.degradedLogged = false
	}

	p.writePostgres(ctx, sig, res)
	p.writeMongo(ctx, sig, res.WorkItemID)
	p.writeTimescale(ctx, sig, res.WorkItemID)

	if p.alerters != nil && res.Action == debounce.ActionCreated {
		wi := model.NewWorkItem(res.WorkItemID, sig)
		p.dispatchAlert(wi)
	}
	return nil
}

func (p *Processor) dispatchAlert(wi model.WorkItem) {
	alerter := p.alerters.ForWorkItem(wi)
	if alerter == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), p.cfg.AlertTimeout)
		defer cancel()
		if err := alerter.Dispatch(ctx, wi); err != nil {
			log.Printf("processor: alerter %s failed for work_item_id=%s: %v",
				alerter.Name(), wi.ID, err)
		}
	}()
}

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

func (p *Processor) runWithRetry(parentCtx context.Context, sink string, op func() error, payload any) {

	ctx, cancel := context.WithTimeout(parentCtx, p.cfg.PerSinkTimeout)
	defer cancel()

	policy := backoff.NewExponentialBackOff()
	policy.InitialInterval = p.cfg.InitialBackoff
	policy.Multiplier = 2.0
	policy.MaxInterval = 1 * time.Second
	policy.MaxElapsedTime = p.cfg.PerSinkTimeout

	bo := backoff.WithMaxRetries(policy, uint64(p.cfg.MaxAttempts-1))

	err := backoff.Retry(op, backoff.WithContext(bo, ctx))
	if err == nil {
		return
	}

	log.Printf("processor: sink=%s exhausted retries: %v", sink, err)
	dlCtx, dlCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer dlCancel()
	if dlErr := p.deadLetter.Insert(dlCtx, sink, payload, err); dlErr != nil {
		log.Printf("processor: dead_letter insert ALSO failed: %v", dlErr)
	}
}
