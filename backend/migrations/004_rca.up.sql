-- 004_rca: structured post-mortem record attached to a Work Item.
-- Required fields per 00-master-prd §4.5. The State pattern's
-- ResolvedState.CanTransitionTo(ClosedState) refuses to close a WI
-- without an RCA, so by the time a row lands here, the WI is about
-- to be (or has just been) marked CLOSED in the same transaction.
--
-- UNIQUE on work_item_id enforces "one RCA per Work Item" at the DB
-- level — even if app code has a bug, you cannot accidentally insert
-- two RCAs for the same incident.

CREATE TABLE rca (
    id                   uuid PRIMARY KEY,
    work_item_id         uuid NOT NULL UNIQUE
                              REFERENCES work_items(id) ON DELETE CASCADE,
    incident_start       timestamptz NOT NULL,
    incident_end         timestamptz NOT NULL,
    root_cause_category  text        NOT NULL,
    fix_applied          text        NOT NULL,
    prevention_steps     text        NOT NULL,
    submitted_by         text        NOT NULL DEFAULT 'system',
    created_at           timestamptz NOT NULL DEFAULT now(),

    -- Belt-and-braces: the app's RCA.Validate() enforces these, but DB
    -- constraints stop a `psql` user from poisoning the schema.
    CONSTRAINT rca_category_chk
        CHECK (root_cause_category IN ('CODE_DEFECT', 'INFRASTRUCTURE',
            'CONFIG_CHANGE', 'EXTERNAL_DEPENDENCY', 'CAPACITY',
            'HUMAN_ERROR', 'OTHER')),
    CONSTRAINT rca_fix_min_length
        CHECK (char_length(fix_applied) >= 20),
    CONSTRAINT rca_prevention_min_length
        CHECK (char_length(prevention_steps) >= 20),
    CONSTRAINT rca_time_order
        CHECK (incident_end >= incident_start)
);

-- Lookup index isn't strictly needed because work_item_id is UNIQUE
-- (Postgres auto-creates an index for UNIQUE constraints), but stating
-- it explicitly documents intent and matches Phase 5's GET handler.
