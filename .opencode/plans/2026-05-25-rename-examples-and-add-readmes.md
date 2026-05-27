# Rename example/ to examples/ and Add READMEs

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename `example/` directory to `examples/`, update all references, and add README documentation to both example directories.

**Architecture:** Simple rename and search-replace across go.work, go.mod, imports, Makefile, and README.md. Then create two new README files.

**Tech Stack:** Go, Go workspace (go.work), Make

---

### Files to modify

| Action | File |
|--------|------|
| Rename dir | `example/` → `examples/` |
| Modify | `go.work` — `./example` → `./examples` |
| Modify | `example/go.mod` → `examples/go.mod` — module path `github.com/ddd-qce/example` → `github.com/ddd-qce/examples` |
| Modify | `example/main.go` → `examples/main.go` — 5 import paths |
| Modify | `Makefile` — 6 references to `example` |
| Modify | `README.md` — 2 references |
| Create | `examples/README.md` |
| Create | `exampleapp/README.md` |

---

### Task 1: Rename directory and update go.mod

- [ ] **Step 1: Rename the directory**

Run: `git mv example examples`
Expected: directory renamed, staged in git

- [ ] **Step 2: Update the module path in go.mod**

Read `examples/go.mod` and change line 1:
```diff
-module github.com/ddd-qce/example
+module github.com/ddd-qce/examples
```

- [ ] **Step 3: Verify rename and module change**

Run: `ls examples/ && head -1 examples/go.mod`
Expected: lists example subdirectories, shows `module github.com/ddd-qce/examples`

- [ ] **Step 4: Commit**

```bash
git add examples/ && git commit -m "refactor: rename example/ to examples/"
```

---

### Task 2: Update go.work

- [ ] **Step 1: Update go.work**

Read `go.work` and change the reference:
```diff
 use (
 	.
 	./cmd/ddd
-	./example
+	./examples
 	./exampleapp
 	./it
 )
```

- [ ] **Step 2: Verify workspace still works**

Run: `go work sync`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add go.work && git commit -m "refactor: update go.work to reference examples/"
```

---

### Task 3: Update import paths in example source files

- [ ] **Step 1: Update `examples/main.go` imports**

Read `examples/main.go` and change all 5 import paths:
```diff
-	"github.com/ddd-qce/example/command"
-	"github.com/ddd-qce/example/event"
-	"github.com/ddd-qce/example/job"
-	"github.com/ddd-qce/example/query"
-	"github.com/ddd-qce/example/traceexample"
+	"github.com/ddd-qce/examples/command"
+	"github.com/ddd-qce/examples/event"
+	"github.com/ddd-qce/examples/job"
+	"github.com/ddd-qce/examples/query"
+	"github.com/ddd-qce/examples/traceexample"
```

- [ ] **Step 2: Verify no other Go files in examples/ need import updates**

Run: `rg "github.com/ddd-qce/example/" examples/ --type go`
Expected: no matches (only `github.com/ddd-qce/examples/` should appear, if at all — the subdirs like command/, event/ etc. don't import sibling packages)

- [ ] **Step 3: Commit**

```bash
git add examples/main.go && git commit -m "refactor: update import paths to github.com/ddd-qce/examples"
```

---

### Task 4: Update Makefile

- [ ] **Step 1: Update all references in Makefile**

Read `Makefile` and make these replacements:

```diff
-test:
-	go test ./... github.com/ddd-qce/example/... github.com/ddd-qce/exampleapp/...
+test:
+	go test ./... github.com/ddd-qce/examples/... github.com/ddd-qce/exampleapp/...

 test-core:
 	go test ./...

-test-example:
-	go test github.com/ddd-qce/example/...
+test-example:
+	go test github.com/ddd-qce/examples/...

 test-exampleapp:
 	go test github.com/ddd-qce/exampleapp/...
```

```diff
 lint-example:
-	cd example && GOWORK=off $(LINT) run $(LINT_FLAGS) ./...
+	cd examples && GOWORK=off $(LINT) run $(LINT_FLAGS) ./...
```

```diff
 fix-example:
