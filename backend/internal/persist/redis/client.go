package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type ClientConfig struct {
	Addr           string
	Password       string
	DB             int
	ConnectTimeout time.Duration
}

func NewClient(ctx context.Context, cfg ClientConfig) (*redis.Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("redis: Addr is required")
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 3 * time.Second
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return client, nil
}

func LoadScript(ctx context.Context, client *redis.Client, body string) (string, error) {
	sha, err := client.ScriptLoad(ctx, body).Result()
	if err != nil {
		return "", fmt.Errorf("redis: SCRIPT LOAD: %w", err)
	}
	return sha, nil
}

type PingChecker struct{ Client *redis.Client }

func (p PingChecker) Ping(ctx context.Context) error { return p.Client.Ping(ctx).Err() }
func (PingChecker) Name() string                     { return "redis" }
