# DDD Schema Viewer 设计文档

## 目标

在 ddd-qce 框架中新增 SchemaViewer 功能，允许引入框架的项目通过一行代码注册路由，即可在浏览器中直观查看 DDD 相关数据库表的结构、统计信息和关联关系。

## 核心设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 包位置 | `observability/` 下新增 | 同属可观测性范畴，复用 Dashboard 模式 |
| `*sql.DB` 传递 | `WithPgDB(db)` 独立注入 | 不改 Backend 结构，与现有 PG 组件模式一致 |
| 模板嵌入 | `embed.FS` | 编译进二进制，零外部依赖，部署可靠 |
| 展示形式 | 服务端渲染 HTML | 随框架包分发，引入项目无需前端工作 |
| Memory 后端 | 静态 schema 定义 | 无真实 DB 时仍可展示表结构，行数显示 N/A |

## 包结构

```
observability/
├── schema_viewer.go          # SchemaViewer struct + 构造器 + RegisterRoutes + HTTP handlers
├── schema_types.go           # TableInfo, ColumnInfo, IndexInfo, TableDetail, TableRelation
├── schema_reader.go          # SchemaReader 接口 + InMemorySchemaReader
├── templates/
│   ├── schema_layout.html    # 自包含布局（CSS + 导航）
│   ├── schema_tables.html    # 表列表页（搜索、排序、统计、关联关系图）
│   └── schema_detail.html    # 单表详情页（列定义 + 索引）
├── pg/
│   └── schema_reader.go      # PgSchemaReader 实现
├── dashboard.go              # (现有，不改)
├── ...
```

## 数据模型

```go
type TableInfo struct {
    Name         string
    Description  string     // 中文说明，如 "事件溯源事件存储"
    RowCount     int64      // PG 实时查，Memory 返回 -1
    LastUpdated  *time.Time // PG: max(created_at)，Memory: nil
    DiskSize     int64      // PG: pg_total_relation_size，Memory: -1
    AvgRowSize   int64      // PG: pg_stat_user_tables，Memory: -1
}

type ColumnInfo struct {
    Name         string
    Type         string
    Nullable     bool
    DefaultValue string
    IsPrimaryKey bool
    Description  string     // 中文说明
}

type IndexInfo struct {
    Name    string
    Columns []string
    Unique  bool
}

type TableDetail struct {
    TableInfo
    Columns []ColumnInfo
    Indexes []IndexInfo
}

type TableRelation struct {
    FromTable  string
    FromColumn string
    ToTable    string
    ToColumn   string
}
```

## SchemaReader 接口

```go
type SchemaReader interface {
    ListTables(ctx context.Context) ([]TableInfo, error)
    GetTable(ctx context.Context, name string) (*TableDetail, error)
    ListRelations(ctx context.Context) ([]TableRelation, error)
}
```

### PgSchemaReader（`observability/pg/schema_reader.go`）

- 存储 `db *sql.DB`，使用 `corepg.GetQuerier(ctx, s.db)` 模式
- `ListTables`:
  - 查询 `information_schema.tables WHERE table_schema='public' AND table_name LIKE 'ddd_%'`
  - 每张表 `SELECT count(*)` 获取行数
  - 查询 `pg_stat_user_tables` 获取 last_autovacuum
  - 查询 `pg_total_relation_size()` 获取磁盘占用
- `GetTable`:
  - 查询 `information_schema.columns` 获取列详情
  - 查询 `pg_indexes` 获取索引信息
  - 查询 `count(*)` 获取行数
- `ListRelations`:
  - 查询 `information_schema.table_constraints + key_column_usage` 获取外键关系
  - 加上框架已知的逻辑关联（如 `ddd_event_handler_log.event_log_id → ddd_event_log.id`）

### InMemorySchemaReader（`observability/schema_reader.go`）

- 将 `pg/migrate.go` 中 9 张表的结构硬编码为 Go 变量
- 包含所有列定义、索引、中文说明
- `RowCount` 返回 `-1`
- `LastUpdated`、`DiskSize`、`AvgRowSize` 返回零值
- `ListRelations` 返回框架硬编码的关联关系

### 静态 Schema 定义来源

从 `pg/migrate.go` 提取 9 张表的完整定义：

| 表名 | 说明 |
|------|------|
| `ddd_domain_events` | 事件溯源事件存储 |
| `ddd_jobs` | 异步任务队列 |
| `ddd_job_execution_log` | 任务执行日志 |
| `ddd_spans` | 分布式追踪 |
| `ddd_aggregate_snapshots` | 聚合快照 |
| `ddd_command_log` | 命令审计日志 |
| `ddd_query_log` | 查询审计日志 |
| `ddd_event_log` | 事件审计日志 |
| `ddd_event_handler_log` | 事件处理器日志 |

## SchemaViewer struct

```go
type SchemaViewer struct {
    reader SchemaReader
    config SchemaViewerConfig
    tmpl   *template.Template
}

type SchemaViewerConfig struct {
    Prefix string  // 默认 "/api/ddd/schema"
}

type SchemaViewerOption func(*SchemaViewer)

func WithSchemaPrefix(prefix string) SchemaViewerOption
func WithPgDB(db *sql.DB) SchemaViewerOption  // 传入 *sql.DB 时用 PgSchemaReader
```

