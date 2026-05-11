package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const deadLetterCollection = "dead_letter"

type DeadLetterRepository struct {
	coll *mongo.Collection
}

func NewDeadLetterRepository(db *mongo.Database) *DeadLetterRepository {
	return &DeadLetterRepository{coll: db.Collection(deadLetterCollection)}
}

type Record struct {
	Sink     string    `bson:"sink"`
	Payload  any       `bson:"payload"`
	Err      string    `bson:"err"`
	FailedAt time.Time `bson:"failed_at"`
}

func (r *DeadLetterRepository) Insert(ctx context.Context, sink string, payload any, err error) error {
	rec := Record{
		Sink:     sink,
		Payload:  payload,
		Err:      err.Error(),
		FailedAt: time.Now().UTC(),
	}
	if _, ierr := r.coll.InsertOne(ctx, rec); ierr != nil {
		return fmt.Errorf("mongo: insert dead_letter: %w", ierr)
	}
	return nil
}

func (r *DeadLetterRepository) Count(ctx context.Context) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{})
}
