package observability

import (
	"context"
	"fmt"
)

type SchemaReader interface {
	ListTables(ctx context.Context) ([]TableInfo, error)
	GetTable(ctx context.Context, name string) (*TableDetail, error)
	ListRelations(ctx context.Context) ([]TableRelation, error)
}

type InMemorySchemaReader struct{}

func NewInMemorySchemaReader() *InMemorySchemaReader {
	return &InMemorySchemaReader{}
}

var _ SchemaReader = (*InMemorySchemaReader)(nil)

func (r *InMemorySchemaReader) ListTables(_ context.Context) ([]TableInfo, error) {
	tables := staticSchema()
	result := make([]TableInfo, 0, len(tables))
	for _, t := range tables {
		info := t.TableInfo
		info.RowCount = -1
		info.DiskSize = -1
		info.AvgRowSize = -1
		result = append(result, info)
	}
	return result, nil
}

func (r *InMemorySchemaReader) GetTable(_ context.Context, name string) (*TableDetail, error) {
	tables := staticSchema()
	for _, t := range tables {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("table %q not found", name)
}

func (r *InMemorySchemaReader) ListRelations(_ context.Context) ([]TableRelation, error) {
	return staticRelations(), nil
}

func staticRelations() []TableRelation {
	return []TableRelation{
		{FromTable: "ddd_job_execution_log", FromColumn: "job_id", ToTable: "ddd_jobs", ToColumn: "id"},
		{FromTable: "ddd_event_handler_log", FromColumn: "event_log_id", ToTable: "ddd_event_log", ToColumn: "id"},
	}
}

func tableDescription(name string) string {
	descs := map[string]string{
		"ddd_domain_events":       "事件溯源事件存储",
		"ddd_jobs":                "异步任务队列",
		"ddd_job_execution_log":   "任务执行日志",
		"ddd_spans":               "分布式追踪",
		"ddd_aggregate_snapshots": "聚合快照",
		"ddd_command_log":         "命令审计日志",
		"ddd_query_log":           "查询审计日志",
		"ddd_event_log":           "事件审计日志",
		"ddd_event_handler_log":   "事件处理器日志",
	}
	return descs[name]
}

func staticSchema() []TableDetail {
	return []TableDetail{
		{
			TableInfo: TableInfo{Name: "ddd_domain_events", Description: tableDescription("ddd_domain_events")},
			Columns: []ColumnInfo{
				{Name: "id", Type: "BIGSERIAL", Nullable: false, IsPrimaryKey: true, Description: "自增主键"},
				{Name: "aggregate_id", Type: "TEXT", Nullable: false, Description: "聚合ID"},
				{Name: "event_type", Type: "TEXT", Nullable: false, Description: "事件类型"},
				{Name: "event_data", Type: "JSONB", Nullable: false, Description: "事件数据"},
				{Name: "occurred_at", Type: "TIMESTAMPTZ", Nullable: false, Description: "发生时间"},
				{Name: "version", Type: "INT", Nullable: false, DefaultValue: "0", Description: "版本号"},
				{Name: "correlation_id", Type: "TEXT", Nullable: false, DefaultValue: "''", Description: "关联ID"},
				{Name: "causation_id", Type: "TEXT", Nullable: false, DefaultValue: "''", Description: "因果ID"},
			},
			Indexes: []IndexInfo{
				{Name: "ddd_domain_events_pkey", Columns: []string{"id"}, Unique: true},
				{Name: "ddd_domain_events_aggregate_id_version_key", Columns: []string{"aggregate_id", "version"}, Unique: true},
				{Name: "idx_ddd_events_aggregate", Columns: []string{"aggregate_id", "version"}, Unique: false},
			},
		},
		{
			TableInfo: TableInfo{Name: "ddd_jobs", Description: tableDescription("ddd_jobs")},
			Columns: []ColumnInfo{
				{Name: "id", Type: "TEXT", Nullable: false, IsPrimaryKey: true, Description: "任务ID"},
				{Name: "command", Type: "JSONB", Nullable: false, Description: "命令数据"},
				{Name: "command_type", Type: "TEXT", Nullable: false, Description: "命令类型"},
				{Name: "status", Type: "TEXT", Nullable: false, Description: "任务状态"},
				{Name: "result", Type: "JSONB", Nullable: true, Description: "执行结果"},
				{Name: "result_type", Type: "TEXT", Nullable: true, Description: "结果类型"},
				{Name: "error", Type: "TEXT", Nullable: true, Description: "错误信息"},
				{Name: "created_at", Type: "TIMESTAMPTZ", Nullable: false, Description: "创建时间"},
				{Name: "started_at", Type: "TIMESTAMPTZ", Nullable: true, Description: "开始时间"},
				{Name: "completed_at", Type: "TIMESTAMPTZ", Nullable: true, Description: "完成时间"},
				{Name: "timeout_ns", Type: "BIGINT", Nullable: true, DefaultValue: "0", Description: "超时(纳秒)"},
				{Name: "retry_count", Type: "INT", Nullable: true, DefaultValue: "0", Description: "重试次数"},
				{Name: "max_retries", Type: "INT", Nullable: true, DefaultValue: "0", Description: "最大重试次数"},
			},
			Indexes: []IndexInfo{
				{Name: "ddd_jobs_pkey", Columns: []string{"id"}, Unique: true},
			},
		},
		{
			TableInfo: TableInfo{Name: "ddd_job_execution_log", Description: tableDescription("ddd_job_execution_log")},
			Columns: []ColumnInfo{
				{Name: "id", Type: "BIGSERIAL", Nullable: false, IsPrimaryKey: true, Description: "自增主键"},
				{Name: "job_id", Type: "TEXT", Nullable: false, Description: "任务ID"},
				{Name: "attempt", Type: "INT", Nullable: false, Description: "尝试次数"},
				{Name: "status", Type: "TEXT", Nullable: false, Description: "执行状态"},
				{Name: "error", Type: "TEXT", Nullable: true, Description: "错误信息"},
				{Name: "started_at", Type: "TIMESTAMPTZ", Nullable: false, Description: "开始时间"},
				{Name: "completed_at", Type: "TIMESTAMPTZ", Nullable: true, Description: "完成时间"},
				{Name: "duration_ns", Type: "BIGINT", Nullable: true, Description: "耗时(纳秒)"},
			},
			Indexes: []IndexInfo{
				{Name: "ddd_job_execution_log_pkey", Columns: []string{"id"}, Unique: true},
				{Name: "idx_ddd_jel_job", Columns: []string{"job_id"}, Unique: false},
			},
		},
		{
			TableInfo: TableInfo{Name: "ddd_spans", Description: tableDescription("ddd_spans")},
			Columns: []ColumnInfo{
				{Name: "id", Type: "TEXT", Nullable: false, IsPrimaryKey: true, Description: "Span ID"},
				{Name: "trace_id", Type: "TEXT", Nullable: false, Description: "追踪ID"},
				{Name: "parent_id", Type: "TEXT", Nullable: true, Description: "父Span ID"},
				{Name: "type", Type: "TEXT", Nullable: false, Description: "类型"},
				{Name: "name", Type: "TEXT", Nullable: false, Description: "名称"},
				{Name: "status", Type: "TEXT", Nullable: false, Description: "状态"},
				{Name: "error", Type: "TEXT", Nullable: true, Description: "错误信息"},
				{Name: "started_at", Type: "TIMESTAMPTZ", Nullable: false, Description: "开始时间"},
				{Name: "duration_ns", Type: "BIGINT", Nullable: false, Description: "耗时(纳秒)"},
			},
			Indexes: []IndexInfo{
				{Name: "ddd_spans_pkey", Columns: []string{"id"}, Unique: true},
				{Name: "idx_ddd_spans_trace", Columns: []string{"trace_id"}, Unique: false},
			},
		},
		{
			TableInfo: TableInfo{Name: "ddd_aggregate_snapshots", Description: tableDescription("ddd_aggregate_snapshots")},
			Columns: []ColumnInfo{
				{Name: "aggregate_id", Type: "TEXT", Nullable: false, IsPrimaryKey: true, Description: "聚合ID"},
				{Name: "aggregate_type", Type: "TEXT", Nullable: false, Description: "聚合类型"},
				{Name: "snapshot_data", Type: "JSONB", Nullable: false, Description: "快照数据"},
				{Name: "version", Type: "INT", Nullable: false, DefaultValue: "0", Description: "版本号"},
				{Name: "updated_at", Type: "TIMESTAMPTZ", Nullable: false, Description: "更新时间"},
			},
			Indexes: []IndexInfo{
				{Name: "ddd_aggregate_snapshots_pkey", Columns: []string{"aggregate_id"}, Unique: true},
			},
		},
		{
			TableInfo: TableInfo{Name: "ddd_command_log", Description: tableDescription("ddd_command_log")},
			Columns: []ColumnInfo{
				{Name: "id", Type: "BIGSERIAL", Nullable: false, IsPrimaryKey: true, Description: "自增主键"},
				{Name: "trace_id", Type: "TEXT", Nullable: true, Description: "追踪ID"},
				{Name: "span_id", Type: "TEXT", Nullable: true, Description: "Span ID"},
				{Name: "command_type", Type: "TEXT", Nullable: false, Description: "命令类型"},
				{Name: "command_data", Type: "JSONB", Nullable: true, Description: "命令数据"},
				{Name: "result_type", Type: "TEXT", Nullable: true, Description: "结果类型"},
				{Name: "result_data", Type: "JSONB", Nullable: true, Description: "结果数据"},
				{Name: "error", Type: "TEXT", Nullable: true, Description: "错误信息"},
				{Name: "duration_ns", Type: "BIGINT", Nullable: true, Description: "耗时(纳秒)"},
				{Name: "created_at", Type: "TIMESTAMPTZ", Nullable: false, Description: "创建时间"},
			},
			Indexes: []IndexInfo{
				{Name: "ddd_command_log_pkey", Columns: []string{"id"}, Unique: true},
				{Name: "idx_ddd_command_log_type", Columns: []string{"command_type"}, Unique: false},
				{Name: "idx_ddd_command_log_trace", Columns: []string{"trace_id"}, Unique: false},
			},
		},
		{
			TableInfo: TableInfo{Name: "ddd_query_log", Description: tableDescription("ddd_query_log")},
			Columns: []ColumnInfo{
				{Name: "id", Type: "BIGSERIAL", Nullable: false, IsPrimaryKey: true, Description: "自增主键"},
				{Name: "trace_id", Type: "TEXT", Nullable: true, Description: "追踪ID"},
				{Name: "span_id", Type: "TEXT", Nullable: true, Description: "Span ID"},
				{Name: "query_type", Type: "TEXT", Nullable: false, Description: "查询类型"},
				{Name: "query_data", Type: "JSONB", Nullable: false, Description: "查询数据"},
				{Name: "result_type", Type: "TEXT", Nullable: true, Description: "结果类型"},
				{Name: "result_data", Type: "JSONB", Nullable: true, Description: "结果数据"},
				{Name: "error", Type: "TEXT", Nullable: true, Description: "错误信息"},
				{Name: "duration_ns", Type: "BIGINT", Nullable: true, Description: "耗时(纳秒)"},
				{Name: "created_at", Type: "TIMESTAMPTZ", Nullable: false, Description: "创建时间"},
			},
			Indexes: []IndexInfo{
				{Name: "ddd_query_log_pkey", Columns: []string{"id"}, Unique: true},
				{Name: "idx_ddd_query_log_type", Columns: []string{"query_type"}, Unique: false},
				{Name: "idx_ddd_query_log_trace", Columns: []string{"trace_id"}, Unique: false},
			},
		},
		{
			TableInfo: TableInfo{Name: "ddd_event_log", Description: tableDescription("ddd_event_log")},
			Columns: []ColumnInfo{
				{Name: "id", Type: "BIGSERIAL", Nullable: false, IsPrimaryKey: true, Description: "自增主键"},
				{Name: "trace_id", Type: "TEXT", Nullable: true, Description: "追踪ID"},
				{Name: "span_id", Type: "TEXT", Nullable: true, Description: "Span ID"},
				{Name: "aggregate_id", Type: "TEXT", Nullable: false, Description: "聚合ID"},
				{Name: "event_type", Type: "TEXT", Nullable: false, Description: "事件类型"},
				{Name: "event_data", Type: "JSONB", Nullable: false, Description: "事件数据"},
				{Name: "handler_count", Type: "INT", Nullable: true, DefaultValue: "0", Description: "处理器数量"},
				{Name: "error", Type: "TEXT", Nullable: true, Description: "错误信息"},
				{Name: "duration_ns", Type: "BIGINT", Nullable: true, Description: "耗时(纳秒)"},
				{Name: "created_at", Type: "TIMESTAMPTZ", Nullable: false, Description: "创建时间"},
			},
			Indexes: []IndexInfo{
				{Name: "ddd_event_log_pkey", Columns: []string{"id"}, Unique: true},
				{Name: "idx_ddd_event_log_type", Columns: []string{"event_type"}, Unique: false},
				{Name: "idx_ddd_event_log_aggregate", Columns: []string{"aggregate_id"}, Unique: false},
				{Name: "idx_ddd_event_log_trace", Columns: []string{"trace_id"}, Unique: false},
			},
		},
		{
			TableInfo: TableInfo{Name: "ddd_event_handler_log", Description: tableDescription("ddd_event_handler_log")},
			Columns: []ColumnInfo{
				{Name: "id", Type: "BIGSERIAL", Nullable: false, IsPrimaryKey: true, Description: "自增主键"},
				{Name: "event_log_id", Type: "BIGINT", Nullable: true, Description: "事件日志ID"},
				{Name: "trace_id", Type: "TEXT", Nullable: true, Description: "追踪ID"},
				{Name: "span_id", Type: "TEXT", Nullable: true, Description: "Span ID"},
				{Name: "aggregate_id", Type: "TEXT", Nullable: false, Description: "聚合ID"},
				{Name: "event_type", Type: "TEXT", Nullable: false, Description: "事件类型"},
				{Name: "handler_type", Type: "TEXT", Nullable: false, Description: "处理器类型"},
				{Name: "status", Type: "TEXT", Nullable: false, Description: "执行状态"},
				{Name: "error", Type: "TEXT", Nullable: true, Description: "错误信息"},
				{Name: "duration_ns", Type: "BIGINT", Nullable: true, Description: "耗时(纳秒)"},
				{Name: "created_at", Type: "TIMESTAMPTZ", Nullable: false, Description: "创建时间"},
			},
			Indexes: []IndexInfo{
				{Name: "ddd_event_handler_log_pkey", Columns: []string{"id"}, Unique: true},
				{Name: "idx_ddd_ehl_event_log", Columns: []string{"event_log_id"}, Unique: false},
				{Name: "idx_ddd_ehl_handler", Columns: []string{"handler_type"}, Unique: false},
				{Name: "idx_ddd_ehl_status", Columns: []string{"status"}, Unique: false},
			},
		},
	}
}
