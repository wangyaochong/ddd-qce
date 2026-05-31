# ddd-qce 稳定化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 ddd-qce core 稳定为可被其他项目依赖的 Go 库，通过 `./scripts/tag.sh --push` 发布稳定版

**Architecture:** 分三个阶段：(1) 清理 API 表面，消除歧义；(2) 分离 lint 为独立模块，减少传递依赖；(3) 建立 CI/CD 和版本发布流程

**Tech Stack:** Go 1.26, GitHub Actions, golangci-lint

---

## Phase 1: API 表面清理

### Task 1: 删除重复的 domain/domainevent 包

`domain/event` 和 `domain/domainevent` 是完全相同的包。`domain/event` 被 21 处引用，`domain/domainevent` 仅被 1 处引用（`observability/bus_collector.go`）。直接删除 `domain/domainevent`，不保留废弃兼容。

**Files:**
- Modify: `observability/bus_collector.go`
- Delete: `domain/domainevent/doc.go`
- Delete: `domain/domainevent/event.go`

- [ ] **Step 1: 修改 observability/bus_collector.go 的 import**

将 `observability/bus_collector.go:9` 的：
```go
"github.com/ddd-qce/core/domain/domainevent"
```
改为：
```go
"github.com/ddd-qce/core/domain/event"
```

同时将文件中所有 `domainevent.Event` 替换为 `event.Event`（第 15 行 `BusTypeSampleProvider` 接口）。

- [ ] **Step 2: 删除 domain/domainevent 目录**

```bash
rm -rf domain/domainevent/
```

- [ ] **Step 3: 验证编译通过**

Run: `go build ./...`
Expected: 无错误

- [ ] **Step 4: 运行全部测试**

Run: `go test ./...`
Expected: 全部通过

- [ ] **Step 5: Commit**

```bash
git add observability/bus_collector.go
git add -u domain/domainevent/
git commit -m "refactor: remove duplicate domain/domainevent, use domain/event"
```

---

### Task 2: 验示 exampleapp 不依赖 replace 指令

exampleapp 的 go.mod 使用 `replace github.com/ddd-qce/core => ../`。发布后用户需要通过版本号引用。需要验证 exampleapp 可以通过版本号引用 core。

**Files:**
- Modify: `exampleapp/go.mod` (临时移除 replace 验证)

- [ ] **Step 1: 临时注释 replace 指令验证**

将 `exampleapp/go.mod` 最后一行 `replace github.com/ddd-qce/core => ../` 注释掉，然后运行：
Run: `cd exampleapp && GOWORK=off go build ./...`
Expected: 失败（因为 core 还没发布到 GOPATH/GOPROXY）

这只是验证步骤，确认 replace 是必要的。完成后恢复原样。

- [ ] **Step 2: 恢复 replace 指令**

```bash
git checkout exampleapp/go.mod
```

- [ ] **Step 3: Commit（无实际变更，跳过）**

---

### Task 3: 审计并文档化公共 API 契约

创建 API 稳定性文档，明确哪些包是稳定的公共 API，哪些是内部实现。

**Files:**
- Create: `docs/api-stability.md`

- [ ] **Step 1: 创建 API 稳定性文档**

创建 `docs/api-stability.md`，内容如下：

