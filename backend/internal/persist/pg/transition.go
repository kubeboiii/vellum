package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kubeboiii/vellum/internal/model"
)

var ErrSerializationFailure = errors.New("pg: serialization failure (40001); retry")

func wrapPgError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "40001" {
		return fmt.Errorf("%w: %v", ErrSerializationFailure, err)
	}
	return err
}

func (r *WorkItemRepository) BeginTx(ctx context.Context) (*workItemTx, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("pg: begin tx: %w", err)
	}
	return &workItemTx{tx: tx}, nil
}

type workItemTx struct {
	tx pgx.Tx
}

func (t *workItemTx) LockWorkItem(ctx context.Context, id uuid.UUID) (model.WorkItem, error) {
	const q = `
		SELECT id, component_id, component_type, severity, status,
		       signal_count, first_signal_ts, last_signal_ts,
		       mttr_seconds, incident_start, incident_end, closed_at,
		       created_at, updated_at
		  FROM work_items
		 WHERE id = $1
		 FOR UPDATE
	`
	var wi model.WorkItem
	var componentType, severity, status string
	err := t.tx.QueryRow(ctx, q, id).Scan(
		&wi.ID,
		&wi.ComponentID,
		&componentType,
		&severity,
		&status,
		&wi.SignalCount,
		&wi.FirstSignalTS,
		&wi.LastSignalTS,
		&wi.MTTRSeconds,
		&wi.IncidentStart,
		&wi.IncidentEnd,
		&wi.ClosedAt,
		&wi.CreatedAt,
		&wi.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.WorkItem{}, ErrNotFound
		}
		return model.WorkItem{}, fmt.Errorf("pg: lock work_item: %w", wrapPgError(err))
	}
	wi.ComponentType = model.ComponentType(componentType)
	wi.Severity = model.Severity(severity)
	wi.Status = model.Status(status)
	return wi, nil
}

func (t *workItemTx) UpdateWorkItemStateAndMTTR(ctx context.Context, wi model.WorkItem) error {
	const q = `
		UPDATE work_items
		   SET status         = $2,
		       mttr_seconds   = $3,
		       incident_start = $4,
		       incident_end   = $5,
		       closed_at      = $6,
		       updated_at     = now()
		 WHERE id = $1
	`
	tag, err := t.tx.Exec(ctx, q,
		wi.ID,
		string(wi.Status),
		wi.MTTRSeconds,
		wi.IncidentStart,
		wi.IncidentEnd,
		wi.ClosedAt,
	)
	if err != nil {
		return fmt.Errorf("pg: update work_item state: %w", wrapPgError(err))
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (t *workItemTx) InsertStateTransition(ctx context.Context, st model.StateTransition) error {
	const q = `
		INSERT INTO state_transitions
		    (id, work_item_id, from_state, to_state, reason, actor)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''))
	`
	_, err := t.tx.Exec(ctx, q,
		st.ID, st.WorkItemID,
		string(st.FromState), string(st.ToState),
		st.Reason, st.Actor,
	)
	if err != nil {
		return fmt.Errorf("pg: insert state_transition: %w", wrapPgError(err))
	}
	return nil
}

func (t *workItemTx) InsertRCA(ctx context.Context, rca model.RCA) error {
	const q = `
		INSERT INTO rca
		    (id, work_item_id, incident_start, incident_end,
		     root_cause_category, fix_applied, prevention_steps,
		     submitted_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := t.tx.Exec(ctx, q,
		rca.ID, rca.WorkItemID,
		rca.IncidentStart, rca.IncidentEnd,
		string(rca.RootCauseCategory),
		rca.FixApplied, rca.PreventionSteps,
		rca.SubmittedBy, rca.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("pg: insert rca: %w", wrapPgError(err))
	}
	return nil
}

func (t *workItemTx) Commit() error {
	if err := t.tx.Commit(context.Background()); err != nil {
		return fmt.Errorf("pg: commit: %w", wrapPgError(err))
	}
	return nil
}

func (t *workItemTx) Rollback() error {
	_ = t.tx.Rollback(context.Background())
	return nil
}
