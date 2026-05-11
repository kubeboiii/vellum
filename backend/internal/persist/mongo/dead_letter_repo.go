package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// deadLetterCollection holds records that exhausted retry attempts
// against a sink. We use Mongo (not Postgres) for the dead-letter store
// because (a) the records have heterogeneous shapes — different sinks
// fail with different metadata, and (b) Mongo is the *only* sink
// guaranteed to still be reachable when Postgres is what failed.
const deadLetterCollection = "dead_letter"

// DeadLetterRepository writes records that survived retry exhaustion.
// 01-architecture §6.3: "human-inspected; we do not auto-replay in v1."
type DeadLetterRepository struct {
	coll *mongo.Collection
}

// NewDeadLetterRepository builds the repo against the given database.
func NewDeadLetterRepository(db *mongo.Database) *DeadLetterRepository {
	return &DeadLetterRepository{coll: db.Collection(deadLetterCollection)}
}

// Record is the public shape callers fill in. Sink is which downstream
// failed ("postgres", "mongo", "timescale"); Payload is the original
// item we tried to write (we use `any` so each sink can shove in its
// own struct); Err is the final error message.
type Record struct {
	Sink     string    `bson:"sink"`
	Payload  any       `bson:"payload"`
	Err      string    `bson:"err"`
	FailedAt time.Time `bson:"failed_at"`
}

// Insert appends one dead-letter entry. We deliberately do NOT retry
// the dead-letter write — if Mongo is down too, we log to stderr and
// move on (better than a worker wedging forever). The fact that we got
// here at all means the system is in a degraded state; losing the DL
// entry is acceptable in that mode.
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

// Count returns the total number of dead-letter records. Used in tests
// and by operators inspecting the queue.
func (r *DeadLetterRepository) Count(ctx context.Context) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{})
}