### 构造逻辑

```go
func NewSchemaViewer(opts ...SchemaViewerOption) *SchemaViewer {
    cfg := SchemaViewerConfig{Prefix: "/api/ddd/schema"}
    v := &SchemaViewer{config: cfg}

    for _, opt := range opts {
        opt(v)
    }

    // 如果没有设置 reader，默认用 InMemory
    if v.reader == nil {
        v.reader = NewInMemorySchemaReader()
    }

    // 使用 embed.FS 加载模板
    tmpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"))
    v.tmpl = tmpl

    return v
}
```

`WithPgDB` 选项内部创建 `pg.NewPgSchemaReader(db)` 并设置到 `v.reader`。

### 路由注册

```go
func (v *SchemaViewer) RegisterRoutes(mux *http.ServeMux) {
    p := v.config.Prefix
    mux.HandleFunc("GET "+p+"/", v.handleTableList)
    mux.HandleFunc("GET "+p+"/{table}", v.handleTableDetail)
}
```

## HTTP Handlers

### `handleTableList`

1. 调用 `reader.ListTables(ctx)` 获取表列表
2. 调用 `reader.ListRelations(ctx)` 获取关联关系
3. 渲染 `schema_tables.html` 模板

### `handleTableDetail`

1. 从 URL 路径参数取 `{table}`
2. 调用 `reader.GetTable(ctx, tableName)` 获取详情
3. 如果表名不以 `ddd_` 开头，返回 404
4. 渲染 `schema_detail.html` 模板

## HTML 页面设计

### 模板加载：embed.FS

```go
//go:embed templates/*.html
var templateFS embed.FS
```

### 表列表页功能

1. **搜索过滤**：前端 JS 实时过滤，输入关键词匹配表名/描述
2. **排序**：点击列头按表名（字母序）、行数（数值序）排序
3. **统计信息**：每行显示行数、最近更新时间、磁盘占用
4. **关联关系图**：页面底部用 HTML/CSS 展示表间关联，箭头方向表示依赖

```
┌────────────────────────────────────────────────────────────┐
│  DDD Schema Viewer                                         │
│  [搜索: _______________]                                    │
├────────────────────────────────────────────────────────────┤
│  Table Name               │ Description     │ Rows  │ Size │
│  ddd_domain_events        │ 事件溯源事件存储 │ 1,234 │ 2MB  │
│  ddd_jobs                 │ 异步任务队列    │ 56    │ 56KB │
│  ...                                                       │
├────────────────────────────────────────────────────────────┤
│  Relations                                                 │
│  ddd_event_handler_log.event_log_id → ddd_event_log.id    │
│  ddd_job_execution_log.job_id → ddd_jobs.id               │
│  ...                                                       │
└────────────────────────────────────────────────────────────┘
```

### 单表详情页

```
┌────────────────────────────────────────────────────────────┐
│  ← Back    ddd_domain_events                               │
│  事件溯源事件存储 │ Rows: 1,234 │ Size: 2MB                │
├────────────────────────────────────────────────────────────┤
│  Columns                                                   │
│  Name          │ Type        │ Null │ Default │ PK  │ Desc │
│  id            │ BIGSERIAL   │ NO   │         │ ✓   │ 自增主键│
│  aggregate_id  │ TEXT        │ NO   │         │     │ 聚合ID │
│  ...                                                       │
├────────────────────────────────────────────────────────────┤
│  Indexes                                                   │
│  Name                      │ Columns            │ Unique   │
│  ddd_domain_events_pkey    │ id                 │ ✓        │
│  idx_ddd_events_aggregate  │ aggregate_id,version│ ✓       │
└────────────────────────────────────────────────────────────┘
```

- 自包含 CSS，无外部依赖
- 暗色/亮色自动跟随系统 `prefers-color-scheme`

## 引入项目的方式

### PG 后端

```go
// server.go
mux := http.NewServeMux()
// ... 业务路由 ...

// DDD Schema Viewer（2 行代码）
viewer := observability.NewSchemaViewer(observability.WithPgDB(db))
viewer.RegisterRoutes(mux)

// 访问 /api/ddd/schema/ 即可
```

### Memory 后端

```go
viewer := observability.NewSchemaViewer()
viewer.RegisterRoutes(mux)
```

### 自定义前缀

```go
viewer := observability.NewSchemaViewer(
    observability.WithPgDB(db),
    observability.WithSchemaPrefix("/debug/schema"),
)
viewer.RegisterRoutes(mux)
```

## 不做的事

- 不修改 `infra.Backend` 结构
- 不修改现有 `observability.Dashboard` 代码
- 不提供数据预览功能（仅表结构 + 统计）
- 不提供 JSON API（仅 HTML 页面，后续可按需扩展）
- 不支持非 `ddd_` 前缀的业务表

## 测试策略

1. **单元测试**：`InMemorySchemaReader` 返回正确的 9 张表结构
2. **单元测试**：`SchemaViewer` 构造器选项逻辑
3. **集成测试**：`PgSchemaReader` 对真实 PG 数据库的查询（可放入 `integrationtest/`）
4. **HTTP 测试**：`httptest.NewRecorder` 验证路由注册和页面渲染
