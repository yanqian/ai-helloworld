# Go Server Example

This example is a tiny dependency-free HTTP server for projects that want to adapt the harness to a Go service.

Endpoints:

- `GET /healthz` returns `{"status":"ok"}`.
- `GET /greet?name=Codex` returns `{"message":"hello, Codex"}`.

Run tests:

```bash
go test ./...
```

Run locally:

```bash
PORT=8080 go run .
```
