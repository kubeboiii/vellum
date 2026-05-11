-- 002_state_transitions: append-only audit log of every state change
-- (00-master-prd §4.4.4). Required for Rule 2 in CLAUDE.md (state
-- transitions are transactional). Phase 4 inserts the rows; Phase 3 just
-- creates the table so the foreign key is in place.

CREATE TABLE state_transitions (
    id            uuid PRIMARY KEY,
    work_item_id  uuid        NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
    from_state    text        NOT NULL,
    to_state      text        NOT NULL,
    reason        text,                       -- optional free-form
    actor         text,                       -- user id or "system"
    created_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT state_transitions_states_chk
        CHECK (from_state IN ('OPEN', 'INVESTIGATING', 'RESOLVED')
           AND to_state   IN ('INVESTIGATING', 'RESOLVED', 'CLOSED')
           AND from_state <> to_state)
);

-- Chronological lookup by work item ("show me this incident's timeline").
-- The (work_item_id, created_at) order matches the detail page query
-- (Phase 5 frontend).
CREATE INDEX idx_state_transitions_wi_time
    ON state_transitions (work_item_id, created_at);
