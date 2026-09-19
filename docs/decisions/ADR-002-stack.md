# ADR-002: Стек Go 1.27 + pgx + sqlc + Goose + Echo

## Status
Accepted

## Date
2026-09-19

## Context
Сентябрь 2026: стабильный Go 1.27. Analytic использует GORM + golang-migrate; для review-api выбран явный SQL. Исходная вилка HTTP была chi либо Echo.

## Decision
- HTTP: Echo v4 поверх `net/http` — `RegisterHTTP(*echo.Group)` на BC.
- DB: PostgreSQL, pgx v5 pool, sqlc, Goose.
- Cache: Redis. Files: S3-compatible в модуле media.
- API: REST + OpenAPI.
- Async: worker + outbox.
- Observability: slog + OpenTelemetry + Prometheus.

## Alternatives Considered

### GORM как в analytic
- Rejected: стек продукта — sqlc, не ORM analytic

### chi
- Pros: тонкая обёртка над `net/http`, ближе к stdlib
- Cons: меньше готовых биндингов и group middleware, чем у Echo
- Rejected: выбрана вторая опция исходной вилки стека

### Atlas вместо Goose
- Rejected на старте: две модели схемы. Goose SQL рядом с sqlc.

## Consequences
Репозитории модулей — тонкие обёртки над sqlc. Смена SQL — миграция Goose + query sqlc в одном PR.
HTTP-ошибки BC возвращают `error`; единый `HTTPErrorHandler` отдаёт `{error:{code,message}}`.
