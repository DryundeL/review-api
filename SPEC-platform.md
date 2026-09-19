# Spec: platform

## Objective

Общая инфраструктура API и worker: конфиг, PostgreSQL (pgx), Redis, Goose, transactional outbox, in-process event bus, Echo-сервер, slog, OpenTelemetry, Prometheus, health. Не bounded context: не содержит доменных правил обзоров.

Успех: `cmd/api` поднимается, `/health` зелёный, миграции накатываются, worker читает пустой outbox без ошибок.

## Tech Stack

См. `.cursor/rules/stack.mdc`. Go 1.27, Echo, pgxpool, Goose, slog, OTel, Prometheus.

## Commands

```
go run ./cmd/api
go run ./cmd/worker
go run ./cmd/migrator up
go test ./internal/platform/...
```

## Project Structure

```
cmd/api/
cmd/worker/
cmd/migrator/
internal/app/                 # Bootstrap, WireModules
internal/platform/config/
internal/platform/database/   # pgxpool
internal/platform/redis/
internal/platform/transaction/
internal/platform/outbox/
internal/platform/eventbus/
internal/platform/httpserver/
internal/platform/observability/
migrations/                   # Goose SQL
```

## Code Style

```go
err := txm.WithinTransaction(ctx, func(txCtx context.Context) error {
    if err := repo.Save(txCtx, agg); err != nil {
        return err
    }
    return publisher.Publish(txCtx, eventName, aggregateType, aggregateID, payload)
})
```

Outbox: `pending` → relay `FOR UPDATE SKIP LOCKED` → publish → `sent`. Poison payload: consumer возвращает nil, relayer не крутит вечно (как analytic worker: невалидный JSON логируется и не ретраится бесконечно — зафиксировать в реализации: `failed` после N попыток или skip).

## Testing Strategy

- Unit: outbox store на testdb / pgxmock по возможности; transaction helper.
- Интеграция: Testcontainers PostgreSQL — migrate up, append + relay одного сообщения.
- HTTP: health без модулей.

## Boundaries

- Always: секреты только из env; slog без initData и токенов; параметризованный SQL.
- Ask first: новые зависимости, смена Echo → другой роутер, смена Goose → Atlas.
- Never: GORM, импорт `modules/*/internal` из platform, бизнес-логика обзоров в platform.

## Success Criteria

- [ ] `cmd/api` слушает порт, `/healthz` и `/readyz` (ready = ping PG + Redis).
- [ ] Goose up/down на чистой БД.
- [ ] Outbox append в TX виден worker'у; concurrent relay не берёт одну строку дважды.
- [ ] Метрики Prometheus на `/metrics`, трейсы OTel опционально выключены без endpoint.
- [ ] Redis пингуется; падение Redis не роняет процесс на старте, если cache optional — **нет**: Redis обязателен для ready.

## Open Questions

- OpenAPI: отдельный spec-файл. Пока: Echo handlers, OpenAPI соберём в platform/httpserver на этапе auth.
