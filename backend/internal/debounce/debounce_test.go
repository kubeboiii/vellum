package debounce

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func startRedis(t *testing.T) *Debouncer {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping container-backed test in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, err := tcredis.Run(ctx, "redis:7.4.1-alpine")
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(container)
	})

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	opts, err := redis.ParseURL(uri)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })

	sha, err := client.ScriptLoad(ctx, ScriptBody()).Result()
	if err != nil {
		t.Fatalf("script load: %v", err)
	}
	return New(client, sha, Config{WindowSeconds: 10, MaxSignals: 5})
}

func TestProcess_FirstSignalCreates(t *testing.T) {
	d := startRedis(t)
	res, err := d.Process(context.Background(), "CACHE_01")
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if res.Action != ActionCreated {
		t.Errorf("want CREATED, got %s", res.Action)
	}
	if res.Count != 1 {
		t.Errorf("want count 1, got %d", res.Count)
	}
	if res.WorkItemID == uuid.Nil {
		t.Error("work_item_id should not be nil")
	}
}

func TestProcess_SecondSignalJoinsSameWindow(t *testing.T) {
	d := startRedis(t)
	ctx := context.Background()

	first, err := d.Process(ctx, "CACHE_01")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := d.Process(ctx, "CACHE_01")
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if second.Action != ActionJoined {
		t.Errorf("second: want JOINED, got %s", second.Action)
	}
	if second.WorkItemID != first.WorkItemID {
		t.Errorf("second should share work_item_id; first=%v second=%v",
			first.WorkItemID, second.WorkItemID)
	}
	if second.Count != 2 {
		t.Errorf("want count 2, got %d", second.Count)
	}
}

func TestProcess_CapTriggersNewWindow(t *testing.T) {
	d := startRedis(t)
	ctx := context.Background()

	var firstID uuid.UUID
	for i := 0; i < 5; i++ {
		r, err := d.Process(ctx, "CACHE_01")
		if err != nil {
			t.Fatalf("process %d: %v", i, err)
		}
		if i == 0 {
			firstID = r.WorkItemID
		} else if r.WorkItemID != firstID {
			t.Errorf("signal %d: expected to join %v, got %v", i, firstID, r.WorkItemID)
		}
	}

	r, err := d.Process(ctx, "CACHE_01")
	if err != nil {
		t.Fatalf("6th: %v", err)
	}
	if r.Action != ActionCreated {
		t.Errorf("6th: want CREATED, got %s", r.Action)
	}
	if r.WorkItemID == firstID {
		t.Error("6th: should NOT share work_item_id with the previous window")
	}
}

func TestProcess_DifferentComponents_AreIndependent(t *testing.T) {
	d := startRedis(t)
	ctx := context.Background()

	a, _ := d.Process(ctx, "CACHE_01")
	b, _ := d.Process(ctx, "RDBMS_01")

	if a.Action != ActionCreated || b.Action != ActionCreated {
		t.Errorf("both should CREATE; got A=%s B=%s", a.Action, b.Action)
	}
	if a.WorkItemID == b.WorkItemID {
		t.Error("different components must get different work_item_ids")
	}
}

func TestProcess_ConcurrentSameComponent(t *testing.T) {
	d := startRedis(t)
	ctx := context.Background()

	const N = 50

	d.cfg.MaxSignals = N + 1

	results := make([]Result, N)
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := d.Process(ctx, "STORM_01")
			if err != nil {
				t.Errorf("process %d: %v", i, err)
				return
			}
			results[i] = r
		}(i)
	}
	wg.Wait()

	var creates int
	idSet := map[uuid.UUID]struct{}{}
	for _, r := range results {
		if r.Action == ActionCreated {
			creates++
		}
		idSet[r.WorkItemID] = struct{}{}
	}
	if creates != 1 {
		t.Errorf("exactly 1 CREATE expected, got %d", creates)
	}
	if len(idSet) != 1 {
		t.Errorf("all 50 should share the same work_item_id; got %d distinct", len(idSet))
	}
}

func TestProcess_RedisDown_FallsBackToCreated(t *testing.T) {

	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = client.Close() })

	d := New(client, "fakesha", DefaultConfig())
	res, err := d.Process(context.Background(), "X")
	if err != ErrRedisDegraded {
		t.Fatalf("want ErrRedisDegraded, got %v", err)
	}
	if !res.Degraded {
		t.Error("Result.Degraded should be true")
	}
	if res.Action != ActionCreated {
		t.Errorf("fallback must CREATE, got %s", res.Action)
	}
	if res.Count != 1 {
		t.Errorf("fallback count want 1, got %d", res.Count)
	}
}
