# Implementation Plan: platform + auth

## Overview

Первый вертикальный срез: процесс API поднимается, health/ready пингуют PG и Redis, Goose накатывает outbox, worker крутит пустой relay, Mini App может пройти HMAC initData и получить identity.

## Architecture Decisions

- Форма BC — как analytic/project. Стек — pgx/sqlc/Goose/Echo (sqlc появится в catalog/reviews).
- `auth` не зависит от `users`. Identity в context через `auth/contract`.
- HMAC на каждый запрос, заголовок `Authorization: tma <initData>`.
- Outbox на pgx + `FOR UPDATE SKIP LOCKED`.

## Task List

См. [todo.md](todo.md).

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Нет локальных PG/Redis | Med | Unit-тесты auth/health без инфры; migrate/ready — вручную или с DSN |
| Путаница HMAC key/message | High | Фикстуры по официальному алгоритму Telegram |

## Open Questions

Нет для этого среза. OpenAPI — следующий срез после users.
