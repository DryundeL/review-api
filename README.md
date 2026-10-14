# review-api

Telegram Mini App backend. Modular monolith: форма модуля как `analytic/project`, стек — Go 1.27, Echo, pgx, sqlc (позже), Goose, Redis, slog.

Карта: [CAPABILITY-MAP.md](CAPABILITY-MAP.md).

## Docker и сервер

Настройка пользователя `wapp`, переменных окружения, запуск и откат:
[deploy/README.md](deploy/README.md). Конфигурация сервисов — `compose.yaml`.
Push в `main` автоматически запускает проверки, сборку образа и деплой
через GitHub Actions. В pull request выполняются только проверки и сборка.

## Запуск

```
cp .env.example .env   # DB_*, REDIS_*, TELEGRAM_BOT_TOKEN, TELEGRAM_BOT_USERNAME, JWT_SECRET
go run ./cmd/migrator -command=up
go run ./cmd/api
go run ./cmd/worker
go run ./cmd/bot
```

- `GET /healthz` — процесс жив
- `GET /readyz` — PostgreSQL + Redis
- `GET /metrics` — Prometheus
- `POST /v1/auth/telegram` — Mini App, `Authorization: tma <initData>`, сразу `token`
- `POST /v1/auth/login` — вход из браузера, в ответе `url` на бота
- `POST /v1/auth/login/poll` — `{ "loginId" }`, пока не придёт `token`
- `GET /v1/profile` — `Authorization: Bearer <jwt>`
- `GET /v1/auth/identity` — `Authorization: tma <initData>`
- `GET /v1/review-types` — активные типы обзоров
- `GET /v1/review-statuses` — фиксированные статусы публикации и релиза
- `GET /v1/admin/review-types` — все типы, заголовок `X-Admin-Token`
- `POST /v1/admin/review-types` — добавить тип, тот же заголовок
- `PATCH /v1/admin/review-types/:code` — подпись, порядок, `isActive`, тот же заголовок
- `GET /v1/works?q=&type=&limit=` — произведения
- `GET /v1/works/:id`
- `POST /v1/works` — создать произведение, `Authorization: Bearer <jwt>`

```
go test ./...
go build ./cmd/api ./cmd/worker ./cmd/bot ./cmd/migrator
```
