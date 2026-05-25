# Getter Naming Rename: `GetID()` → `ID()`, `GetAggregateRoot()` → `AggregateRoot()`

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename `GetID()` → `ID()` and `GetAggregateRoot()` → `AggregateRoot()` to comply with Effective Go's getter naming convention.

**Architecture:** Pure mechanical rename refactoring. Change definitions first (entity.go, aggregate.go), then update all callers via `replaceAll` per file, then update code generator templates and documentation. Compile and test after each major group.

**Tech Stack:** Go 1.26, no `gorename` available — manual `replaceAll` edits with compile verification.

**Scope:** ~147 references across 25+ files. Out of scope: `job/core/job.go` has similar `GetX` patterns but is a separate subsystem.

---

## Impact Summary

| Rename | Definition | Interface | Callers (prod) | Callers (test) | Generator templates | Docs |
|--------|-----------|-----------|-----------------|-----------------|---------------------|------|
| `GetID()` → `ID()` | `entity/entity.go:16` | — | 32 | 58 | 9 | ~4 |
| `GetAggregateRoot()` → `AggregateRoot()` | `aggregate/aggregate.go:53` | `AggregateRef` in `aggregate/aggregate.go:15` | 11 | 4 | 0 | 0 |

---

### Task 1: Rename `Entity.GetID()` → `Entity.ID()` (definition)

**Files:**
- Modify: `domain/entity/entity.go:16`

- [ ] **Step 1: Rename the method definition**

Change line 16 from:
```go
func (e *Entity) GetID() string {
```
to:
```go
func (e *Entity) ID() string {
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./domain/entity/`
Expected: PASS (definition file compiles alone)

Run: `go build ./...`
Expected: FAIL — callers still reference `GetID()`

---

### Task 2: Rename `AggregateRoot.GetAggregateRoot()` → `AggregateRoot.AggregateRoot()` (definition + interface)

**Files:**
- Modify: `domain/aggregate/aggregate.go:15,53,61`

- [ ] **Step 1: Rename the interface method**

Change line 15 from:
```go
GetAggregateRoot() *AggregateRoot
```
to:
```go
AggregateRoot() *AggregateRoot
```

- [ ] **Step 2: Rename the concrete method**

Change line 53 from:
```go
func (a *AggregateRoot) GetAggregateRoot() *AggregateRoot {
```
to:
```go
func (a *AggregateRoot) AggregateRoot() *AggregateRoot {
```

- [ ] **Step 3: Update internal reference in `Equals`**

Change line 61 from:
```go
return a.GetID() == other.GetID()
```
to:
```go
return a.ID() == other.ID()
```

- [ ] **Step 4: Verify definition compiles**

Run: `go build ./domain/aggregate/`
Expected: PASS

---

### Task 3: Update all `.GetID()` → `.ID()` callers in production code

**Files (use `replaceAll` for `.GetID()` → `.ID()` in each file):**
- `exampleapp/domain/model.go` (8 occurrences)
- `exampleapp/application/command_handlers.go` (3 occurrences)
- `exampleapp/application/repositories.go` (2 occurrences)
- `exampleapp/application/query_handlers.go` (2 occurrences)
- `infra/repository/pg/repository.go` (5 occurrences)
- `infra/repository/memory/repository.go` (4 occurrences)

- [ ] **Step 1: Replace all `.GetID()` with `.ID()` in each file**

For each file listed above, use edit tool with `replaceAll: true`:
- oldString: `.GetID()`
- newString: `.ID()`

- [ ] **Step 2: Verify production code compiles**

Run: `go build ./...`
Expected: FAIL only on test files and generator (not yet updated)

---

### Task 4: Update all `.GetAggregateRoot()` → `.AggregateRoot()` callers in production code

**Files (use `replaceAll` for `.GetAggregateRoot()` → `.AggregateRoot()` in each file):**
- `infra/repository/pg/repository.go` (5 occurrences)
- `infra/repository/memory/repository.go` (4 occurrences)

- [ ] **Step 1: Replace all `.GetAggregateRoot()` with `.AggregateRoot()` in each file**

