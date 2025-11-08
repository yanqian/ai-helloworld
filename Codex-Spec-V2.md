# 🫭 Codex Specification v2 — Microservice Backend Boilerplate

**Version:** 2.0
**Author:** Yan Qiang
**Target:** Go microservice backend (REST/gRPC + DB + Kafka)
**Goal:** Provide a production-grade, testable, debuggable, and extendable baseline project.

---

## 1. Repository Layout

```
backend-service/
├── cmd/
│   └── app/
│       ├── main.go
│       └── wire.go                # Dependency injection (Google Wire)
├── internal/
│   ├── domain/
│   │   └── user/
│   │       ├── model.go
│   │       ├── repository.go
│   │       └── service.go
│   ├── infra/
│   │   ├── db/
│   │   │   └── mysql.go
│   │   ├── cache/
│   │   │   └── redis.go
│   │   ├── queue/
│   │   │   └── kafka_producer.go
│   │   └── config/
│   │       └── config.go
│   ├── interface/
│   │   ├── http/
│   │   │   ├── handler.go
│   │   │   └── router.go
│   │   └── grpc/
│   │       └── server.go
│   └── bootstrap/
│       └── app.go
├── pkg/
│   ├── logger/
│   │   └── logger.go
│   ├── errors/
│   │   └── errors.go
│   └── util/
│       └── timeutil.go
├── tests/
│   ├── unit/
│   │   └── user_service_test.go
│   ├── integration/
│   │   └── mysql_integration_test.go
│   └── e2e/
│       └── http_signup_test.go
├── configs/
│   └── config.yaml
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
├── README.md
└── .github/
    └── workflows/
        └── ci.yaml
```

---

## 2. Makefile (Build & Test Automation)

```makefile
APP_NAME=backend-service

.PHONY: all lint test build run docker

all: lint test build

lint:
	golangci-lint run

test:
	go test ./... -v -coverprofile=coverage.out

build:
	go build -o bin/$(APP_NAME) ./cmd/app

run:
	./bin/$(APP_NAME)

docker:
	docker build -t $(APP_NAME):latest .
```

---

## 3. Dockerfile (Production-Ready)

```dockerfile
FROM golang:1.23 AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app ./cmd/app

FROM gcr.io/distroless/base-debian12
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
```

---

## 4. go.mod (Module Template)

```go
module github.com/yanqian/ai-helloworld

go 1.23

require (
	github.com/gin-gonic/gin v1.10.0
	github.com/go-redis/redis/v9 v9.5.1
	github.com/segmentio/kafka-go v0.5.1
	github.com/google/wire v0.6.0
	go.opentelemetry.io/otel v1.25.0
)
```

---

## 5. Config Example (`configs/config.yaml`)

```yaml
server:
  port: 8080
  mode: release

db:
  host: localhost
  port: 3306
  user: root
  password: password
  name: user_service

redis:
  addr: localhost:6379
  db: 0

kafka:
  brokers:
    - localhost:9092

log:
  level: info
```

---

## 6. HTTP Example (`internal/interface/http/handler.go`)

```go
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"backend-service/internal/domain/user"
)

type UserHandler struct {
	Service *user.Service
}

func (h *UserHandler) Create(c *gin.Context) {
	var req user.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.Service.Create(c, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}
```

---

## 7. Logging (`pkg/logger/logger.go`)

```go
package logger

import (
	"log/slog"
	"os"
)

func New(level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

func parseLevel(l string) slog.Level {
	switch l {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
```

---

## 8. Bootstrap (`internal/bootstrap/app.go`)

```go
package bootstrap

import (
	"backend-service/internal/infra/config"
	"backend-service/internal/infra/db"
	"backend-service/internal/domain/user"
	"backend-service/internal/interface/http"
	"backend-service/pkg/logger"
)

func Start() {
	cfg := config.Load()
	log := logger.New(cfg.Log.Level)
	dbConn := db.Connect(cfg.DB)
	userRepo := user.NewRepository(dbConn)
	userService := user.NewService(userRepo)
	httpSrv := http.NewServer(cfg.Server, userService, log)
	httpSrv.Start()
}
```

---

## 9. CI/CD Example (GitHub Actions)

`.github/workflows/ci.yaml`

```yaml
name: CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: 1.23
      - name: Lint
        run: golangci-lint run
      - name: Test
        run: go test ./... -v -cover
      - name: Build
        run: go build ./cmd/app
```

---

## 10. Developer Workflow

| Step         | Command       | Description                |
| ------------ | ------------- | -------------------------- |
| 👩‍💻 Setup  | `make build`  | Build binary               |
| 🥪 Test      | `make test`   | Run unit/integration tests |
| 🧹 Lint      | `make lint`   | Ensure code quality        |
| 🐳 Docker    | `make docker` | Build container image      |
| 🚀 Run local | `make run`    | Start local server         |
| ✅ CI         | automatic     | Build + test + lint on PR  |

---

## 11. Local Development with Docker Compose

`docker-compose.yaml`

```yaml
version: "3.9"
services:
  db:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: user_service
    ports:
      - "3306:3306"

  redis:
    image: redis:7
    ports:
      - "6379:6379"

  kafka:
    image: bitnami/kafka:3
    ports:
      - "9092:9092"

  app:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - db
      - redis
      - kafka
```

---

## 12. Extension Pattern Example (Adding a Payment Gateway)

```go
// domain/payment/gateway.go
type Gateway interface {
	Charge(ctx context.Context, req ChargeRequest) (ChargeResponse, error)
}

// infra/payment/stripe.go
type StripeGateway struct {}
func (s *StripeGateway) Charge(ctx context.Context, req ChargeRequest) (ChargeResponse, error) {
	// call Stripe API
}
```

Adding a new gateway (e.g. PayPal) means:
✅ Implement `Gateway`
✅ Inject via `wire.go`
✅ Zero core changes to domain logic

---

## 13. Observability Standards

| Aspect      | Library             | Rule                                   |
| ----------- | ------------------- | -------------------------------------- |
| Logging     | `slog` JSON         | Include `trace_id` and `component`     |
| Metrics     | `Prometheus`        | Track QPS, latency, errors             |
| Tracing     | `OpenTelemetry`     | Propagate context across microservices |
| Healthcheck | `/healthz` endpoint | Return OK/ready                        |

---

## 14. Production Ready Checklist

* [ ] All configs validated at startup
* [ ] Graceful shutdown (`context.WithTimeout`)
* [ ] Structured JSON logs
* [ ] Unit tests > 80% coverage
* [ ] Prometheus metrics exported
* [ ] Docker image built and tagged by Git SHA
* [ ] CI gates enforced

---

## 15. Future Extensions

| Category      | Possible Add-on      |
| ------------- | -------------------- |
| Auth          | JWT middleware       |
| Rate-limit    | Redis token bucket   |
| Config        | Consul or etcd       |
| Deployment    | Helm + ArgoCD        |
| Queue         | Kafka consumer group |
| Observability | Jaeger + Grafana     |

---

## 16. Final Note

> “A professional backend is a system, not a script.”
> This codex exists to ensure every service in your ecosystem is **consistent**, **predictable**, and **observable**.
> Once you can build one service like this, you can scale a hundred.

---
