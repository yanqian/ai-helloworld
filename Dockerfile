FROM golang:1.23 AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app ./cmd/app

FROM gcr.io/distroless/base-debian12
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
