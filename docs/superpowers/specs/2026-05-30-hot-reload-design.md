# Hot Reload Design for exampleapp

## Problem

When developing the exampleapp frontend (HTML templates), editing a `.html` template file requires manually restarting `main.go` to see changes. This slows down development iteration.

## Goal

Two capabilities in development mode:
1. **Template hot reload**: Modified `.html` template files are picked up without restarting the Go process
2. **Browser auto-refresh**: Connected browsers automatically reload when templates change (LiveReload pattern)

## Architecture

### 1. Template Hot Reload

Currently `NewHandler` calls `ParseGlob` once at startup (`handlers.go:37`). In dev mode, templates should be re-parsed on each request.

Changes to `infrastructure/config.go`:
- Add `DevMode bool` field to `Config`, read from `DDD_DEV_MODE` env var (default `false`)

Changes to `interfaces/http/handlers.go`:
- `NewHandler` checks `Config.DevMode`
- If `true`: store the template glob pattern and `FuncMap`, don't parse at init. `render()` re-parses templates on every call
- If `false`: current behavior (parse once at startup, no overhead)

### 2. Browser Auto-Refresh via LiveReload

Add a lightweight WebSocket-based LiveReload server that:
- Watches template files for changes using `fsnotify`
- On change, sends a "reload" message to all connected browser clients
- Injects a small `<script>` tag into HTML responses in dev mode that connects to the WebSocket server

Changes to `interfaces/http/server.go`:
- If dev mode, register a `/livereload` WebSocket endpoint on the same `ServeMux`
- Inject livereload script into `render()` output when dev mode is on

New file `interfaces/http/livereload.go`:
- `LiveReloadServer` struct: manages WebSocket connections and file watching
- Uses `fsnotify` to watch the templates directory
- On file change, broadcasts reload to all connected clients
- Serves a minimal JS snippet at `/livereload.js`
- Starts in a goroutine, cleans up on server shutdown

### 3. Template Injection

In dev mode, `render()` injects a `<script>` tag before `</body>` that connects to `ws://host/livereload`. On receiving a reload message, the browser does `location.reload()`.

## Environment Variable

`DDD_DEV_MODE=1` enables:
- Template re-parsing on every render
- LiveReload WebSocket server
- Livereload script injection in HTML responses

When unset or `false`, behavior is identical to current production code (no overhead).

## Dependencies

- `fsnotify` (already commonly used in Go ecosystem, will add to go.mod)

## Files Changed

- `exampleapp/infrastructure/config.go` — add `DevMode` field
- `exampleapp/infrastructure/wire.go` — pass `DevMode` to server
- `exampleapp/interfaces/http/handlers.go` — conditional template re-parsing + script injection
- `exampleapp/interfaces/http/server.go` — register LiveReload WebSocket route in dev mode
- `exampleapp/interfaces/http/livereload.go` — new file: LiveReload server implementation

## Testing

- Existing tests pass unchanged (dev mode off by default)
- New test: verify LiveReload server starts/stops cleanly
- New test: verify livereload script is injected in dev mode responses
- Manual test: set `DDD_DEV_MODE=1`, edit a template, see browser refresh automatically