```markdown
# API Stability

## Stable Public API

These packages are part of the stable public API:

| Package | Purpose |
|---------|---------|
| `app` | Application bootstrap and lifecycle |
| `aspect` | AOP aspect chain |
| `aspect/builtin` | Built-in aspects (logging, tracing, metrics, transaction, persistence) |
| `config` | Configuration loading |
| `cqrs/command` | Command interfaces and dispatch |
| `cqrs/query` | Query interfaces and dispatch |
| `cqrs/event` | Event interfaces and dispatch |
| `domain/aggregate` | AggregateRoot |
| `domain/entity` | Entity, AuditableEntity, SoftDeletableEntity |
| `domain/event` | Event interface |
| `domain/repository` | Repository interfaces |
| `domain/valueobject` | ValueObject |
| `error` | Domain error types |
| `infra` | Backend abstraction |
| `infra/repository` | Repository error types |
| `job/core` | Job interfaces |
| `pg` | PostgreSQL utilities |
| `trace` | Tracing interfaces |

## Stable Implementations

These implementations are stable and can be used directly:

| Package | Purpose |
|---------|---------|
| `cqrs/impl/memory` | In-memory CQRS buses |
| `infra/repository/memory` | In-memory repositories |
| `job/memory` | In-memory job system |
| `trace` | In-memory trace store |

## Experimental

These packages may change in future versions:

| Package | Purpose |
|---------|---------|
| `observability` | Dashboard and schema viewer |
| `lint/*` | DDD static analysis (separate module) |
```

- [ ] **Step 2: Commit**

```bash
git add docs/api-stability.md
git commit -m "docs: add API stability classification"
```

---

## Phase 2: Lint 模块分离

### Task 4: 将 lint 分离为独立 Go 模块

`lint` 包引入了 `golang.org/x/tools` 依赖（约 45 个传递依赖包）。将其分离为独立模块 `github.com/ddd-qce/core/lint`，使用者按需引入。

**Files:**
- Create: `lint/go.mod`
- Modify: `go.mod` (移除 golang.org/x/tools 依赖)
- Modify: `go.work` (添加 lint 模块)
- Modify: `Makefile` (更新 ddd-lint 命令)

- [ ] **Step 1: 创建 lint/go.mod**

```go
module github.com/ddd-qce/core/lint

go 1.26.0

require (
	github.com/ddd-qce/core v0.0.0
	golang.org/x/tools v0.45.0
)

require (
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
)
```

- [ ] **Step 2: 更新 go.work 添加 lint 模块**

将 `go.work` 更新为：
```
go 1.26.0

use (
	.
	./exampleapp
	./examples
	./integrationtest
	./lint
)
```

- [ ] **Step 3: 更新 Makefile 中的 ddd-lint 命令**

将 `Makefile` 中的 ddd-lint 目标更新为使用 `cd lint && go run`：

```makefile
ddd-lint-core:
	cd lint && go run ./cmd/ddd-lint ../...

ddd-lint-example:
	cd lint && go run ./cmd/ddd-lint ../../examples/...

ddd-lint-exampleapp:
	cd lint && go run ./cmd/ddd-lint ../../exampleapp/...

ddd-lint-it:
	cd lint && go run ./cmd/ddd-lint ../../integrationtest/...
```

- [ ] **Step 4: 从 core go.mod 移除 golang.org/x/tools**

从 `go.mod` 中移除 `golang.org/x/tools v0.45.0` 及其间接依赖：
- `golang.org/x/mod`
- `golang.org/x/sync`（如果没有其他包使用）

注意：`golang.org/x/sync` 可能被其他包使用，需要检查。

- [ ] **Step 5: 运行 go mod tidy**

```bash
go mod tidy
cd lint && go mod tidy
```

- [ ] **Step 6: 验证编译**

```bash
go build ./...
cd lint && go build ./...
```

- [ ] **Step 7: 运行测试**

```bash
go test ./...
cd lint && go test ./...
```

- [ ] **Step 8: Commit**

```bash
git add lint/go.mod go.work go.mod go.sum Makefile
git commit -m "refactor: separate lint into independent Go module"
```

---

### Task 5: 更新 exampleapp 和 examples 对 lint 的引用

exampleapp 和 examples 中如果有引用 lint 的地方，需要更新 import 路径。

**Files:**
- Check: `exampleapp/**/*.go`
- Check: `examples/**/*.go`

- [ ] **Step 1: 搜索对 lint 包的引用**

Run: `grep -r "ddd-qce/core/lint" exampleapp/ examples/ integrationtest/`
Expected: 无引用（lint 是独立工具，不被应用代码引用）

- [ ] **Step 2: 如果有引用，更新 import 路径**

如果有引用，将 `github.com/ddd-qce/core/lint` 改为 `github.com/ddd-qce/core/lint`（路径不变，因为是独立模块）。

- [ ] **Step 3: Commit（如有变更）**

---

## Phase 3: CI/CD 和版本发布

### Task 6: 创建 GitHub Actions CI 流水线

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: 创建 CI 配置**

创建 `.github/workflows/ci.yml`：

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: ['1.26.x']
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go-version }}

      - name: Run go vet
        run: go vet ./...

      - name: Run tests
        run: go test ./... -race -count=1

      - name: Run linter
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  test-examples:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.26.x'

      - name: Run example tests
        run: go test github.com/ddd-qce/examples/... -race -count=1
        env:
          GOWORK: "off"

      - name: Run exampleapp tests
        run: go test github.com/ddd-qce/exampleapp/... -race -count=1
        env:
          GOWORK: "off"

  ddd-lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.26.x'

      - name: Run DDD lint
        run: |
          cd lint && go run ./cmd/ddd-lint ../...
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add GitHub Actions CI pipeline"
```

