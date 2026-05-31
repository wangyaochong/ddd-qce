package pg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ddd-qce/core/observability"
	corepg "github.com/ddd-qce/core/pg"
)

type PgSchemaReader struct {
	db *sql.DB
}

func NewSchemaReader(db *sql.DB) *PgSchemaReader {
	return &PgSchemaReader{db: db}
}

var _ observability.SchemaReader = (*PgSchemaReader)(nil)

func (r *PgSchemaReader) ListTables(ctx context.Context) ([]observability.TableInfo, error) {
	q := corepg.GetQuerier(ctx, r.db)

	rows, err := q.QueryContext(ctx,
		`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name LIKE 'ddd_%' ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]observability.TableInfo, 0, len(tableNames))
	for _, name := range tableNames {
		info := observability.TableInfo{
			Name:        name,
			Description: tableDescription(name),
		}

		var count int64
		if err := q.QueryRowContext(ctx, "SELECT count(*) FROM "+name).Scan(&count); err == nil {
			info.RowCount = count
		}

		var diskSize int64
		if err := q.QueryRowContext(ctx, "SELECT pg_total_relation_size($1)", name).Scan(&diskSize); err == nil {
			info.DiskSize = diskSize
		}

		var lastUpdated sql.NullTime
		if hasColumn(ctx, q, name, "created_at") {
			if err := q.QueryRowContext(ctx, "SELECT max(created_at) FROM "+name).Scan(&lastUpdated); err == nil && lastUpdated.Valid {
				info.LastUpdated = &lastUpdated.Time
			}
		}

		result = append(result, info)
	}

	return result, nil
}

func (r *PgSchemaReader) GetTable(ctx context.Context, name string) (*observability.TableDetail, error) {
	if !strings.HasPrefix(name, "ddd_") {
		return nil, fmt.Errorf("table %q not found", name)
	}

	q := corepg.GetQuerier(ctx, r.db)

	detail := &observability.TableDetail{
		TableInfo: observability.TableInfo{
			Name:        name,
			Description: tableDescription(name),
		},
	}

	var count int64
	if err := q.QueryRowContext(ctx, "SELECT count(*) FROM "+name).Scan(&count); err == nil {
		detail.RowCount = count
	}

	var diskSize int64
	if err := q.QueryRowContext(ctx, "SELECT pg_total_relation_size($1)", name).Scan(&diskSize); err == nil {
		detail.DiskSize = diskSize
	}

	var lastUpdated sql.NullTime
	if hasColumn(ctx, q, name, "created_at") {
		if err := q.QueryRowContext(ctx, "SELECT max(created_at) FROM "+name).Scan(&lastUpdated); err == nil && lastUpdated.Valid {
			detail.LastUpdated = &lastUpdated.Time
		}
	}

	colRows, err := q.QueryContext(ctx,
		`SELECT column_name, data_type, is_nullable, column_default
		 FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = $1
		 ORDER BY ordinal_position`, name)
	if err != nil {
		return nil, fmt.Errorf("query columns: %w", err)
	}
	defer colRows.Close()

	for colRows.Next() {
		var colName, dataType, isNullable string
		var colDefault sql.NullString
		if err := colRows.Scan(&colName, &dataType, &isNullable, &colDefault); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		detail.Columns = append(detail.Columns, observability.ColumnInfo{
			Name:         colName,
			Type:         dataType,
			Nullable:     isNullable == "YES",
			DefaultValue: colDefault.String,
			Description:  columnDescription(name, colName),
		})
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	pkCols, err := getPrimaryKeyColumns(ctx, q, name)
	if err == nil {
		pkSet := make(map[string]bool, len(pkCols))
		for _, c := range pkCols {
			pkSet[c] = true
		}
		for i := range detail.Columns {
			if pkSet[detail.Columns[i].Name] {
				detail.Columns[i].IsPrimaryKey = true
			}
		}
	}

	idxRows, err := q.QueryContext(ctx,
		`SELECT indexname, indexdef FROM pg_indexes WHERE schemaname = 'public' AND tablename = $1 ORDER BY indexname`, name)
	if err != nil {
		return nil, fmt.Errorf("query indexes: %w", err)
	}
	defer idxRows.Close()

	for idxRows.Next() {
		var idxName, idxDef string
		if err := idxRows.Scan(&idxName, &idxDef); err != nil {
			return nil, fmt.Errorf("scan index: %w", err)
		}
		cols := parseIndexColumns(idxDef)
		isUnique := strings.Contains(strings.ToUpper(idxDef), "UNIQUE")
		detail.Indexes = append(detail.Indexes, observability.IndexInfo{
			Name:    idxName,
			Columns: cols,
			Unique:  isUnique,
		})
	}
	if err := idxRows.Err(); err != nil {
		return nil, err
	}

	return detail, nil
}

func (r *PgSchemaReader) ListRelations(ctx context.Context) ([]observability.TableRelation, error) {
	q := corepg.GetQuerier(ctx, r.db)

	rows, err := q.QueryContext(ctx,
		`SELECT
			kcu.table_name AS from_table,
			kcu.column_name AS from_column,
			ccu.table_name AS to_table,
			ccu.column_name AS to_column
		 FROM information_schema.table_constraints tc
		 JOIN information_schema.key_column_usage kcu
		   ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		 JOIN information_schema.constraint_column_usage ccu
		   ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
		 WHERE tc.constraint_type = 'FOREIGN KEY'
		   AND tc.table_schema = 'public'
		   AND kcu.table_name LIKE 'ddd_%'`)
	if err != nil {
		return nil, fmt.Errorf("query relations: %w", err)
	}
	defer rows.Close()

	var result []observability.TableRelation
	for rows.Next() {
		var rel observability.TableRelation
		if err := rows.Scan(&rel.FromTable, &rel.FromColumn, &rel.ToTable, &rel.ToColumn); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		result = append(result, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result = append(result, staticRelations()...)
	return result, nil
}

func getPrimaryKeyColumns(ctx context.Context, q corepg.DBTX, tableName string) ([]string, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT kcu.column_name
		 FROM information_schema.table_constraints tc
		 JOIN information_schema.key_column_usage kcu
		   ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		 WHERE tc.constraint_type = 'PRIMARY KEY'
		   AND tc.table_schema = 'public'
		   AND tc.table_name = $1
		 ORDER BY kcu.ordinal_position`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

func hasColumn(ctx context.Context, q corepg.DBTX, tableName, columnName string) bool {
	var count int
	err := q.QueryRowContext(ctx,
		`SELECT count(*) FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`,
		tableName, columnName).Scan(&count)
	return err == nil && count > 0
}

func parseIndexColumns(indexDef string) []string {
	idx := strings.Index(indexDef, "(")
	if idx < 0 {
		return nil
	}
	rest := indexDef[idx+1:]
	endIdx := strings.Index(rest, ")")
	if endIdx < 0 {
		return nil
	}
	inner := rest[:endIdx]
	parts := strings.Split(inner, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		col := strings.TrimSpace(p)
		if spaceIdx := strings.Index(col, " "); spaceIdx > 0 {
			col = col[:spaceIdx]
		}
		if col != "" {
			result = append(result, col)
		}
	}
	return result
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

func columnDescription(tableName, columnName string) string {
	type colKey struct{ table, column string }
	descs := map[colKey]string{
		{"ddd_domain_events", "id"}:                   "自增主键",
		{"ddd_domain_events", "aggregate_id"}:         "聚合ID",
		{"ddd_domain_events", "event_type"}:           "事件类型",
		{"ddd_domain_events", "event_data"}:           "事件数据",
		{"ddd_domain_events", "occurred_at"}:          "发生时间",
		{"ddd_domain_events", "version"}:              "版本号",
		{"ddd_domain_events", "correlation_id"}:       "关联ID",
		{"ddd_domain_events", "causation_id"}:         "因果ID",
		{"ddd_jobs", "id"}:                            "任务ID",
		{"ddd_jobs", "command"}:                       "命令数据",
		{"ddd_jobs", "command_type"}:                  "命令类型",
		{"ddd_jobs", "status"}:                        "任务状态",
		{"ddd_jobs", "result"}:                        "执行结果",
		{"ddd_jobs", "result_type"}:                   "结果类型",
		{"ddd_jobs", "error"}:                         "错误信息",
		{"ddd_jobs", "created_at"}:                    "创建时间",
		{"ddd_jobs", "started_at"}:                    "开始时间",
		{"ddd_jobs", "completed_at"}:                  "完成时间",
		{"ddd_jobs", "timeout_ns"}:                    "超时(纳秒)",
		{"ddd_jobs", "retry_count"}:                   "重试次数",
		{"ddd_jobs", "max_retries"}:                   "最大重试次数",
		{"ddd_job_execution_log", "id"}:               "自增主键",
		{"ddd_job_execution_log", "job_id"}:           "任务ID",
		{"ddd_job_execution_log", "attempt"}:          "尝试次数",
		{"ddd_job_execution_log", "status"}:           "执行状态",
		{"ddd_job_execution_log", "error"}:            "错误信息",
		{"ddd_job_execution_log", "started_at"}:       "开始时间",
		{"ddd_job_execution_log", "completed_at"}:     "完成时间",
		{"ddd_job_execution_log", "duration_ns"}:      "耗时(纳秒)",
		{"ddd_spans", "id"}:                           "Span ID",
		{"ddd_spans", "trace_id"}:                     "追踪ID",
		{"ddd_spans", "parent_id"}:                    "父Span ID",
		{"ddd_spans", "type"}:                         "类型",
		{"ddd_spans", "name"}:                         "名称",
		{"ddd_spans", "status"}:                       "状态",
		{"ddd_spans", "error"}:                        "错误信息",
		{"ddd_spans", "started_at"}:                   "开始时间",
		{"ddd_spans", "duration_ns"}:                  "耗时(纳秒)",
		{"ddd_aggregate_snapshots", "aggregate_id"}:   "聚合ID",
		{"ddd_aggregate_snapshots", "aggregate_type"}: "聚合类型",
		{"ddd_aggregate_snapshots", "snapshot_data"}:  "快照数据",
		{"ddd_aggregate_snapshots", "version"}:        "版本号",
		{"ddd_aggregate_snapshots", "updated_at"}:     "更新时间",
		{"ddd_command_log", "id"}:                     "自增主键",
		{"ddd_command_log", "trace_id"}:               "追踪ID",
		{"ddd_command_log", "span_id"}:                "Span ID",
		{"ddd_command_log", "command_type"}:           "命令类型",
		{"ddd_command_log", "command_data"}:           "命令数据",
		{"ddd_command_log", "result_type"}:            "结果类型",
		{"ddd_command_log", "result_data"}:            "结果数据",
		{"ddd_command_log", "error"}:                  "错误信息",
		{"ddd_command_log", "duration_ns"}:            "耗时(纳秒)",
		{"ddd_command_log", "created_at"}:             "创建时间",
		{"ddd_query_log", "id"}:                       "自增主键",
		{"ddd_query_log", "trace_id"}:                 "追踪ID",
		{"ddd_query_log", "span_id"}:                  "Span ID",
		{"ddd_query_log", "query_type"}:               "查询类型",
		{"ddd_query_log", "query_data"}:               "查询数据",
		{"ddd_query_log", "result_type"}:              "结果类型",
		{"ddd_query_log", "result_data"}:              "结果数据",
		{"ddd_query_log", "error"}:                    "错误信息",
		{"ddd_query_log", "duration_ns"}:              "耗时(纳秒)",
		{"ddd_query_log", "created_at"}:               "创建时间",
		{"ddd_event_log", "id"}:                       "自增主键",
		{"ddd_event_log", "trace_id"}:                 "追踪ID",
		{"ddd_event_log", "span_id"}:                  "Span ID",
		{"ddd_event_log", "aggregate_id"}:             "聚合ID",
		{"ddd_event_log", "event_type"}:               "事件类型",
		{"ddd_event_log", "event_data"}:               "事件数据",
		{"ddd_event_log", "handler_count"}:            "处理器数量",
		{"ddd_event_log", "error"}:                    "错误信息",
		{"ddd_event_log", "duration_ns"}:              "耗时(纳秒)",
		{"ddd_event_log", "created_at"}:               "创建时间",
		{"ddd_event_handler_log", "id"}:               "自增主键",
		{"ddd_event_handler_log", "event_log_id"}:     "事件日志ID",
		{"ddd_event_handler_log", "trace_id"}:         "追踪ID",
		{"ddd_event_handler_log", "span_id"}:          "Span ID",
		{"ddd_event_handler_log", "aggregate_id"}:     "聚合ID",
		{"ddd_event_handler_log", "event_type"}:       "事件类型",
		{"ddd_event_handler_log", "handler_type"}:     "处理器类型",
		{"ddd_event_handler_log", "status"}:           "执行状态",
		{"ddd_event_handler_log", "error"}:            "错误信息",
		{"ddd_event_handler_log", "duration_ns"}:      "耗时(纳秒)",
		{"ddd_event_handler_log", "created_at"}:       "创建时间",
	}
	return descs[colKey{tableName, columnName}]
}

func staticRelations() []observability.TableRelation {
	return []observability.TableRelation{
		{FromTable: "ddd_job_execution_log", FromColumn: "job_id", ToTable: "ddd_jobs", ToColumn: "id"},
		{FromTable: "ddd_event_handler_log", FromColumn: "event_log_id", ToTable: "ddd_event_log", ToColumn: "id"},
	}
}
