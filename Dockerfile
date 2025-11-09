FROM golang:1.23 AS builder
WORKDIR /src

# Download dependencies first for better layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy the remaining source and build the binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /src/bin/app ./cmd/app

FROM gcr.io/distroless/base-debian12
ENV PORT=8080
WORKDIR /app

COPY --from=builder /src/bin/app ./app
COPY configs ./configs

EXPOSE 8080
ENTRYPOINT ["/app/app"]
