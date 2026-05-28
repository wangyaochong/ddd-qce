# Testcontainers-go 集成测试实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `it/testutil.OpenTestDB` 改为默认使用 testcontainers-go 自动启动 PostgreSQL 容器，同时保留 `TEST_PG_DSN` 环境变量支持真实 PG 切换，使开发者零配置即可运行集成测试。

**Architecture:** 修改 `it/testutil/db.go`，优先检查 `TEST_PG_DSN` 环境变量（真实 PG），未设置时自动启动 testcontainers PostgreSQL 容器（`sync.Once` 共享，进程退出自动清理）。所有 `it/` 下 8 个测试包和 `cqrs/impl/pg`、`infra/repository/pg` 的 real test 无需任何改动。

**Tech Stack:** Go 1.26, testcontainers-go, testcontainers-go/modules/postgres, pgx/v5

---

## 当前状态

- `it/testutil/db.go` 的 `OpenTestDB(t)` 读取 `TEST_PG_DSN` 或回退到 Unix socket 连接
- 回退 DSN `host=/var/run/postgresql dbname=ddd_qce_test user=$USER sslmode=disable` 依赖本地已安装的 PostgreSQL
- 所有 9 个测试包都通过 `testutil.OpenTestDB(t)` 获取 `*sql.DB`，无需修改

## 目标状态

- 默认：testcontainers 自动启动 `postgres:17-alpine` 容器，零配置
- 切换：设置 `TEST_PG_DSN` 环境变量使用真实 PG（CI 环境更快）
- 同一进程内所有测试共享 1 个容器（`sync.Once`）
- 容器在进程退出时由 testcontainers Ryuk 自动清理

---

## Task 1: 添加 testcontainers-go 依赖

**Files:**
- Modify: `it/go.mod`

- [ ] **Step 1: 在 `it/` 模块中添加 testcontainers 依赖**

Run:
```bash
cd it && go get github.com/testcontainers/testcontainers-go github.com/testcontainers/testcontainers-go/modules/postgres
```

- [ ] **Step 2: 运行 go mod tidy**

Run: `cd it && go mod tidy`

- [ ] **Step 3: 验证 go.mod 已更新**

Run: `cat it/go.mod`
Expected: 包含 `github.com/testcontainers/testcontainers-go` 和 `github.com/testcontainers/testcontainers-go/modules/postgres`

---

## Task 2: 重写 `it/testutil/db.go`

**Files:**
- Modify: `it/testutil/db.go`

- [ ] **Step 1: 重写 `it/testutil/db.go`**

将文件全部内容替换为：

```go
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	corepg "github.com/ddd-qce/core/pg"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	containerOnce sync.Once
	container     *postgres.PostgresContainer
	containerDSN  string
	containerErr  error
)

func startPostgresContainer(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	containerOnce.Do(func() {
		c, err := postgres.Run(ctx,
			"postgres:17-alpine",
			postgres.WithDatabase("ddd_qce_test"),
			postgres.WithUsername("ddd"),
			postgres.WithPassword("ddd"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(30*time.Second),
			),
		)
		if err != nil {
			containerErr = fmt.Errorf("start postgres container: %w", err)
			return
		}
		container = c
		dsn, err := c.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			containerErr = fmt.Errorf("get container connection string: %w", err)
			return
		}
		containerDSN = dsn
	})
	return container, containerDSN, containerErr
}

func OpenTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		_, dsnFromContainer, err := startPostgresContainer(ctx)
		if err != nil {
			t.Fatalf("start postgres container: %v", err)
		}
		dsn = dsnFromContainer
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

- [ ] **Step 2: 验证编译**

Run: `cd it && go build ./testutil/`
Expected: 编译成功

---

## Task 3: 运行集成测试验证

**Files:** 无新增/修改

- [ ] **Step 1: 运行 `it/` 全部集成测试（testcontainers 模式，不设 TEST_PG_DSN）**

确保 Docker 已启动，然后运行：

Run: `cd it && go test ./... -v -count=1`
Expected: 全部 PASS

注意：首次运行会拉取 `postgres:17-alpine` 镜像，耗时较长。后续运行使用本地缓存镜像，启动容器约 3-5 秒。

- [ ] **Step 2: 验证真实 PG 切换（如有本地 PG 可用）**

Run: `TEST_PG_DSN="host=/var/run/postgresql dbname=ddd_qce_test user=$(whoami) sslmode=disable" go test ./it/... -v -count=1`
Expected: 全部 PASS（使用本地 PG，跳过容器启动）

- [ ] **Step 3: 运行核心模块的 real DB 测试**

Run: `RUN_REAL_DB_TESTS=1 go test ./cqrs/impl/pg/... -v -count=1`
Expected: 全部 PASS（使用 testcontainers）

Run: `RUN_REAL_DB_TESTS=1 go test ./infra/repository/pg/... -v -count=1`
Expected: 全部 PASS

---

## Task 4: 运行 lint 验证

**Files:** 无新增/修改

- [ ] **Step 1: 运行 lint**

Run: `cd it && golangci-lint run ./...`
Expected: 无错误

---

## Task 5: Commit

- [ ] **Step 1: 提交**

```bash
git add it/testutil/db.go it/go.mod it/go.sum
git commit -m "feat: integrate testcontainers-go for PostgreSQL integration tests

