---
name: backend
description: Claude Code rules for Go backend development (REST /api for Svelte frontend)
---

You are an expert in Go, backend architecture, REST API design, PostgreSQL, migrations, observability, and secure production systems.
You build maintainable, testable, and well-structured Go services that power a frontend under `/api`.

## Key Principles

- Write concise, technical responses with accurate Go examples.
- Prefer simplicity and clarity over cleverness.
- Design for correctness, debuggability, and operational reliability.
- Keep code modular: handlers orchestrate, services implement use-cases, repositories handle persistence.
- Favor explicit dependencies (dependency injection) over global state.
- Optimize only with evidence; focus on latency, throughput, and correctness.

## Go Style and Conventions

- Follow standard Go conventions: `gofmt` always, `go vet`, `golangci-lint`.
- Keep functions small and single-purpose.
- Prefer early returns and guard clauses; avoid deep nesting.
- Use `context.Context` as the first parameter in request-bound functions: `func (s *Svc) Do(ctx context.Context, ...)`.
- Return errors, don’t panic (panic only for programmer errors at init-time).
- Errors must be wrapped with context: `fmt.Errorf("create user: %w", err)`.
- Avoid named returns except in very small functions where it improves clarity.
- Prefer interfaces defined at the consumer boundary (small, focused). Avoid “mega interfaces”.
- Keep structs cohesive; avoid “utility” packages with unrelated helpers.

## API Design (/api)

- RESTful, resource-oriented routes:
  - `/api/users`, `/api/users/{id}`, `/api/posts`, etc.
- Use plural nouns for collections, stable IDs, and consistent casing (prefer kebab-case in URLs only if required; otherwise lowercase paths).
- Use correct HTTP methods and status codes:
  - `GET` 200, `POST` 201, `PUT/PATCH` 200/204, `DELETE` 204
  - validation errors 400/422, auth 401, forbidden 403, not found 404, conflicts 409
- All responses are JSON:
  - set `Content-Type: application/json; charset=utf-8`
- Standardize an error response shape:
  - `{ "error": { "code": "...", "message": "...", "details": ... } }`
- Support request IDs and correlation IDs (header-based, echoed back).

## Project Structure

Prefer a structure that scales with features and avoids circular dependencies.

Example:

- `cmd/api/` (main package entrypoint)
- `internal/`
  - `app/` (wiring, DI, server setup)
  - `http/`
    - `handlers/` (transport layer)
    - `middleware/`
    - `router/`
  - `domain/` (entities + domain rules; no infra dependencies)
  - `services/` (use-cases / business logic)
  - `repo/` (persistence implementations)
  - `db/` (queries, migrations, connection)
  - `auth/` (token validation, session, permissions)
  - `observability/` (logging, metrics, tracing)
- `pkg/` only for truly reusable public libraries (often avoid entirely).

Rules:

- Transport (HTTP) depends on services; services depend on domain + repo interfaces.
- Repos implement interfaces in services layer (or service-defined boundary).
- Domain must not depend on database, HTTP, or external clients.

## HTTP Server

- Use `net/http` with a lightweight router (chi preferred) unless requirements demand otherwise.
- Handler responsibilities:
  - parse/validate input
  - call service
  - map service errors to HTTP
  - encode response
- No DB calls directly in handlers.
- Enforce timeouts:
  - server read/write timeouts
  - per-request context timeouts where appropriate
- Always close request bodies and limit size:
  - `http.MaxBytesReader` for JSON bodies

### JSON Handling

- Decode with `json.Decoder` and `DisallowUnknownFields()` for safer APIs.
- Validate after decoding; never trust client input.
- Encode with `json.Encoder` and set `SetEscapeHTML(false)` if you need raw strings (otherwise default is fine).

## Validation

- Validate at the boundary (handlers) and again in services for invariants.
- Prefer explicit validation functions and small helper types.
- Return field-level errors in a consistent format for frontend forms.

## Business Logic (Services)

- Services implement use-cases; keep them pure and deterministic where possible.
- Use RORO-style request/response structs:
  - `type CreateUserRequest struct { ... }`
  - `type CreateUserResponse struct { ... }`
- Avoid leaking transport models into services:
  - handlers map HTTP DTOs ↔ domain/service models.

## Persistence (Database)

- PostgreSQL recommended.
- Migrations are mandatory (goose, migrate, atlas—choose one and standardize).
- Use `database/sql` or `sqlc` for typed queries; avoid heavy ORMs.
- Repositories:
  - accept `ctx`
  - return domain models
  - never return raw DB rows to services
- Use transactions explicitly:
  - transaction boundaries live in services for multi-step use-cases
  - repositories accept a `Querier`/`Tx` abstraction as needed

### Query Rules

- Always parameterize queries (no string concat).
- Add indexes based on query patterns.
- Prefer pagination for lists:
  - cursor-based when possible
  - otherwise limit/offset with stable ordering

## Error Handling

- Define a small, typed error model:
  - sentinel errors for common cases (`ErrNotFound`, `ErrConflict`, `ErrUnauthorized`)
  - optional structured error type with `Code`, `Message`, `Fields`
- Wrap errors with context; don’t double-log.
- Map service errors → HTTP responses in one place (an error mapper).

## Security Baselines

- Input validation everywhere.
- Rate limiting at the edge or middleware.
- CORS configured explicitly (origins, methods, headers).
- Secure headers where appropriate.
- Secrets via environment variables or secret manager, never in repo.
- Always use TLS in production (terminate at proxy/load balancer is fine).

## Observability

- Structured logging (zap/zerolog/slog) with consistent fields:
  - `request_id`, `user_id` (if available), `method`, `path`, `status`, `latency_ms`
- Metrics (Prometheus/OpenTelemetry):
  - request count, latency histograms, error rates
- Tracing (OpenTelemetry) for service boundaries and DB calls.
- Health endpoints:
  - `/api/healthz` (liveness)
  - `/api/readyz` (readiness: DB connectivity, migrations applied)

## Concurrency and Performance

- Prefer bounded concurrency; avoid spawning goroutines per request without limits.
- Always pass `ctx` into goroutines; stop work on cancellation.
- Use connection pooling settings for DB.
- Avoid premature optimization; measure with pprof when needed.

## Configuration

- Use a single config struct populated from env.
- Validate config on startup; fail fast with clear errors.
- Keep defaults explicit and documented.

## Testing

- Unit tests for:
  - validation
  - service logic (with mocked repo interfaces)
  - error mapping
- Integration tests for:
  - repository + DB (use testcontainers if available)
  - HTTP handlers (httptest) with realistic requests/responses
- Avoid flaky tests: deterministic time, stable fixtures, isolated DB schema per test run.

## Key Conventions

1. `gofmt`, `go vet`, `golangci-lint` are non-negotiable.
2. Handlers orchestrate; services implement use-cases; repos handle persistence.
3. All request-bound functions accept `context.Context` first.
4. Standardize JSON responses and error shapes for the frontend.
5. Use migrations and typed queries (prefer `sqlc`) over ORMs.
6. Wrap errors with context and map them to HTTP in one place.
7. Security + observability are built-in, not bolted on.
