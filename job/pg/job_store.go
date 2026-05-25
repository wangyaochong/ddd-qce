package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	ddderror "github.com/ddd-qce/core/error"
	jobcore "github.com/ddd-qce/core/job/core"
	corepg "github.com/ddd-qce/core/pg"
)

type PgJobStore struct {
	db       *sql.DB
	registry *jobcore.TypeRegistry
}

func NewJobStore(db *sql.DB, opts ...JobStoreOption) *PgJobStore {
	s := &PgJobStore{db: db}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type JobStoreOption func(*PgJobStore)

func WithTypeRegistry(registry *jobcore.TypeRegistry) JobStoreOption {
	return func(s *PgJobStore) {
		s.registry = registry
	}
}

func (s *PgJobStore) Create(ctx context.Context, job *jobcore.Job) error {
	q := corepg.GetQuerier(ctx, s.db)
	cmdData, err := json.Marshal(job.Command)
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}
	commandType := job.CommandType
	if commandType == "" {
		commandType = jobcore.TypeName(job.Command)
	}
	resultData, err := corepg.JSONOrNull(job.GetResult())
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	_, err = q.ExecContext(ctx,
		`INSERT INTO ddd_jobs (id, command, command_type, status, result, result_type, error, created_at, started_at, completed_at, timeout_ns, retry_count, max_retries)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		job.ID, cmdData, commandType, string(job.GetStatus()),
		resultData, corepg.NullString(job.GetResultType()),
		corepg.NullString(job.GetError()),
		job.CreatedAt, corepg.NullTime(job.GetStartedAt()), corepg.NullTime(job.GetCompletedAt()),
		job.Timeout.Nanoseconds(), job.RetryCount, job.MaxRetries,
	)
	return err
}

func (s *PgJobStore) Get(ctx context.Context, id string) (*jobcore.Job, error) {
	q := corepg.GetQuerier(ctx, s.db)
	row := q.QueryRowContext(ctx,
		`SELECT id, command, command_type, status, result, result_type, error, created_at, started_at, completed_at, timeout_ns, retry_count, max_retries
		 FROM ddd_jobs WHERE id = $1`, id,
	)
	var job jobcore.Job
	var cmdData []byte
	var resultData []byte
	var commandType string
	var status string
	var resultType sql.NullString
	var errStr sql.NullString
	var startedAt, completedAt sql.NullTime
	var timeoutNs int64
	if err := row.Scan(&job.ID, &cmdData, &commandType, &status, &resultData, &resultType, &errStr, &job.CreatedAt, &startedAt, &completedAt, &timeoutNs, &job.RetryCount, &job.MaxRetries); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job %s: %w", id, ddderror.ErrNotFound)
		}
		return nil, err
	}
	job.CommandType = commandType
	job.SetStatus(jobcore.JobStatus(status))
	job.Timeout = time.Duration(timeoutNs)
	if resultType.Valid {
		job.SetResultType(resultType.String)
	}
	if errStr.Valid {
		job.SetError(errStr.String)
	}
	if startedAt.Valid {
		job.SetStartedAt(startedAt.Time)
	}
	if completedAt.Valid {
		job.SetCompletedAt(completedAt.Time)
	}
	if len(cmdData) > 0 {
		cmd, err := s.unmarshalTyped(cmdData, commandType)
		if err != nil {
			return nil, fmt.Errorf("unmarshal command: %w", err)
		}
		job.Command = cmd
	}
	if len(resultData) > 0 {
		resultTypeName := ""
		if resultType.Valid {
			resultTypeName = resultType.String
		}
		result, err := s.unmarshalTyped(resultData, resultTypeName)
		if err != nil {
			return nil, fmt.Errorf("unmarshal result: %w", err)
		}
		job.SetResult(result)
	}
	return &job, nil
}

func (s *PgJobStore) Update(ctx context.Context, job *jobcore.Job) error {
	q := corepg.GetQuerier(ctx, s.db)
	resultType := job.GetResultType()
	if resultType == "" && job.GetResult() != nil {
		resultType = jobcore.TypeName(job.GetResult())
	}
	resultData, err := corepg.JSONOrNull(job.GetResult())
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	_, err = q.ExecContext(ctx,
		`UPDATE ddd_jobs SET status=$2, result=$3, result_type=$4, error=$5, started_at=$6, completed_at=$7, timeout_ns=$8, retry_count=$9, max_retries=$10
		 WHERE id=$1`,
		job.ID, string(job.GetStatus()), resultData, corepg.NullString(resultType), corepg.NullString(job.GetError()),
		corepg.NullTime(job.GetStartedAt()), corepg.NullTime(job.GetCompletedAt()),
		job.Timeout.Nanoseconds(), job.RetryCount, job.MaxRetries,
	)
	return err
}

func (s *PgJobStore) List(ctx context.Context, status jobcore.JobStatus) ([]*jobcore.Job, error) {
	q := corepg.GetQuerier(ctx, s.db)
	rows, err := q.QueryContext(ctx,
		`SELECT id, command, command_type, status, result, result_type, error, created_at, started_at, completed_at, timeout_ns, retry_count, max_retries
		 FROM ddd_jobs WHERE status = $1`, string(status),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*jobcore.Job
	for rows.Next() {
		var job jobcore.Job
		var cmdData []byte
		var resultData []byte
		var commandType string
		var statusStr string
		var resultType sql.NullString
		var errStr sql.NullString
		var startedAt, completedAt sql.NullTime
		var timeoutNs int64
		if err := rows.Scan(&job.ID, &cmdData, &commandType, &statusStr, &resultData, &resultType, &errStr, &job.CreatedAt, &startedAt, &completedAt, &timeoutNs, &job.RetryCount, &job.MaxRetries); err != nil {
			return nil, err
		}
		job.CommandType = commandType
		job.SetStatus(jobcore.JobStatus(statusStr))
		job.Timeout = time.Duration(timeoutNs)
		if resultType.Valid {
			job.SetResultType(resultType.String)
		}
		if errStr.Valid {
			job.SetError(errStr.String)
		}
		if startedAt.Valid {
			job.SetStartedAt(startedAt.Time)
		}
		if completedAt.Valid {
			job.SetCompletedAt(completedAt.Time)
		}
		if len(cmdData) > 0 {
			cmd, err := s.unmarshalTyped(cmdData, commandType)
			if err != nil {
				return nil, fmt.Errorf("unmarshal command for job %s: %w", job.ID, err)
			}
			job.Command = cmd
		}
		if len(resultData) > 0 {
			resultTypeName := ""
			if resultType.Valid {
				resultTypeName = resultType.String
			}
			result, err := s.unmarshalTyped(resultData, resultTypeName)
			if err != nil {
				return nil, fmt.Errorf("unmarshal result for job %s: %w", job.ID, err)
			}
			job.SetResult(result)
		}
		result = append(result, &job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return result, nil
}

func (s *PgJobStore) Delete(ctx context.Context, id string) error {
	q := corepg.GetQuerier(ctx, s.db)
	res, err := q.ExecContext(ctx, `DELETE FROM ddd_jobs WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("job %s: %w", id, ddderror.ErrNotFound)
	}
	return nil
}

func (s *PgJobStore) unmarshalTyped(data []byte, typeName string) (any, error) {
	if s.registry == nil || typeName == "" {
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("unmarshal untyped data: %w", err)
		}
		return v, nil
	}
	inst, ok := s.registry.NewInstance(typeName)
	if !ok {
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("unmarshal unregistered type %q: %w", typeName, err)
		}
		return v, nil
	}
	if err := json.Unmarshal(data, inst); err != nil {
		return nil, fmt.Errorf("unmarshal type %q: %w", typeName, err)
	}
	return inst, nil
}

func RecordJobExecution(ctx context.Context, db *sql.DB, jobID string, attempt int, status string, jobErr error, startedAt time.Time) error {
	q := corepg.GetQuerier(ctx, db)
	var errStr any
	if jobErr != nil {
		errStr = jobErr.Error()
	}
	var durationNs any
	if !startedAt.IsZero() {
		durationNs = time.Since(startedAt).Nanoseconds()
	}
	_, err := q.ExecContext(ctx,
		`INSERT INTO ddd_job_execution_log (job_id, attempt, status, error, started_at, completed_at, duration_ns)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		jobID, attempt, status, errStr, startedAt, time.Now(), durationNs,
	)
	return err
}
