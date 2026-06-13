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

Starts a web server on http://localhost:8555 with an order management UI.

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
