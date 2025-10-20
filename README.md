# Product Update Service Simulator

[![Test Reports](https://img.shields.io/badge/Test%20Reports-GitHub%20Pages-blue)](https://fairyhunter13.github.io/product-update-service-simulator/)

A minimal, production-informed Go service that accepts product update events asynchronously and exposes product state over HTTP. Designed to demonstrate: partial updates, non-blocking ingestion via an effectively-unbounded queue, dynamic worker scaling, strict JSON decoding, structured JSON logging, and graceful shutdown.

## Setup Instructions

- Prerequisites
  - Go 1.25+
  - Optional: Docker 24+, Docker Compose

- Build & run
```bash
go run ./cmd/product-update-service-simulator
# or
HTTP_ADDR=":8080" WORKER_MIN=3 WORKER_MAX=8 go run ./cmd/product-update-service-simulator
```

- Send events (partial updates allowed)
```bash
curl -i -X POST "http://localhost:8080/events" \
  -H "Content-Type: application/json" \
  -H "X-Request-Id: demo-req-1" \
  -d '{"product_id":"p-1","price":10.5}'
```

- Get product state
```bash
curl -i "http://localhost:8080/products/p-1"
```

## Environment variables

- HTTP_ADDR (default ":8080"): HTTP listen address
- SHUTDOWN_TIMEOUT (seconds, default 15): drain window before exit
- WORKER_COUNT (default = WORKER_MIN): initial worker count
- WORKER_MIN (default 3), WORKER_MAX (default 8): worker bounds
- SCALE_INTERVAL_MS (default 500): scaler tick interval (ms)
- SCALE_UP_BACKLOG_PER_WORKER (default 100): scale-up threshold per worker
- SCALE_DOWN_IDLE_TICKS (default 6): scale-down after this many idle ticks
- QUEUE_HIGH_WATERMARK (default 5000): soft cap; warn when backlog exceeds (no drops)

## API

- POST /events
  - Content-Type: application/json (strict). Unknown fields → 400.
  - Body: `{ "product_id": "...", "price": 12.3?, "stock": 7? }`
    - `product_id` required
    - `price` and/or `stock` optional, each `>= 0` when present
  - Response: `202 Accepted` with JSON acknowledgment
```json
{
  "status": "accepted",
  "request_id": "...",
  "sequence": 123,
  "product_id": "p-1",
  "received_at": "2025-10-20T15:04:05Z",
  "queue_depth": 42,
  "backlog_size": 12,
  "worker_count": 4
}
```
  - During shutdown: `503` `{ "error": "shutting_down" }`
  - API Documentation: `/openapi.yaml` (OpenAPI) and `/docs` (Swagger UI)
    - Local links: http://localhost:8080/openapi.yaml and http://localhost:8080/docs
    - Static (GitHub Pages): https://fairyhunter13.github.io/product-update-service-simulator/api/ and https://fairyhunter13.github.io/product-update-service-simulator/api/openapi.yaml

## Reports (GitHub Pages)

- **Dashboard**: https://fairyhunter13.github.io/product-update-service-simulator/
- **Unit only**: https://fairyhunter13.github.io/product-update-service-simulator/unit.html
- **Integration only**: https://fairyhunter13.github.io/product-update-service-simulator/integration.html
- **History by tag**: https://fairyhunter13.github.io/product-update-service-simulator/<tag>/ (e.g., `/v1.0.0/`)

- GET /products/{id}
  - 200 with `{ "product_id", "price", "stock" }` or 404 if unknown

## Design Choices

- Queue & ingestion
  - Non-blocking enqueue to a slice-backed backlog with channel handoff
  - Soft cap: `QUEUE_HIGH_WATERMARK` emits warnings (no 503, no drops)
  - Monotonic sequence assigned at intake for last-write-wins
  - Production note: replace the in-memory queue with RabbitMQ. Use durable queues, publisher confirms, manual acks, dead-lettering with retry backoff, and keep consumer-side sequence gating (only `event.sequence > last_sequence` mutates state) to achieve effective exactly-once with external stores.
- Dynamic worker scaling
  - Scale up when `backlog_size > worker_count * SCALE_UP_BACKLOG_PER_WORKER`
  - Scale down after `SCALE_DOWN_IDLE_TICKS` intervals of zero backlog
  - Clamped to `[WORKER_MIN, WORKER_MAX]`
- Store semantics
  - Thread-safe map with `sync.RWMutex`
  - Partial updates: only provided fields mutate state
  - Last-write-wins by sequence; equal sequence is idempotent no-op
- Strict JSON decoding & validation
  - `json.Decoder.DisallowUnknownFields()`; 400 on unknown/malformed
  - Enforce `Content-Type: application/json` → 415 otherwise
- Logging & observability
  - `log/slog` JSON output
  - Correlation via `X-Request-Id` (or generated UUID)
  - Queue metrics available in logs: `events_enqueued/processed`, `backlog_size`, `worker_count`
- Graceful shutdown
  - Reject new events with 503 while draining queued items
  - Logs mark begin/end drain and timeouts

## Production Considerations

- Queuing (RabbitMQ)
  - Durability & confirms:
    - Declare durable exchanges/queues; publish persistent messages.
    - Use publisher confirms to guarantee broker acceptance; handle nacks/retries.
  - Idempotency and ordering:
    - Use a deterministic idempotency key (e.g., `message_id = product_id+sequence`) and the service’s sequence gating so only `event.sequence > last_sequence` mutates state.
    - To improve per-product ordering, route by `product_id` (e.g., consistent-hash or topic exchange with hashed routing keys) to a bounded set of queues/consumers.
  - Consumption & retries:
    - Manual acks with `basic.ack`; set `basic.qos` (prefetch) to control concurrency.
    - Use a DLX (dead-letter exchange) for retries with backoff (TTL + DLX or delayed exchange plugin). After N attempts, route to a DLQ for inspection.
  - Exactly-once in practice:
    - RabbitMQ cannot guarantee global exactly-once delivery, but combining publisher confirms, persistent messages, manual acks, and idempotent consumers (sequence gating) achieves effective exactly-once for state updates.
  - Operations:
    - Monitor queue depths, redeliveries, unacked counts, and DLQ volumes. Alert on sustained growth.

- Persistence (PostgreSQL or Redis)
  - PostgreSQL: Upsert with sequence gating to enforce last-write-wins, e.g., conflict on `product_id` and update only when `excluded.last_sequence > products.last_sequence`.
  - Redis: Store product hash with `last_sequence`; use a Lua script to apply updates only when `new_seq > last_seq` atomically.
  - Index on `product_id`; expose read models with caching where appropriate.

- Large-scale and high throughput
  - Scale workers horizontally; consider autoscaling (HPA/KEDA based on backlog/lag).
  - Batch event processing when safe; apply gzip compression and limit payload size.
  - Add admission control and rate limits; enforce memory guards when backlog grows.
  - Consider switching to protobuf/MsgPack if JSON serialization becomes a bottleneck.

- Error handling and retries
  - Exponential backoff with jitter; move poison messages to DLQ after N attempts.
  - Ensure updates are idempotent via `product_id+sequence` so retries are safe.
  - Emit structured errors with correlation id; define SLOs and alert on breach.

## Project layout

- `cmd/product-update-service-simulator/` — service entrypoint
- `internal/http/` — handlers, router, middleware
- `internal/model/` — API types
- `internal/store/` — thread-safe in-memory store
- `internal/queue/` — queue, manager, sequencer
- `internal/obs/` — logging setup
- `internal/config/` — env-driven configuration
- `build/Dockerfile` — multi-stage build

Follows common patterns from the community `golang-standards/project-layout` (use `internal/` for non-exported code; `cmd/` for entrypoints).

## Testing

- Unit & integration tests
```bash
go test ./... -race -covermode=atomic -coverprofile=coverage.out
```
- Highlights
  - Handlers: happy-path, strict decoding (400 unknown fields), 415 content-type, shutdown 503
  - Store: partial updates, last-write-wins, concurrency
  - Queue/Manager: non-blocking enqueue, shutdown intake, drain
  - Test layout: unit tests under `internal/...`; integration tests under `test/integration/`
  - Integration: end-to-end HTTP tests against a running service

### Integration tests (Docker Compose)

Run the service and execute integration tests in containers using Compose:

```bash
docker compose up -d app
docker compose run --rm itest
docker compose down -v
```

## CI/CD

- GitHub Actions (`.github/workflows/ci.yml`)
  - `go vet` and `go test` with `-race -cover`
  - Coverage gate: fails if total coverage < 80%
  - `golangci-lint` (action) for static analysis
  - Docker build verification with `build/Dockerfile`

### Linting note

- `.golangci.yml` disables the `typecheck` analyzer to avoid Go 1.25 stdlib export data mismatches during CI.
- CI builds `golangci-lint` from source with Go 1.25 (`install-mode: goinstall`) to ensure compatibility.
- Local setup: install `golangci-lint` per official docs and run `make lint`.
  - Docs: https://golangci-lint.run/docs/welcome/install/
  - GitHub Action: https://github.com/golangci/golangci-lint-action

## Docker

```bash
docker build -f build/Dockerfile -t product-update-service-simulator:dev .
docker run --rm -p 8080:8080 product-update-service-simulator:dev
```

## Troubleshooting Strategies

- Data consistency problems
  - Verify `last_sequence` monotonicity per product; older events should be ignored by design.
  - Check worker logs for out-of-order processing; sequence gating prevents regressions.
  - Confirm single-writer semantics for each product in persistence (upsert with sequence predicate).

- Products aren't updating despite events being received
  - Confirm POST acks include a `sequence` and `status=accepted`.
  - Inspect `/debug/metrics` for `backlog_size`, `queue_depth`, and `worker_count` to ensure processing is active.
  - Search logs for worker panics or validation errors; ensure `Content-Type: application/json`.
  - Check that `product_id` in events matches the queried `GET /products/{id}`.
  - Ensure sequence gating is not discarding equal/older events (expected behavior); send a newer sequence.
  - If shutting down, POST will return 503; wait for drain to complete.
  - For RabbitMQ: check queue depth, unacked messages, and DLQ counts. Verify consumers are acknowledging (no stuck unacked), publisher confirms are enabled and succeeding, and retry routing (TTL/DLX) is working as expected.

## License

MIT
