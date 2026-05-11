-- 003_signal_metrics: TimescaleDB hypertable for per-signal timeseries
-- aggregations. One row per incoming signal (after debounce). Used by
-- the (bonus) Grafana dashboard and the metrics roll-ups for MTTR
-- distribution, debounce ratio, etc.
--
-- Why a hypertable and not a plain table:
--   * Auto-partitioning by `ts` keeps individual chunks small enough to
--     fit in memory; queries that filter on time can prune entire chunks.
--   * Continuous aggregates (added later, when needed) materialize
--     per-minute rollups so dashboard queries don't scan raw rows.
--
-- The timescaledb extension is enabled by docker/postgres/init.sql, so
-- create_hypertable is available here.

CREATE TABLE signal_metrics (
    ts             timestamptz NOT NULL,
    component_type text        NOT NULL,
    severity       text        NOT NULL,
    work_item_id   uuid        NOT NULL,
    count          integer     NOT NULL DEFAULT 1
);

-- Turn the table into a hypertable partitioned on `ts`. Default chunk
-- interval is 7 days — fine for our retention horizon.
SELECT create_hypertable('signal_metrics', 'ts');

-- Optional but recommended: an index on (component_type, ts DESC) lets
-- "per-component last hour" queries skip irrelevant chunks.
CREATE INDEX idx_signal_metrics_component_ts
    ON signal_metrics (component_type, ts DESC);
