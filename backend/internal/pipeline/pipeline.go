package pipeline

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kubeboiii/vellum/internal/model"
)

type Processor func(ctx context.Context, sig model.Signal) error

type Config struct {
	Capacity        int
	Workers         int
	ShutdownTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{
		Capacity:        50_000,
		Workers:         16,
		ShutdownTimeout: 30 * time.Second,
	}
}

type Stats struct {
	Accepted   uint64
	Processed  uint64
	Dropped    uint64
	Errors     uint64
	QueueDepth int
	Capacity   int
}

type Pipeline struct {
	cfg       Config
	queue     chan model.Signal
	processor Processor

	accepted  atomic.Uint64
	processed atomic.Uint64
	dropped   atomic.Uint64
	errors    atomic.Uint64

	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	done      chan struct{}
}

var ErrNotStarted = errors.New("pipeline not started")

func New(cfg Config, p Processor) *Pipeline {
	if cfg.Capacity <= 0 {
		cfg.Capacity = 50_000
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 16
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
	return &Pipeline{
		cfg:       cfg,
		queue:     make(chan model.Signal, cfg.Capacity),
		processor: p,
		done:      make(chan struct{}),
	}
}

func (p *Pipeline) Start(ctx context.Context) {
	p.startOnce.Do(func() {
		for i := 0; i < p.cfg.Workers; i++ {
			p.wg.Add(1)
			go p.workerLoop(ctx, i)
		}
		log.Printf("pipeline: started %d workers (capacity=%d)", p.cfg.Workers, p.cfg.Capacity)
	})
}

func (p *Pipeline) Submit(sig model.Signal) bool {
	select {
	case p.queue <- sig:
		p.accepted.Add(1)
		return true
	default:
		p.dropped.Add(1)
		return false
	}
}

func (p *Pipeline) Stats() Stats {
	return Stats{
		Accepted:   p.accepted.Load(),
		Processed:  p.processed.Load(),
		Dropped:    p.dropped.Load(),
		Errors:     p.errors.Load(),
		QueueDepth: len(p.queue),
		Capacity:   p.cfg.Capacity,
	}
}

func (p *Pipeline) Capacity() int { return p.cfg.Capacity }

func (p *Pipeline) Stop() {
	p.stopOnce.Do(func() {
		close(p.queue)

		drained := make(chan struct{})
		go func() {
			p.wg.Wait()
			close(drained)
		}()
		select {
		case <-drained:
			log.Print("pipeline: drained cleanly")
		case <-time.After(p.cfg.ShutdownTimeout):
			log.Printf("pipeline: drain deadline %s exceeded, exiting", p.cfg.ShutdownTimeout)
		}
		close(p.done)
	})
}

func (p *Pipeline) Done() <-chan struct{} { return p.done }

func (p *Pipeline) workerLoop(ctx context.Context, id int) {
	defer p.wg.Done()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("pipeline: worker %d panicked: %v", id, r)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case sig, ok := <-p.queue:
			if !ok {
				return
			}
			if err := p.processor(ctx, sig); err != nil {
				p.errors.Add(1)
				log.Printf("pipeline: worker %d processor error: %v (signal_id=%s)", id, err, sig.SignalID)
			}
			p.processed.Add(1)
		}
	}
}

func NoopProcessor(_ context.Context, _ model.Signal) error { return nil }
