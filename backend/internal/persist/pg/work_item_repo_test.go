package pg

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/kubeboiii/vellum/internal/model"
)

func startPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping container-backed test in -short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	_, thisFile, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations")

	container, err := tcpostgres.Run(ctx,

		"timescale/timescaledb:2.17.2-pg16",
		tcpostgres.WithDatabase("ims"),
		tcpostgres.WithUsername("ims"),
		tcpostgres.WithPassword("ims"),

		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get DSN: %v", err)
	}

	pool, err := NewPool(ctx, PoolConfig{DSN: dsn, MaxConns: 4})
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS timescaledb"); err != nil {
		t.Fatalf("enable timescaledb: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}

	for _, path := range matches {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(path), err)
		}
	}
	return pool
}

func sampleSignal() model.Signal {
	return model.Signal{
		SignalID:      uuid.New(),
		ComponentID:   "RDBMS_PRIMARY_01",
		ComponentType: model.ComponentRDBMS,
		Severity:      model.SeverityP0,
		Source:        "test",
		Timestamp:     time.Now().UTC(),
		Payload:       json.RawMessage(`{"err":"oom"}`),
	}
}

func TestWorkItemRepo_InsertAndIncrement(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	ctx := context.Background()

	sig := sampleSignal()
	wi := model.NewWorkItem(uuid.New(), sig)

	if err := repo.Insert(ctx, wi); err != nil {
		t.Fatalf("insert: %v", err)
	}

	ts := sig.Timestamp
	for i := 0; i < 3; i++ {
		ts = ts.Add(time.Second)
		if err := repo.IncrementSignalCount(ctx, wi.ID, ts); err != nil {
			t.Fatalf("increment %d: %v", i, err)
		}
	}

	var (
		count      int
		lastSignal time.Time
	)
	err := pool.QueryRow(ctx,
		`SELECT signal_count, last_signal_ts FROM work_items WHERE id = $1`,
		wi.ID,
	).Scan(&count, &lastSignal)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if count != 4 {
		t.Errorf("signal_count: want 4, got %d", count)
	}
	if !lastSignal.Equal(ts) {
		t.Errorf("last_signal_ts: want %v, got %v", ts, lastSignal)
	}
}

func TestWorkItemRepo_IncrementNotFound(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	err := repo.IncrementSignalCount(context.Background(), uuid.New(), time.Now())
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

func TestWorkItemRepo_ConcurrentIncrement(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	ctx := context.Background()

	wi := model.NewWorkItem(uuid.New(), sampleSignal())
	if err := repo.Insert(ctx, wi); err != nil {
		t.Fatalf("insert: %v", err)
	}

	const N = 50
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ts := wi.LastSignalTS.Add(time.Duration(i) * time.Millisecond)
			if err := repo.IncrementSignalCount(ctx, wi.ID, ts); err != nil {
				t.Errorf("increment: %v", err)
			}
		}(i)
	}
	wg.Wait()

	var count int
	err := pool.QueryRow(ctx, `SELECT signal_count FROM work_items WHERE id = $1`, wi.ID).Scan(&count)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if count != N+1 {
		t.Errorf("signal_count: want %d, got %d (lost updates?)", N+1, count)
	}
}

func TestWorkItemRepo_CountByComponent(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	ctx := context.Background()

	if err := repo.Insert(ctx, model.NewWorkItem(uuid.New(), sampleSignal())); err != nil {
		t.Fatal(err)
	}
	if err := repo.Insert(ctx, model.NewWorkItem(uuid.New(), sampleSignal())); err != nil {
		t.Fatal(err)
	}

	n, err := repo.CountByComponent(ctx, "RDBMS_PRIMARY_01")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Errorf("want 2, got %d", n)
	}
}
