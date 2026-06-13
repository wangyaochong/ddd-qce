# DDD Viewer Graph — Design Spec

## Goal

Add a "Graph" tab to DDD Viewer that visualizes static and runtime dependencies between Commands, Queries, Events, and Handlers across domains.

## Node Shapes

| Type | Shape |
|------|-------|
| Command | Rectangle |
| Query | Parallelogram |
| Event | Circle |
| Handler | Pentagon |

## Architecture: Independent RelationshipRegistry

Centralized registry that records handler↔type relationships at registration time and aggregates runtime trace data.

### Core: `core/relationship/registry.go`

```go
type RelationshipRegistry struct {
    commandHandlers map[string]string    // commandName → handlerName
    queryHandlers   map[string]string    // queryName → handlerName
    eventHandlers   map[string][]string  // eventName → [handlerName...]
    typeDomains     map[string]string    // typeName → domain
    mu              sync.RWMutex
}

type Graph struct {
    Nodes   []Node   `json:"nodes"`
    Edges   []Edge   `json:"edges"`
    Domains []string `json:"domains"`
}

type Node struct {
    ID     string `json:"id"`
    Type   string `json:"type"`   // "command", "query", "event", "handler"
    Domain string `json:"domain"`
}

type Edge struct {
    Source string `json:"source"`
    Target string `json:"target"`
    Type   string `json:"type"`   // "handles", "emits", "subscribes"
}
```

Key methods:
- `RecordCommandHandler(commandName, handlerName string)`
- `RecordQueryHandler(queryName, handlerName string)`
- `RecordEventHandler(eventName, handlerName string)`
- `RecordTypeDomain(typeName, domain string)`
- `BuildGraph(domainFilter string, typeFilter string) *Graph`
- `BuildRuntimeGraph(traceStore, traceID) *Graph`

### Core: `core/relationship/notifying_bus.go`

Decorator buses that auto-notify the registry on handler registration:

```go
type NotifyingCommandBus struct {
    inner command.CommandBus
    reg   *RelationshipRegistry
}
```

- `RegisterHandler(handler)` → extract type name + handler name → `reg.RecordCommandHandler()` → delegate to `inner.RegisterHandler()`
- Same pattern for `NotifyingQueryBus`, `NotifyingEventBus`
- All other methods (Execute, RegisteredTypes, Shutdown) delegate to inner

### Observability: DDDViewer integration

- New field `relRegistry *RelationshipRegistry` on `DDDViewer`
- New option `WithDDDViewerRelationshipRegistry(reg *RelationshipRegistry)`
- New route `GET /api/ddd/graph` with query params:
  - `mode=static|runtime|summary` (default: static)
  - `domain=order|inventory` (optional filter)
  - `type=command|query|event|handler` (optional filter)
  - `traceId=xxx` (for mode=runtime)
- New handler `handleGraph` that renders `ddd_graph.html`
- New JSON endpoint on same route (via `?json=1` or Accept header)

### Runtime trace aggregation

- From `TraceStore`: iterate spans, link command spans to event spans via ParentID/TraceID
- Build `handlerEmits` mapping: handler → [events it produces]
- In "summary" mode, aggregate across all traces

### Frontend: `observability/templates/ddd_graph.html`

- D3.js force-directed graph
- Node shapes: rectangle (command), parallelogram (query), circle (event), pentagon (handler)
- Color-coded by domain
- Interactions: drag, zoom, pan
- Left sidebar: domain filter checkboxes, node type filter checkboxes
- Click node → navigate to detail page (e.g. Command_Types page with highlight)
- Toggle: Static / Runtime views
- Edge labels: "handles", "emits", "subscribes"

### Nav: `observability/templates/ddd_layout.html`

Add `<a href="{{.Prefix}}/ddd_graph">Graph</a>` after Domains tab.

## File Changes

| File | Change |
|------|--------|
| `core/relationship/registry.go` | NEW: Registry + Graph/Node/Edge structs |
| `core/relationship/notifying_bus.go` | NEW: Decorator buses |
| `observability/ddd_viewer.go` | Add relRegistry field, option, handleGraph, RegisterRoutes entry |
| `observability/dashboard.go` | Add /api/ddd/graph JSON handler |
| `observability/templates/ddd_layout.html` | Add Graph nav tab |
| `observability/templates/ddd_graph.html` | NEW: D3.js graph page |
| `exampleapp/infrastructure/wire.go` | Create Registry, wrap buses, pass to DDDViewer |

## Data Flow

```
RegisterHandler(handler)
  → NotifyingBus extracts typeName + handlerName
  → Registry.RecordCommandHandler(typeName, handlerName)
  → inner.RegisterHandler(handler)

Command executes → TracingAspect records span
  → Handler publishes Event → TracingAspect records child span
  → TraceStore stores both spans with shared TraceID

GET /api/ddd/graph?mode=static
  → Registry.BuildGraph() → static relationships

GET /api/ddd/graph?mode=summary
  → Registry.BuildGraph() + TraceStore aggregation → runtime relationships

GET /api/ddd/graph?mode=runtime&traceId=xxx
  → TraceStore.GetTrace(xxx) → single trace graph
```
