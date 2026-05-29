package pg

import (
	"database/sql"
)

func Migrate(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS ddd_domain_events (
			id             BIGSERIAL PRIMARY KEY,
			aggregate_id   TEXT NOT NULL,
			event_type     TEXT NOT NULL,
			event_data     JSONB NOT NULL,
			occurred_at    TIMESTAMPTZ NOT NULL,
			version        INT NOT NULL DEFAULT 0,
			correlation_id TEXT NOT NULL DEFAULT '',
			causation_id   TEXT NOT NULL DEFAULT '',
			UNIQUE(aggregate_id, version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_events_aggregate ON ddd_domain_events(aggregate_id, version)`,

		`CREATE TABLE IF NOT EXISTS ddd_jobs (
			id           TEXT PRIMARY KEY,
			command      JSONB NOT NULL,
			command_type TEXT NOT NULL,
			status       TEXT NOT NULL,
			result       JSONB,
			result_type  TEXT,
			error        TEXT,
			created_at   TIMESTAMPTZ NOT NULL,
			started_at   TIMESTAMPTZ,
			completed_at TIMESTAMPTZ,
			timeout_ns   BIGINT DEFAULT 0,
			retry_count  INT DEFAULT 0,
			max_retries  INT DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS ddd_job_execution_log (
			id           BIGSERIAL PRIMARY KEY,
			job_id       TEXT NOT NULL,
			attempt      INT NOT NULL,
			status       TEXT NOT NULL,
			error        TEXT,
			started_at   TIMESTAMPTZ NOT NULL,
			completed_at TIMESTAMPTZ,
			duration_ns  BIGINT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_jel_job ON ddd_job_execution_log(job_id)`,

		`CREATE TABLE IF NOT EXISTS ddd_spans (
			id          TEXT PRIMARY KEY,
			trace_id    TEXT NOT NULL,
			parent_id   TEXT,
			type        TEXT NOT NULL,
			name        TEXT NOT NULL,
			status      TEXT NOT NULL,
			error       TEXT,
			started_at  TIMESTAMPTZ NOT NULL,
			duration_ns BIGINT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_spans_trace ON ddd_spans(trace_id)`,

		`CREATE TABLE IF NOT EXISTS ddd_aggregate_snapshots (
			aggregate_id   TEXT PRIMARY KEY,
			aggregate_type TEXT NOT NULL,
			snapshot_data  JSONB NOT NULL,
			version        INT NOT NULL DEFAULT 0,
			updated_at     TIMESTAMPTZ NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS ddd_command_log (
			id            BIGSERIAL PRIMARY KEY,
			trace_id      TEXT,
			span_id       TEXT,
			command_type  TEXT NOT NULL,
			command_data  JSONB,
			result_type   TEXT,
			result_data   JSONB,
			error         TEXT,
			duration_ns   BIGINT,
			created_at    TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_command_log_type ON ddd_command_log(command_type)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_command_log_trace ON ddd_command_log(trace_id)`,

		`CREATE TABLE IF NOT EXISTS ddd_query_log (
			id           BIGSERIAL PRIMARY KEY,
			trace_id     TEXT,
			span_id      TEXT,
			query_type   TEXT NOT NULL,
			query_data   JSONB NOT NULL,
			result_type  TEXT,
			result_data  JSONB,
			error        TEXT,
			duration_ns  BIGINT,
			created_at   TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_query_log_type ON ddd_query_log(query_type)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_query_log_trace ON ddd_query_log(trace_id)`,

		`CREATE TABLE IF NOT EXISTS ddd_event_log (
			id            BIGSERIAL PRIMARY KEY,
			trace_id      TEXT,
			span_id       TEXT,
			aggregate_id  TEXT NOT NULL,
			event_type    TEXT NOT NULL,
			event_data    JSONB NOT NULL,
			handler_count INT DEFAULT 0,
			error         TEXT,
			duration_ns   BIGINT,
			created_at    TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_event_log_type ON ddd_event_log(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_event_log_aggregate ON ddd_event_log(aggregate_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_event_log_trace ON ddd_event_log(trace_id)`,

		`CREATE TABLE IF NOT EXISTS ddd_event_handler_log (
			id            BIGSERIAL PRIMARY KEY,
			event_log_id  BIGINT,
			trace_id      TEXT,
			span_id       TEXT,
			aggregate_id  TEXT NOT NULL,
			event_type    TEXT NOT NULL,
			handler_type  TEXT NOT NULL,
			status        TEXT NOT NULL,
			error         TEXT,
			duration_ns   BIGINT,
			created_at    TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_ehl_event_log ON ddd_event_handler_log(event_log_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_ehl_handler ON ddd_event_handler_log(handler_type)`,
		`CREATE INDEX IF NOT EXISTS idx_ddd_ehl_status ON ddd_event_handler_log(status)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func DropAll(db *sql.DB) error {
	tables := []string{
		"ddd_event_handler_log",
		"ddd_event_log",
		"ddd_query_log",
		"ddd_command_log",
		"ddd_aggregate_snapshots",
		"ddd_spans",
		"ddd_job_execution_log",
		"ddd_jobs",
		"ddd_domain_events",
	}
	for _, t := range tables {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + t + " CASCADE"); err != nil {
			return err
		}
	}
	return nil
}

func TruncateAll(db *sql.DB) error {
	tables := []string{
		"ddd_event_handler_log",
		"ddd_event_log",
		"ddd_query_log",
		"ddd_command_log",
		"ddd_aggregate_snapshots",
		"ddd_spans",
		"ddd_job_execution_log",
		"ddd_jobs",
		"ddd_domain_events",
	}
	for _, t := range tables {
		if _, err := db.Exec("TRUNCATE TABLE " + t + " CASCADE"); err != nil {
			return err
		}
	}
	return nil
}
