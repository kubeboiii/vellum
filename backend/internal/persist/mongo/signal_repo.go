package mongo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/kubeboiii/vellum/internal/model"
)

const signalsCollection = "signals"

type SignalRepository struct {
	coll *mongo.Collection
}

func NewSignalRepository(db *mongo.Database) *SignalRepository {
	return &SignalRepository{coll: db.Collection(signalsCollection)}
}

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

func (r *SignalRepository) CountByComponent(ctx context.Context, componentID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"component_id": componentID})
}

type SignalPage struct {
	Items   []map[string]any `json:"items"`
	Page    int              `json:"page"`
	PerPage int              `json:"per_page"`
	Total   int64            `json:"total"`
}

func (r *SignalRepository) ListByWorkItem(ctx context.Context, workItemID uuid.UUID, page, perPage int) (SignalPage, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	filter := bson.M{"work_item_id": workItemID}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return SignalPage{}, fmt.Errorf("mongo: count signals: %w", err)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetSkip(int64((page - 1) * perPage)).
		SetLimit(int64(perPage))

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return SignalPage{}, fmt.Errorf("mongo: find signals: %w", err)
	}
	defer cursor.Close(ctx)

	items := make([]map[string]any, 0, perPage)
	for cursor.Next(ctx) {

		var typed struct {
			SignalID   uuid.UUID `bson:"signal_id"`
			WorkItemID uuid.UUID `bson:"work_item_id"`
		}
		if err := cursor.Decode(&typed); err != nil {
			return SignalPage{}, fmt.Errorf("mongo: decode signal uuids: %w", err)
		}
		var doc map[string]any
		if err := cursor.Decode(&doc); err != nil {
			return SignalPage{}, fmt.Errorf("mongo: decode signal: %w", err)
		}
		delete(doc, "_id")

		doc["signal_id"] = typed.SignalID.String()
		doc["work_item_id"] = typed.WorkItemID.String()
		items = append(items, doc)
	}
	if err := cursor.Err(); err != nil {
		return SignalPage{}, fmt.Errorf("mongo: cursor: %w", err)
	}
	return SignalPage{Items: items, Page: page, PerPage: perPage, Total: total}, nil
}

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

func (r *SignalRepository) Ping(ctx context.Context) error {
	return r.coll.Database().Client().Ping(ctx, nil)
}

func (r *SignalRepository) Name() string { return "mongo" }
