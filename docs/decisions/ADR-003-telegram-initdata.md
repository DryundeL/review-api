# ADR-003: Auth через Telegram Mini App initData

## Status
Accepted

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
