-- 001_work_items: source-of-truth table for incidents.
--
-- One row per "Work Item" (an aggregated incident on one component, per
-- 00-master-prd §4). The debouncer creates a new row when no active
-- window exists for the component_id; subsequent signals within the
-- window INCREMENT signal_count + bump last_signal_ts on the existing row.
--
-- Indexes chosen for the two hot read paths:
--   * Live feed: WHERE status != 'CLOSED' ORDER BY severity, last_signal_ts
--     → idx_work_items_active.
--   * Component lookup (Phase 4 PATCH /state): WHERE component_id = ? AND
--     status != 'CLOSED' → idx_work_items_component_active.

CREATE TABLE work_items (
    id              uuid PRIMARY KEY,
    component_id    text        NOT NULL,
    component_type  text        NOT NULL,
    severity        text        NOT NULL,
    status          text        NOT NULL DEFAULT 'OPEN',
    signal_count    integer     NOT NULL DEFAULT 1,
    first_signal_ts timestamptz NOT NULL,
    last_signal_ts  timestamptz NOT NULL,
    mttr_seconds    integer,                            -- filled on CLOSE (Phase 4)
    incident_start  timestamptz,                        -- editable in RCA (Phase 4)
    incident_end    timestamptz,                        -- editable in RCA (Phase 4)
    closed_at       timestamptz,                        -- set in OnEnter(ClosedState)
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    -- Allowed values are policed by the State pattern in code; CHECK is
    -- belt-and-braces in case raw SQL bypasses the app layer.
    CONSTRAINT work_items_status_chk
        CHECK (status IN ('OPEN', 'INVESTIGATING', 'RESOLVED', 'CLOSED')),
    CONSTRAINT work_items_severity_chk
        CHECK (severity IN ('P0', 'P1', 'P2', 'P3'))
);

-- Live feed sort: active items ordered by severity (P0 first), then by
-- most-recent-signal-time DESC. A partial index excludes CLOSED items so
-- the index stays small even as historical incidents accumulate.
CREATE INDEX idx_work_items_active
    ON work_items (severity, last_signal_ts DESC)
    WHERE status <> 'CLOSED';

-- Component lookup for the debouncer's Postgres-side fallback / for
-- Phase 4's transition path.
CREATE INDEX idx_work_items_component_active
    ON work_items (component_id)
    WHERE status <> 'CLOSED';