---

### Task 7: 创建 Release 流水线

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: 创建 Release 配置**

创建 `.github/workflows/release.yml`：

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.26.x'

      - name: Run all tests
        run: go test ./... -race -count=1

      - name: Run linter
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

      - name: Run DDD lint
        run: cd lint && go run ./cmd/ddd-lint ../...

  create-release:
    needs: verify
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          generate_release_notes: true
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add release verification pipeline"
```

---

### Task 8: 更新 README 添加安装说明

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 在 README 中添加安装和版本说明**

在 README.md 的 Quick Start 部分之前添加：

```markdown
## Installation

```bash
go get github.com/ddd-qce/core@v20260530.v1
```

### Versioning

This project uses date-based tags: `v{YYYYMMDD}.v{N}`

- `v20260530.v1` — first release on 2026-05-30
- `v20260531.v1` — first release on 2026-05-31

For production use, pin to a specific tag. The `main` branch is always green.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add installation instructions and versioning policy"
```

---

## Phase 4: 发布验证

### Task 9: 端到端验证

**Files:**
- (无文件变更)

- [ ] **Step 1: 运行全量测试**

```bash
make test
```

- [ ] **Step 2: 运行全量 lint**

```bash
make lint
```

- [ ] **Step 3: 运行 DDD lint**

```bash
make ddd-lint
```

- [ ] **Step 4: 验证 lint 模块独立工作**

```bash
cd lint && go test ./... && go build ./...
```

- [ ] **Step 5: 验证 go mod tidy 无变更**

```bash
go mod tidy
git diff go.mod go.sum
# 应该无变更
```

- [ ] **Step 6: 打 tag 并发布**

```bash
./scripts/tag.sh --push
```

---

## 执行顺序

```
Phase 1 (API 清理)        Phase 2 (Lint 分离)       Phase 3 (CI/CD)         Phase 4 (验证)
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│ Task 1: 合并     │      │ Task 4: 分离     │      │ Task 6: CI       │      │ Task 9: 端到端   │
│ Event 接口       │─────▶│ lint 模块        │─────▶│ 流水线           │─────▶│ 验证             │
│                 │      │                 │      │                 │      │                 │
│ Task 2: 验证     │      │ Task 5: 更新     │      │ Task 7: Release  │      │                 │
│ replace 指令     │      │ 引用             │      │ 流水线           │      │                 │
│                 │      │                 │      │                 │      │                 │
│ Task 3: API     │      │                 │      │ Task 8: README  │      │                 │
│ 稳定性文档       │      │                 │      │ 安装说明         │      │                 │
└─────────────────┘      └─────────────────┘      └─────────────────┘      └─────────────────┘
```

每个 Phase 完成后应运行测试确认无回归。Phase 2（lint 分离）是风险最高的步骤，需要仔细验证。

---

## 风险和缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| lint 模块分离后 import 路径变化 | 低（lint 是工具，不被应用代码引用） | Task 5 搜索确认无引用 |
| golang.org/x/sync 被其他包使用 | 中（移除后编译失败） | Task 4 Step 4 检查依赖 |
| CI 流水线配置错误 | 中（阻塞发布） | 先在 PR 中验证 |
