package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/kubeboiii/ims/internal/model"
)

// startMongo boots an ephemeral MongoDB container, returns a connected
// client + the test database name. Cleans up on test end.
func startMongo(t *testing.T) (*mongo.Client, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping container-backed test in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	container, err := tcmongo.Run(ctx, "mongo:7.0.14")
	if err != nil {
		t.Fatalf("start mongo: %v", err)
	}
	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(container)
	})

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	client, err := NewClient(ctx, ClientConfig{URI: uri, Database: "ims_test"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Disconnect(context.Background())
	})
	return client, "ims_test"
}

func sampleSignalFor(component string) model.Signal {
	return model.Signal{
		SignalID:      uuid.New(),
		ComponentID:   component,
		ComponentType: model.ComponentRDBMS,
		Severity:      model.SeverityP1,
		Source:        "test",
		Timestamp:     time.Now().UTC(),
		Payload:       json.RawMessage(`{"err":"oom","retries":3}`),
	}
}

func TestSignalRepo_InsertAndCount(t *testing.T) {
	client, dbName := startMongo(t)
	db := client.Database(dbName)
	repo := NewSignalRepository(db)
	ctx := context.Background()

	if err := repo.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	wiID := uuid.New()
	for i := 0; i < 5; i++ {
		if err := repo.Insert(ctx, sampleSignalFor("CACHE_01"), wiID); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := repo.Insert(ctx, sampleSignalFor("RDBMS_01"), uuid.New()); err != nil {
			t.Fatalf("insert other: %v", err)
		}
	}

	n, err := repo.CountByComponent(ctx, "CACHE_01")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 5 {
		t.Errorf("want 5, got %d", n)
	}
}

func TestSignalRepo_PayloadStoredAsBSON(t *testing.T) {
	// Confirm we can query by a payload field, proving the JSON was
	// deserialised into native BSON (not stored as a string blob).
	client, dbName := startMongo(t)
	db := client.Database(dbName)
	repo := NewSignalRepository(db)
	ctx := context.Background()

	sig := model.Signal{
		SignalID:      uuid.New(),
		ComponentID:   "X",
		ComponentType: model.ComponentAPI,
		Severity:      model.SeverityP2,
		Source:        "test",
		Timestamp:     time.Now().UTC(),
		Payload:       json.RawMessage(`{"err":"timeout","ms":500}`),
	}
	if err := repo.Insert(ctx, sig, uuid.New()); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Find by nested payload field.
	var got map[string]any
	err := db.Collection(signalsCollection).
		FindOne(ctx, map[string]any{"payload.err": "timeout"}).
		Decode(&got)
	if err != nil {
		t.Fatalf("find by payload.err: %v", err)
	}
}

func TestDeadLetterRepo_Insert(t *testing.T) {
	client, dbName := startMongo(t)
	db := client.Database(dbName)
	dl := NewDeadLetterRepository(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := dl.Insert(ctx, "postgres",
			map[string]any{"work_item_id": uuid.New().String()},
			errors.New("connection refused"),
		); err != nil {
			t.Fatalf("insert dl: %v", err)
		}
	}
	n, err := dl.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Errorf("want 3, got %d", n)
	}
}