-	cd example && GOWORK=off $(LINT) run --fix ./...
+	cd examples && GOWORK=off $(LINT) run --fix ./...
```

- [ ] **Step 2: Verify Makefile syntax**

Run: `make -n test-example`
Expected: prints `cd examples && GOWORK=off ...` without errors

- [ ] **Step 3: Commit**

```bash
git add Makefile && git commit -m "refactor: update Makefile to reference examples/"
```

---

### Task 5: Update README.md

- [ ] **Step 1: Update directory tree in README.md**

Read `README.md` and change:
```diff
-├── /example                     # 独立示例模块（module github.com/ddd-qce/example）
+├── /examples                    # 组件级演示（module github.com/ddd-qce/examples）
 ├── /exampleapp                  # 示例应用模块（module github.com/ddd-qce/exampleapp）
```

- [ ] **Step 2: Commit**

```bash
git add README.md && git commit -m "docs: update README.md for examples/ rename"
```

---

### Task 6: Create examples/README.md

- [ ] **Step 1: Create README**

Create `examples/README.md`:
```markdown
# Examples

Component-level demonstrations showing how to use each module of the DDD-QCE framework.

## Structure

| Directory | Description |
|-----------|-------------|
| `command/` | Command bus usage — register handlers, dispatch commands |
| `event/` | Event bus usage — publish events, subscribe handlers |
| `query/` | Query bus usage — register handlers, dispatch queries |
| `job/` | Job manager usage — submit, wait, cancel, retry jobs |
| `traceexample/` | Tracing aspect — record and inspect operation traces |
| `integration/` | Integration tests demonstrating cross-component workflows |

## Run

```bash
cd examples
go run main.go
```

This is **not** a full application — it's a collection of API usage examples. For a complete DDD web application, see [exampleapp/](../exampleapp/).
```

- [ ] **Step 2: Commit**

```bash
git add examples/README.md && git commit -m "docs: add examples/README.md"
```

---

### Task 7: Create exampleapp/README.md

- [ ] **Step 1: Create README**

Create `exampleapp/README.md`:
```markdown
# Example App

A complete DDD e-commerce web application built on the DDD-QCE framework.

## Structure

| Directory | Description |
|-----------|-------------|
| `domain/` | Domain models (Order, OrderItem), domain events, business logic |
| `application/` | Command handlers, query handlers, event handlers, repository interfaces |
| `infrastructure/` | Dependency wiring, PostgreSQL and memory store implementations, config, logging, metrics |
| `interfaces/http/` | HTTP server, route handlers, HTML templates |
| `integration/` | Full integration tests (memory + PostgreSQL) |

## Run

```bash
cd exampleapp
go run main.go
```

Starts a web server on http://localhost:8080 with an order management UI.

## Supported Backends

- **Memory** (default): In-memory stores for all data
- **PostgreSQL**: Set `DDD_POSTGRES_URI` environment variable to enable

## Tests

```bash
# Memory only
go test ./...

# Memory + PostgreSQL
DDD_POSTGRES_URI=postgres://... go test ./...
```

For simple component-level examples, see [examples/](../examples/).
```

- [ ] **Step 2: Commit**

```bash
git add exampleapp/README.md && git commit -m "docs: add exampleapp/README.md"
```

---

### Task 8: Final verification

- [ ] **Step 1: Run full build and test**

Run: `make test`
Expected: all tests pass

Run: `cd examples && go build ./...`
Expected: no errors

Run: `cd exampleapp && go build ./...`
Expected: no errors

- [ ] **Step 2: Verify no stale references**

Run: `rg "github.com/ddd-qce/example/" --type go --type make`
Expected: only matches containing `examples/` or `exampleapp/` (no bare `example/`)

---

## Self-Review

1. **Spec coverage:** All requirements covered — directory rename, go.work update, go.mod update, import updates, Makefile update, README.md update, two new README files, final verification.
2. **Placeholder scan:** No TBDs, all code blocks and commands specified.
3. **Type consistency:** N/A — this is a refactoring task with no type changes.
