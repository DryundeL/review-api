# Spec: auth

## Objective

Два входа.

Mini App: `POST /v1/auth/telegram` с `Authorization: tma <initData>` сразу отдаёт JWT. Сообщение в бот не отправляется.

Браузер: `POST /v1/auth/login` читает User-Agent и возвращает ссылку `https://t.me/<bot>?start=login_<id>`. Бот пишет человеку браузер и устройство и просит подтвердить. `POST /v1/auth/login/poll` с `{ "loginId" }` отдаёт тот же JWT, пока заявка жива.

`GET /v1/profile` требует `Authorization: Bearer <jwt>`. `GET /v1/auth/identity` по-прежнему проверяет initData. Паролей и SSO нет.

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
- Ask first: смена схемы заголовка, refresh-токены, отказ от initData.
- Never: пароли, SSO, доверие `user` без HMAC или без подтверждения в боте. JWT только после подтверждения, секрет не равен токену бота.

## Success Criteria

- [ ] Валидный initData → context содержит `TelegramID`, username, language, photo URL (если были).
- [ ] Невалидный / протухший → 401 `{ "error": { "code": "UNAUTHORIZED", "message": "..." } }`.
- [ ] Контракт `IdentityFromContext` доступен другим BC без импорта `auth/internal`.
- [ ] Rate limit на `/v1/*` (через platform Redis) не блокирует health.

## Open Questions

- Точное имя заголовка: `Authorization: tma …` vs `X-Telegram-Init-Data`. Предпочтение: `Authorization: tma <initData>`.
- Нужен ли короткий server session после первой проверки, или HMAC на каждый запрос. Предпочтение: HMAC на каждый запрос (stateless).
