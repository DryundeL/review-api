# Spec: auth

## Objective

Единственная аутентификация Mini App: проверить Telegram `initData`, положить `TelegramIdentity` в request context. Паролей, JWT и SSO нет.

Пользователь API: Next.js Mini App шлёт `Authorization: tma <initData>` (или заголовок `X-Telegram-Init-Data`). Неаутентифицированные запросы — только health/metrics/openapi.

## Tech Stack

Go 1.27, Echo middleware, HMAC-SHA256 по [Telegram WebApp initData](https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app). Секрет — `TELEGRAM_BOT_TOKEN`. `users` создаёт строку профиля отдельно (EnsureUser).

## Commands

```
go test ./internal/modules/auth/...
```

## Project Structure

```
internal/modules/auth/
├── module.go
├── contract/          # TelegramIdentity, ErrUnauthorized, context accessors
├── events.go          # пустой, пока нет integration events
└── internal/
    ├── domain/        # правила: max age, обязательные поля
    ├── application/   # ValidateInitData
    ├── infrastructure/
    └── delivery/http/ # Middleware
```

## Code Style

```go
id, err := authcontract.IdentityFromContext(r.Context())
if err != nil {
    // 401
}
```

`auth` не импортирует `users`. Composition root: `Auth.Middleware` → `Users.EnsureUserMiddleware`.

Проверка: HMAC по `secret_key = HMAC_SHA256("WebAppData", botToken)`, сравнение `hash`, `auth_date` не старше `AUTH_MAX_AGE` (default 24h).

## Testing Strategy

- Фикстуры initData: валидная, протухшая, битый hash, пустой user.
- Timing-safe compare.
- Middleware: 401 без заголовка, 200 с валидным.

Не ходить в Telegram network.

## Boundaries

- Always: не логировать сырой initData; не принимать initData из query на write-ручках.
- Ask first: смена схемы заголовка, web login widget, бот-команды как второй auth.
- Never: JWT, сессии с паролем, доверие `user` без HMAC.

## Success Criteria

- [ ] Валидный initData → context содержит `TelegramID`, username, language, photo URL (если были).
- [ ] Невалидный / протухший → 401 `{ "error": { "code": "UNAUTHORIZED", "message": "..." } }`.
- [ ] Контракт `IdentityFromContext` доступен другим BC без импорта `auth/internal`.
- [ ] Rate limit на `/v1/*` (через platform Redis) не блокирует health.

## Open Questions

- Точное имя заголовка: `Authorization: tma …` vs `X-Telegram-Init-Data`. Предпочтение: `Authorization: tma <initData>`.
- Нужен ли короткий server session после первой проверки, или HMAC на каждый запрос. Предпочтение: HMAC на каждый запрос (stateless).