For each file listed above, use edit tool with `replaceAll: true`:
- oldString: `.GetAggregateRoot()`
- newString: `.AggregateRoot()`

- [ ] **Step 2: Verify production code compiles**

Run: `go build ./...`
Expected: FAIL only on test files and generator

---

### Task 5: Update all test code callers

**Files for `.GetID()` → `.ID()` (use `replaceAll`):**
- `it/infra_pg/repository_test.go` (8)
- `domain/entity/id_test.go` (8)
- `domain/entity/entity_test.go` (4)
- `domain/entity/soft_deletable_test.go` (4)
- `domain/entity/auditable_test.go` (4)
- `domain/aggregate/aggregate_test.go` (2)
- `domain/repository/repository_test.go` (6)
- `domain/repository/repositorytest/repository_contract.go` (6)
- `infra/repository/memory/repository_test.go` (5)
- `exampleapp/application/application_test.go` (4)
- `exampleapp/integration/integration_test.go` (3)
- `exampleapp/infrastructure/provider_contract_test.go` (2)
- `exampleapp/domain/domain_test.go` (2)

**Files for `.GetAggregateRoot()` → `.AggregateRoot()` (use `replaceAll`):**
- `domain/repository/repositorytest/repository_contract.go` (3)
- `infra/repository/memory/repository_test.go` (1)

- [ ] **Step 1: Replace `.GetID()` → `.ID()` in all test files**

For each file in the `.GetID()` list above, use edit with `replaceAll: true`:
- oldString: `.GetID()`
- newString: `.ID()`

- [ ] **Step 2: Replace `.GetAggregateRoot()` → `.AggregateRoot()` in test files**

For each file in the `.GetAggregateRoot()` list above, use edit with `replaceAll: true`:
- oldString: `.GetAggregateRoot()`
- newString: `.AggregateRoot()`

- [ ] **Step 3: Run all tests**

Run: `go test ./...`
Expected: ALL PASS (except possibly generator which uses templates)

---

### Task 6: Update code generator templates

**Files:**
- Modify: `cmd/ddd/generator/generator.go` (9 occurrences)

All 9 occurrences are `.GetID()` calls inside Go template strings. Replace with `.ID()`.

- [ ] **Step 1: Replace all `.GetID()` → `.ID()` in generator.go**

Use edit with `replaceAll: true`:
- oldString: `.GetID()`
- newString: `.ID()`

This covers all 9 occurrences at lines 159, 193, 205, 432, 437, 493, 501, 567, 621.

- [ ] **Step 2: Verify generator compiles**

Run: `go build ./cmd/ddd/...`
Expected: PASS

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: ALL PASS

---

### Task 7: Update documentation

**Files:**
- Modify: `docs/guide.md:51` — `p.GetID()` → `p.ID()`
- Modify: `docs/architecture.md:262` — `GetID()` → `ID()` in API table

- [ ] **Step 1: Update guide.md**

Replace `p.GetID()` with `p.ID()` in `docs/guide.md`.

- [ ] **Step 2: Update architecture.md**

Replace the `GetID()` reference in the Entity API table row in `docs/architecture.md:262`.

---

### Task 8: Final verification and commit

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 2: Full test suite**

Run: `go test ./...`
Expected: ALL PASS

- [ ] **Step 3: Grep for any remaining `GetID()` or `GetAggregateRoot()` in .go files**

Run: `rg '\.GetID\(\)|\.GetAggregateRoot\(\)' --type go`
Expected: ZERO matches

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor: rename GetID() → ID(), GetAggregateRoot() → AggregateRoot()

Comply with Effective Go getter naming convention.
Get-prefix is not idiomatic Go for simple accessors."
```

---

## Risk Notes

1. **Breaking change**: This is a public API change. Any external consumers must update their code.
2. **Generated code**: Projects already scaffolded with `ddd generate` will have `GetID()` in their codebase. They need to manually rename or re-run the generator.
3. **`AggregateRef` interface**: All types satisfying this interface must have the method renamed — currently only `*AggregateRoot` implements it, so the impact is contained.
4. **No `gorename`**: The deprecated tool is unavailable. All renames are mechanical `replaceAll` edits, verified by compilation.