- Default: auto-start postgres:17-alpine container via testcontainers
- Fallback: set TEST_PG_DSN env var to use real PostgreSQL (CI-friendly)
- Shared container across all tests in same process (sync.Once)
- Zero config: developers can run integration tests without local PG
- All existing test packages unchanged (only testutil/db.go modified)"
```

---

## 使用方式

### 开发者本地（零配置）

```bash
# 确保 Docker 已启动，然后直接运行
cd it && go test ./...
```

### CI 环境（使用预配 PG，更快）

```bash
TEST_PG_DSN="postgres://ddd:ddd@postgres:5432/ddd_qce_test?sslmode=disable" go test ./it/...
```

### 切换回本地 PG（无需 Docker）

```bash
# 需要本地已安装 PostgreSQL 且存在 ddd_qce_test 库
TEST_PG_DSN="host=/var/run/postgresql dbname=ddd_qce_test user=$(whoami) sslmode=disable" go test ./it/...
```

### 运行核心模块的 real DB 测试

```bash
# 以前需要：RUN_REAL_DB_TESTS=1 + 手动配置 PG
# 现在：只需设置 RUN_REAL_DB_TESTS=1，PG 自动由 testcontainers 提供
RUN_REAL_DB_TESTS=1 go test ./cqrs/impl/pg/... ./infra/repository/pg/...
```

---

## 设计决策

| 决策 | 原因 |
|------|------|
| **默认 testcontainers，保留 `TEST_PG_DSN`** | 开发者零配置；CI 可用预配 PG 避免容器开销 |
| **`sync.Once` 共享容器** | 同一进程所有测试复用 1 个容器，避免每个测试启动一个（3-5s/次） |
| **`DropAll` + `Migrate` 每测试** | 保证测试隔离，同一容器的数据库在测试间清空重建 |
| **不显式 Terminate 容器** | testcontainers 的 Ryuk 自动在进程退出时清理 |
| **`postgres:17-alpine`** | 镜像小（~80MB），启动快，与项目 pgx/v5 兼容 |
| **不修改 `exampleapp/` 测试** | `exampleapp` 使用 `DDD_POSTGRES_URI` 独立机制，其 `WireAppWithConfig` 不经过 `testutil`，改到它需要不同的方案（在 `exampleapp/infrastructure/` 层面引入 testcontainers），超出本次范围 |

## 并行测试安全

与之前方案相同：`go test -p > 1` 并行运行不同包时，包 A 的 `DropAll` 可能影响包 B。`CREATE TABLE IF NOT EXISTS` 是安全的，但 `DROP TABLE` 不是。

**当前建议**：`it/` 测试使用 `-p 1` 运行（`go test -p 1 ./it/...`）。未来可引入 `pg_advisory_lock` 或 schema 隔离解决。
