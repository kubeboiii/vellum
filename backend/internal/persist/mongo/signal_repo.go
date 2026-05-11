package mongo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/kubeboiii/ims/internal/model"
)

// signalsCollection is the name of the collection holding the audit log.
// We hard-code it because it's not a tunable — the rest of the system
// (Phase 5 detail page reads) assumes this name.
const signalsCollection = "signals"

// SignalRepository writes raw signals to the `signals` collection.
// FR-3.4 in 00-master-prd: every signal is persisted regardless of
// debounce decision.
type SignalRepository struct {
	coll *mongo.Collection
}

// NewSignalRepository builds the repo against the given database.
// EnsureIndexes() should be called once at startup so the detail page
// queries (work_item_id, component_id) are fast — but we don't call it
// here to keep the constructor side-effect-free.
func NewSignalRepository(db *mongo.Database) *SignalRepository {
	return &SignalRepository{coll: db.Collection(signalsCollection)}
}

// Insert writes one signal stamped with the work_item_id the debouncer
// assigned. This is on the hot path — keep it cheap. We deliberately
// don't validate signal contents here; the ingest handler already did
// (FR-2 validation), and re-validating wastes cycles.
//
// Payload is the original JSON value, deserialised once and stored as
// native BSON. That means Mongo queries can later filter on payload
// fields (`db.signals.find({"payload.err": "oom"})`) without parsing
// JSON at query time. ~µs cost per signal.
func (r *SignalRepository) Insert(ctx context.Context, sig model.Signal, workItemID uuid.UUID) error {
	var payload any
	if len(sig.Payload) > 0 {
		if err := json.Unmarshal(sig.Payload, &payload); err != nil {
			return fmt.Errorf("mongo: unmarshal payload: %w", err)
		}
	}
	doc := bson.M{
		"signal_id":      sig.SignalID,
		"work_item_id":   workItemID,
		"component_id":   sig.ComponentID,
		"component_type": sig.ComponentType,
		"severity":       sig.Severity,
		"timestamp":      sig.Timestamp,
		"source":         sig.Source,
		"payload":        payload,
		"ingested_at":    time.Now().UTC(),
	}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("mongo: insert signal: %w", err)
	}
	return nil
}

// CountByComponent is a test/acceptance helper. Mirrors the equivalent
// in pg.WorkItemRepository.CountByComponent. Used by the Phase 3 demo
// to prove "200 raw signals landed in Mongo".
func (r *SignalRepository) CountByComponent(ctx context.Context, componentID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"component_id": componentID})
}

// EnsureIndexes creates the two indexes the detail page needs:
//   - (component_id, timestamp DESC) for "raw signals from this component"
//   - (work_item_id, timestamp DESC) for "all signals attached to this
//     work item" (Phase 5 detail page).
//
// Mongo's CreateIndexes is idempotent: re-running on existing indexes
// is a no-op. Safe to call at every startup.
func (r *SignalRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "component_id", Value: 1}, {Key: "timestamp", Value: -1}}},
		{Keys: bson.D{{Key: "work_item_id", Value: 1}, {Key: "timestamp", Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("mongo: ensure indexes: %w", err)
	}
	return nil
}

// Ping is the /health probe.
func (r *SignalRepository) Ping(ctx context.Context) error {
	return r.coll.Database().Client().Ping(ctx, nil)
}

// Name identifies this dep in /health responses.
func (r *SignalRepository) Name() string { return "mongo" }
