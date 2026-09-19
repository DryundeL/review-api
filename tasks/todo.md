# Tasks: platform + auth

## Task 1: Platform skeleton

**Acceptance:**
- [x] Config из env (+ optional `.env`)
- [x] pgxpool, Redis ping, Goose embed, Echo mux
- [x] `/healthz` live, `/readyz` требует PG+Redis, `/metrics`

**Verify:** `go test ./internal/platform/... ./internal/http/...`

**Dependencies:** None

## Task 2: Outbox + worker

**Acceptance:**
- [x] Таблица `outbox_messages`
- [x] Append в TX, relay SKIP LOCKED
- [x] `cmd/worker` poll loop

**Verify:** `go test ./internal/platform/outbox/...`

**Dependencies:** Task 1

## Task 3: Auth HMAC + HTTP

**Acceptance:**
- [x] Валидный initData → identity в context
- [x] Протухший/битый hash → 401
- [x] `GET /v1/auth/identity`
- [x] Rate limit `/v1/*` не трогает health

**Verify:** `go test ./internal/modules/auth/...`

**Dependencies:** Task 1

## Checkpoint

- [x] `go test ./...`
- [x] `go build ./cmd/api ./cmd/worker ./cmd/migrator`
