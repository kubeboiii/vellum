package processor

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubeboiii/vellum/internal/debounce"
	"github.com/kubeboiii/vellum/internal/model"
)

type fakeDebouncer struct {
	result debounce.Result
	err    error
	calls  atomic.Int64
}

func (f *fakeDebouncer) Process(_ context.Context, _ string) (debounce.Result, error) {
	f.calls.Add(1)
	return f.result, f.err
}

type fakeWorkItems struct {
	insertErr     error
	insertErrOnce bool
	incrementErr  error
	inserts       atomic.Int64
	increments    atomic.Int64

	mu     sync.Mutex
	called int
}

func (f *fakeWorkItems) Insert(_ context.Context, _ model.WorkItem) error {
	f.inserts.Add(1)
	if f.insertErrOnce {
		f.mu.Lock()
		f.called++
		first := f.called == 1
		f.mu.Unlock()
		if first {
			return f.insertErr
		}
		return nil
	}
	return f.insertErr
}

func (f *fakeWorkItems) IncrementSignalCount(_ context.Context, _ uuid.UUID, _ time.Time) error {
	f.increments.Add(1)
	return f.incrementErr
}

type fakeSignals struct {
	insertErr error
	inserts   atomic.Int64
}

func (f *fakeSignals) Insert(_ context.Context, _ model.Signal, _ uuid.UUID) error {
	f.inserts.Add(1)
	return f.insertErr
}

type fakeMetrics struct {
	insertErr error
	inserts   atomic.Int64
}

func (f *fakeMetrics) Insert(_ context.Context, _ model.Signal, _ uuid.UUID) error {
	f.inserts.Add(1)
	return f.insertErr
}

type fakeDeadLetter struct {
	mu      sync.Mutex
	records []dlRecord
}

type dlRecord struct {
	sink string
	err  error
}

func (f *fakeDeadLetter) Insert(_ context.Context, sink string, _ any, err error) error {
	f.mu.Lock()
	f.records = append(f.records, dlRecord{sink: sink, err: err})
	f.mu.Unlock()
	return nil
}

func (f *fakeDeadLetter) count(sink string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.records {
		if r.sink == sink {
			n++
		}
	}
	return n
}

func fastConfig() Config {
	return Config{
		MaxAttempts:    3,
		InitialBackoff: 10 * time.Millisecond,
		PerSinkTimeout: 500 * time.Millisecond,
	}
}

func sampleSignal() model.Signal {
	return model.Signal{
		SignalID:      uuid.New(),
		ComponentID:   "CACHE_01",
		ComponentType: model.ComponentCache,
		Severity:      model.SeverityP1,
		Source:        "test",
		Timestamp:     time.Now().UTC(),
	}
}

func TestProcess_CreatedFlow(t *testing.T) {
	wiID := uuid.New()
	d := &fakeDebouncer{result: debounce.Result{
		WorkItemID: wiID, Action: debounce.ActionCreated, Count: 1,
	}}
	wi := &fakeWorkItems{}
	sig := &fakeSignals{}
	mw := &fakeMetrics{}
	dl := &fakeDeadLetter{}
	p := New(fastConfig(), d, wi, sig, mw, dl, nil)

	if err := p.Process(context.Background(), sampleSignal()); err != nil {
		t.Fatalf("process: %v", err)
	}

	if wi.inserts.Load() != 1 {
		t.Errorf("Insert: want 1 call, got %d", wi.inserts.Load())
	}
	if wi.increments.Load() != 0 {
		t.Errorf("Increment: want 0 calls, got %d", wi.increments.Load())
	}
	if sig.inserts.Load() != 1 || mw.inserts.Load() != 1 {
		t.Errorf("fan-out: signals=%d metrics=%d (want 1 each)", sig.inserts.Load(), mw.inserts.Load())
	}
	if len(dl.records) != 0 {
		t.Errorf("no dead-letters expected, got %d", len(dl.records))
	}
}

