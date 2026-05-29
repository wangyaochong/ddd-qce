# 统一测试数据库 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将所有测试数据库统一为 `ddd_qce_test`，消除 8 个碎片化的测试库

**Architecture:** 修改 `it/testutil.OpenTestDB` 移除 dbname 参数、固定使用 `ddd_qce_test`；将 `cqrs/impl/pg` 和 `infra/repository/pg` 下的独立 `openTestDB` 函数统一到 `testutil.OpenTestDB`；`exampleapp` 的 PostgreSQL 测试通过 `DDD_POSTGRES_URI` 指向 `ddd_qce_test` 库。利用已有的 `Migrate`(CREATE TABLE IF NOT EXISTS) + `DropAll`(DROP TABLE IF EXISTS CASCADE) 机制管理表生命周期。

**Tech Stack:** Go, PostgreSQL, pgx/v5

---

## 现状：8 个碎片化测试库

| 包路径 | 数据库名 | 入口 |
|---|---|---|
| `it/cqrs_event_pg` | `ddd_qce_event_test` | `testutil.OpenTestDB` |
| `it/infra_pg` (repository) | `ddd_qce_repo_test` | `testutil.OpenTestDB` |
| `it/infra_pg` (backend) | `ddd_qce_backend_test` | `testutil.OpenTestDB` |
| `it/job_pg` (manager) | `ddd_qce_job_mgr_test` | `testutil.OpenTestDB` |
| `it/job_pg` (store) | `ddd_qce_job_test` | `testutil.OpenTestDB` |
| `it/trace_pg` | `ddd_qce_trace_test` | `testutil.OpenTestDB` |
| `it/aspect_builtin_pg` | `ddd_qce_aspect_test` | `testutil.OpenTestDB` |
| `it/pg` (migrate) | `ddd_qce_test` | `testutil.OpenTestDB` |
| `it/pg` (transaction) | `ddd_qce_tx_test` | `testutil.OpenTestDB` |
| `cqrs/impl/pg` | `test_event_store` | 独立 `openTestDB` |
| `infra/repository/pg` | `test_repo` | 独立 `openTestDB` |
| `exampleapp` | `postgres` 或 `DDD_POSTGRES_URI` | `testDSNFromEnv` |

---

## Task 1: 修改 `testutil.OpenTestDB` 移除 dbname 参数

**Files:**
- Modify: `it/testutil/db.go`

当前签名：
```go
func OpenTestDB(t *testing.T, dbname string) *sql.DB
```

改为：
```go
func OpenTestDB(t *testing.T) *sql.DB
```

DSN 中 `dbname` 固定使用 `ddd_qce_test`。

- [ ] **Step 1: 修改 `it/testutil/db.go`**

```go
package testutil

import (
	"database/sql"
	"os"
	"testing"

	corepg "github.com/ddd-qce/core/pg"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func OpenTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=/var/run/postgresql dbname=ddd_qce_test user=" + os.Getenv("USER") + " sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db failed: %v", err)
	}
	t.Cleanup(func() {
		corepg.DropAll(db)
		db.Close()
	})
	if err := corepg.Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return db
}
```

- [ ] **Step 2: 验证编译失败（所有调用方传了 dbname 参数）**

Run: `go build ./it/testutil/...`
Expected: 编译成功（testutil 本身没问题）

Run: `go build ./it/...`
Expected: 编译错误，所有 `OpenTestDB(t, "xxx")` 调用参数过多

---

## Task 2: 更新 `it/` 下所有 `OpenTestDB` 调用方

**Files:**
- Modify: `it/cqrs_event_pg/event_store_test.go`
- Modify: `it/infra_pg/repository_test.go`
- Modify: `it/infra_pg/backend_test.go`
- Modify: `it/job_pg/job_manager_test.go`
- Modify: `it/job_pg/job_store_test.go`
- Modify: `it/trace_pg/trace_store_test.go`
- Modify: `it/aspect_builtin_pg/message_store_test.go`
- Modify: `it/pg/migrate_test.go`
- Modify: `it/pg/transaction_test.go`

所有 `OpenTestDB(t, "xxx")` → `OpenTestDB(t)`，并删除各文件中包装 `OpenTestDB` 的本地 helper 函数（如 `openTestDBForEventStore`、`openTestDBForTx` 等），直接调用 `testutil.OpenTestDB(t)`。

- [ ] **Step 1: 修改 `it/cqrs_event_pg/event_store_test.go`**

删除 `openTestDBForEventStore` 函数：
```go
func openTestDBForEventStore(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_event_test")
}
```

将所有 `openTestDBForEventStore(t)` 替换为 `testutil.OpenTestDB(t)`。

- [ ] **Step 2: 修改 `it/infra_pg/repository_test.go`**

