// Package mongo holds the MongoDB client + the SignalRepository and
// DeadLetterRepository. Mongo is our high-volume audit log: every raw
// signal lands here, regardless of debounce decision (FR-3.4).
//
// We use the official `go.mongodb.org/mongo-driver/v2` library. The v1
// line is deprecated; v2 has the same API shape but cleaner generics
// and is the current supported branch.
package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ClientConfig parallels pg.PoolConfig — minimal knobs, sane defaults.
type ClientConfig struct {
	URI            string
	Database       string
	ConnectTimeout time.Duration
}

// NewClient opens a Mongo connection, pings to confirm reachability,
// and returns the typed *mongo.Client. Caller closes via
// `client.Disconnect(ctx)` at shutdown.
func NewClient(ctx context.Context, cfg ClientConfig) (*mongo.Client, error) {
	if cfg.URI == "" {
		return nil, fmt.Errorf("mongo: URI is required")
	}
	if cfg.Database == "" {
		cfg.Database = "ims"
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}

	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(cfg.URI))
	if err != nil {
		return nil, fmt.Errorf("mongo: connect: %w", err)
	}
	if err := client.Ping(connectCtx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo: ping: %w", err)
	}
	return client, nil
}
