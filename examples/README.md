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