func TestProcess_JoinedFlow(t *testing.T) {
	d := &fakeDebouncer{result: debounce.Result{
		WorkItemID: uuid.New(), Action: debounce.ActionJoined, Count: 2,
	}}
	wi := &fakeWorkItems{}
	p := New(fastConfig(), d, wi, &fakeSignals{}, &fakeMetrics{}, &fakeDeadLetter{}, nil)

	if err := p.Process(context.Background(), sampleSignal()); err != nil {
		t.Fatalf("process: %v", err)
	}

	if wi.inserts.Load() != 0 {
		t.Errorf("Insert: want 0 calls, got %d", wi.inserts.Load())
	}
	if wi.increments.Load() != 1 {
		t.Errorf("Increment: want 1 call, got %d", wi.increments.Load())
	}
}

func TestProcess_RedisDegradedKeepsGoing(t *testing.T) {
	d := &fakeDebouncer{
		result: debounce.Result{
			WorkItemID: uuid.New(), Action: debounce.ActionCreated, Count: 1, Degraded: true,
		},
		err: debounce.ErrRedisDegraded,
	}
	wi := &fakeWorkItems{}
	p := New(fastConfig(), d, wi, &fakeSignals{}, &fakeMetrics{}, &fakeDeadLetter{}, nil)

	if err := p.Process(context.Background(), sampleSignal()); err != nil {
		t.Fatalf("process should swallow ErrRedisDegraded, got %v", err)
	}
	if wi.inserts.Load() != 1 {
		t.Error("fan-out should proceed despite redis degradation")
	}
}

func TestProcess_RetryOnFlakyPostgres(t *testing.T) {
	d := &fakeDebouncer{result: debounce.Result{
		WorkItemID: uuid.New(), Action: debounce.ActionCreated, Count: 1,
	}}
	wi := &fakeWorkItems{
		insertErr:     errors.New("connection reset"),
		insertErrOnce: true,
	}
	dl := &fakeDeadLetter{}
	p := New(fastConfig(), d, wi, &fakeSignals{}, &fakeMetrics{}, dl, nil)

	if err := p.Process(context.Background(), sampleSignal()); err != nil {
		t.Fatalf("process: %v", err)
	}
	if wi.inserts.Load() != 2 {
		t.Errorf("Insert calls: want 2 (1 fail + 1 success), got %d", wi.inserts.Load())
	}
	if dl.count("postgres") != 0 {
		t.Error("postgres should NOT have been dead-lettered")
	}
}

func TestProcess_DeadLetterOnExhaustion(t *testing.T) {
	d := &fakeDebouncer{result: debounce.Result{
		WorkItemID: uuid.New(), Action: debounce.ActionCreated, Count: 1,
	}}
	wi := &fakeWorkItems{insertErr: errors.New("postgres is down")}
	dl := &fakeDeadLetter{}
	p := New(fastConfig(), d, wi, &fakeSignals{}, &fakeMetrics{}, dl, nil)

	if err := p.Process(context.Background(), sampleSignal()); err != nil {
		t.Fatalf("process should swallow sink errors, got %v", err)
	}
	if dl.count("postgres") != 1 {
		t.Errorf("postgres dead-letters: want 1, got %d", dl.count("postgres"))
	}

	if dl.count("mongo") != 0 || dl.count("timescale") != 0 {
		t.Errorf("other sinks dead-lettered unexpectedly: %+v", dl.records)
	}
}

func TestProcess_DebouncerHardError(t *testing.T) {
	d := &fakeDebouncer{err: errors.New("script gone")}
	p := New(fastConfig(), d, &fakeWorkItems{}, &fakeSignals{}, &fakeMetrics{}, &fakeDeadLetter{}, nil)

	err := p.Process(context.Background(), sampleSignal())
	if err == nil {
		t.Fatal("expected error to propagate from debouncer")
	}
}
