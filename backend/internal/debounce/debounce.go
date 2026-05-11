package debounce

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var scriptBody string

func ScriptBody() string { return scriptBody }

type Action string

const (
	ActionCreated Action = "CREATED"
	ActionJoined  Action = "JOINED"
)

type Result struct {
	WorkItemID uuid.UUID
	Action     Action
	Count      int
	Degraded   bool
}

type Config struct {
	WindowSeconds int
	MaxSignals    int
}

func DefaultConfig() Config {
	return Config{WindowSeconds: 10, MaxSignals: 100}
}

type Debouncer struct {
	client redis.Scripter
	script *redis.Script
	cfg    Config
}

func New(client redis.Scripter, scriptSHA string, cfg Config) *Debouncer {
	_ = scriptSHA

	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = 10
	}
	if cfg.MaxSignals <= 0 {
		cfg.MaxSignals = 100
	}
	return &Debouncer{
		client: client,
		script: redis.NewScript(scriptBody),
		cfg:    cfg,
	}
}

var ErrRedisDegraded = errors.New("debounce: redis unavailable, used fallback")

func (d *Debouncer) Process(ctx context.Context, componentID string) (Result, error) {
	candidateID := uuid.New()

	keys := []string{
		"debounce:" + componentID + ":work_item",
		"debounce:" + componentID + ":count",
	}
	args := []any{
		candidateID.String(),
		strconv.Itoa(d.cfg.WindowSeconds),
		strconv.Itoa(d.cfg.MaxSignals),
	}

	raw, err := d.script.Run(ctx, d.client, keys, args...).Result()
	if err != nil {

		return Result{
			WorkItemID: candidateID,
			Action:     ActionCreated,
			Count:      1,
			Degraded:   true,
		}, ErrRedisDegraded
	}

	return parseScriptResult(raw, candidateID)
}

func parseScriptResult(raw any, candidate uuid.UUID) (Result, error) {
	arr, ok := raw.([]any)
	if !ok || len(arr) != 3 {
		return Result{}, fmt.Errorf("debounce: unexpected script result type %T", raw)
	}

	idStr, ok := arr[0].(string)
	if !ok {
		return Result{}, fmt.Errorf("debounce: result[0] not a string: %T", arr[0])
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return Result{}, fmt.Errorf("debounce: parse work_item_id: %w", err)
	}

	actionStr, ok := arr[1].(string)
	if !ok {
		return Result{}, fmt.Errorf("debounce: result[1] not a string: %T", arr[1])
	}

	count, ok := arr[2].(int64)
	if !ok {
		return Result{}, fmt.Errorf("debounce: result[2] not an int64: %T", arr[2])
	}

	res := Result{
		WorkItemID: id,
		Action:     Action(actionStr),
		Count:      int(count),
	}

	if res.Action == ActionCreated && res.WorkItemID != candidate {
		return Result{}, fmt.Errorf("debounce: CREATED but id mismatch (%v vs %v)", res.WorkItemID, candidate)
	}
	return res, nil
}

func (d *Debouncer) WindowDuration() time.Duration {
	return time.Duration(d.cfg.WindowSeconds) * time.Second
}
