# review-api

Telegram Mini App backend. Modular monolith: форма модуля как `analytic/project`, стек — Go 1.27, Echo, pgx, sqlc (позже), Goose, Redis, slog.

Карта: [CAPABILITY-MAP.md](CAPABILITY-MAP.md).

## Запуск

```
cp .env.example .env   # DB_*, REDIS_*, TELEGRAM_BOT_TOKEN
go run ./cmd/migrator -command=up
go run ./cmd/api
go run ./cmd/worker
```

- `GET /healthz` — процесс жив
- `GET /readyz` — PostgreSQL + Redis
- `GET /metrics` — Prometheus
- `GET /v1/auth/identity` — `Authorization: tma <initData>`

```
go test ./...
go build ./cmd/api ./cmd/worker ./cmd/migrator
```