将所有 `testutil.OpenTestDB(t, "ddd_qce_repo_test")` 替换为 `testutil.OpenTestDB(t)`。

- [ ] **Step 3: 修改 `it/infra_pg/backend_test.go`**

将 `testutil.OpenTestDB(t, "ddd_qce_backend_test")` 替换为 `testutil.OpenTestDB(t)`。

- [ ] **Step 4: 修改 `it/job_pg/job_manager_test.go`**

删除 `openTestDBForJobManager` 函数：
```go
func openTestDBForJobManager(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_job_mgr_test")
}
```

将所有 `openTestDBForJobManager(t)` 替换为 `testutil.OpenTestDB(t)`。

- [ ] **Step 5: 修改 `it/job_pg/job_store_test.go`**

删除 `openTestDBForJob` 函数：
```go
func openTestDBForJob(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_job_test")
}
```

将所有 `openTestDBForJob(t)` 替换为 `testutil.OpenTestDB(t)`。

- [ ] **Step 6: 修改 `it/trace_pg/trace_store_test.go`**

删除 `openTestDBForTrace` 函数：
```go
func openTestDBForTrace(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_trace_test")
}
```

将所有 `openTestDBForTrace(t)` 替换为 `testutil.OpenTestDB(t)`。

- [ ] **Step 7: 修改 `it/aspect_builtin_pg/message_store_test.go`**

删除 `openTestDB` 函数：
```go
func openTestDB(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_aspect_test")
}
```

将所有 `openTestDB(t)` 替换为 `testutil.OpenTestDB(t)`。

- [ ] **Step 8: 修改 `it/pg/migrate_test.go`**

删除 `openTestDB` 函数：
```go
func openTestDB(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_test")
}
```

将所有 `openTestDB(t)` 替换为 `testutil.OpenTestDB(t)`。

- [ ] **Step 9: 修改 `it/pg/transaction_test.go`**

删除 `openTestDBForTx` 函数：
```go
func openTestDBForTx(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_tx_test")
}
```

将所有 `openTestDBForTx(t)` 替换为 `testutil.OpenTestDB(t)`。

- [ ] **Step 10: 验证编译**

Run: `go build ./it/...`
Expected: 编译成功，无错误

---

## Task 3: 统一 `cqrs/impl/pg/event_store_real_test.go` 到 `testutil.OpenTestDB`

**Files:**
- Modify: `cqrs/impl/pg/event_store_real_test.go`

当前该文件有独立的 `openTestDB` 函数，连接 `test_event_store` 库，手动创建/删除 `ddd_domain_events` 表。需要改为使用 `testutil.OpenTestDB`，它已经会自动 `Migrate` + `DropAll`。

- [ ] **Step 1: 修改 `cqrs/impl/pg/event_store_real_test.go`**

1. 删除独立的 `openTestDB` 函数
2. 添加 `"github.com/ddd-qce/it/testutil"` import
3. 删除 `"database/sql"` import（不再直接使用）
4. 将所有 `openTestDB(t)` 替换为 `testutil.OpenTestDB(t)`
5. 删除每个测试中手动的 `CREATE TABLE IF NOT EXISTS ddd_domain_events` 语句
6. 删除每个测试中的 `defer db.Exec("DROP TABLE IF EXISTS ddd_domain_events")` 语句
7. 删除每个测试中的 `defer db.Close()`（`testutil.OpenTestDB` 的 `Cleanup` 已处理）

修改后每个测试函数的骨架变为：
```go
func TestEventSourceStore_RealDB_AppendAndLoad(t *testing.T) {
	if os.Getenv("RUN_REAL_DB_TESTS") != "1" {
		t.Skip("Set RUN_REAL_DB_TESTS=1 to run real DB tests")
	}

	db := testutil.OpenTestDB(t)
	store, err := NewEventSourceStore[*testPgEvent](db)
	if err != nil {
		t.Fatalf("NewEventSourceStore failed: %v", err)
	}

	ctx := context.Background()
	// ... 测试逻辑不变 ...
}
```

注意 `TestEventSourceStore_RealDB_LoadAll` 中原有一行 `TRUNCATE ddd_domain_events;` 需要删除，因为 `OpenTestDB` 每次都会 `DropAll` + `Migrate`，表已是空的。

- [ ] **Step 2: 验证编译**

Run: `go build ./cqrs/impl/pg/...`
Expected: 编译成功

---

## Task 4: 统一 `infra/repository/pg/repository_real_test.go` 到 `testutil.OpenTestDB`

**Files:**
- Modify: `infra/repository/pg/repository_real_test.go`

- [ ] **Step 1: 修改 `infra/repository/pg/repository_real_test.go`**

