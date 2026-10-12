# ADR-003: Auth через Telegram Mini App initData

## Status
Accepted, browser login added 2026-10-13

Браузерный вход: `POST /v1/auth/login` создаёт одноразовую ссылку в бота. Бот показывает браузер и устройство и просит подтвердить. После подтверждения poll отдаёт JWT (HS256, `JWT_SECRET`). `GET /v1/profile` требует `Authorization: Bearer`. initData на `/v1/auth/identity` остаётся для Mini App.

## Date
2026-09-19

## Context
Единственный клиент — Telegram Mini App. Отдельная учётка не нужна.

## Decision
Проверять `initData` HMAC на каждый запрос. `auth` кладёт `TelegramIdentity` в context. `users.EnsureUser` создаёт/обновляет профиль. JWT и SSO из analytic не используем.

## Alternatives Considered

### JWT после первого initData
- Pros: меньше HMAC
- Cons: ревокация, секрет, расхождение с Telegram
- Rejected на MVP: stateless HMAC

## Consequences
`auth` не зависит от `users`. Бот-токен — секрет HMAC, только env.