1. 删除独立的 `openTestDBForRepo` 函数
2. 添加 `"github.com/ddd-qce/it/testutil"` import
3. 删除 `"database/sql"` import（不再直接使用）
4. 将 `openTestDBForRepo(t)` 替换为 `testutil.OpenTestDB(t)`
5. 删除 `defer db.Close()`（`testutil.OpenTestDB` 的 `Cleanup` 已处理）

修改后的文件：

```go
package pg

import (
	"os"
	"testing"

	corepg "github.com/ddd-qce/core/pg"
	"github.com/ddd-qce/it/testutil"
)

func TestPgRepository_RealDB_MigrateAndDropAll(t *testing.T) {
	if os.Getenv("RUN_REAL_DB_TESTS") != "1" {
		t.Skip("Set RUN_REAL_DB_TESTS=1 to run real DB tests")
	}

	db := testutil.OpenTestDB(t)

	err := corepg.Migrate(db)
	if err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	err = corepg.DropAll(db)
	if err != nil {
		t.Fatalf("DropAll failed: %v", err)
	}
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./infra/repository/pg/...`
Expected: 编译成功

---

## Task 5: 更新 `exampleapp` 的 PostgreSQL 测试默认库

**Files:**
- Modify: `exampleapp/infrastructure/provider_contract_test.go`

`exampleapp` 的测试通过 `DDD_POSTGRES_URI` 环境变量配置数据库，默认回退到 `postgres` 库。修改默认回退为 `ddd_qce_test`。

- [ ] **Step 1: 修改 `exampleapp/infrastructure/provider_contract_test.go` 中的 `testDSNFromEnv` 函数**

将：
```go
defaultDSN := "host=/var/run/postgresql dbname=postgres user=" + os.Getenv("USER") + " sslmode=disable"
```

改为：
```go
defaultDSN := "host=/var/run/postgresql dbname=ddd_qce_test user=" + os.Getenv("USER") + " sslmode=disable"
```

- [ ] **Step 2: 验证编译**

Run: `go build ./exampleapp/...`
Expected: 编译成功

---

## Task 6: 全量编译验证与测试

**Files:** 无新增/修改

- [ ] **Step 1: 全量编译**

Run: `go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 2: 运行不依赖数据库的单元测试**

Run: `go test -short ./...`
Expected: 全部 PASS

- [ ] **Step 3: 运行 `it/` 下依赖数据库的集成测试（需要本地 PostgreSQL 且存在 `ddd_qce_test` 库）**

Run: `go test ./it/...`
Expected: 全部 PASS

- [ ] **Step 4: 清理旧测试数据库（手动，可选）**

删除以下不再使用的测试数据库：
```sql
DROP DATABASE IF EXISTS ddd_qce_event_test;
DROP DATABASE IF EXISTS ddd_qce_repo_test;
DROP DATABASE IF EXISTS ddd_qce_backend_test;
DROP DATABASE IF EXISTS ddd_qce_job_mgr_test;
DROP DATABASE IF EXISTS ddd_qce_job_test;
DROP DATABASE IF EXISTS ddd_qce_trace_test;
DROP DATABASE IF EXISTS ddd_qce_aspect_test;
DROP DATABASE IF EXISTS ddd_qce_tx_test;
DROP DATABASE IF EXISTS test_event_store;
DROP DATABASE IF EXISTS test_repo;
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: unify all test databases to ddd_qce_test

- Remove dbname parameter from testutil.OpenTestDB, fixed to ddd_qce_test
- Delete 10 per-package test DB helpers (openTestDBForEventStore, etc.)
- Unify cqrs/impl/pg and infra/repository/pg real DB tests to testutil.OpenTestDB
- Remove manual CREATE TABLE / DROP TABLE in real tests (handled by Migrate/DropAll)
- Update exampleapp default test DB from 'postgres' to 'ddd_qce_test'
- Consolidate from 11 separate test databases to 1"
```

---

## 并行测试安全说明

当前方案中，`go test -p > 1` 并行运行不同包的测试时，可能发生以下竞争：

1. 包 A 的 `OpenTestDB` 执行 `Migrate` → 包 B 的 `OpenTestDB` 也执行 `Migrate`（`CREATE TABLE IF NOT EXISTS`，安全）
2. 包 A 的 `Cleanup` 调用 `DropAll` → 包 B 的测试仍在使用表（会失败）

**现有行为也是同样的**：每个包用独立库名但同一包内多个测试共享该库，同包并行测试已有此问题。

**建议**：如果未来需要 `-p > 1` 并行，可引入 `pg_advisory_lock` 在 `OpenTestDB` 中加锁，或使用 schema 隔离（每个测试用随机 schema）。当前阶段不作为本次改动范围。
