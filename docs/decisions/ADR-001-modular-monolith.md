# ADR-001: Форма модульного монолита как analytic/project

## Status
Accepted

## Date
2026-09-19

## Context
Нужна нарезка Go API с несколькими бизнес-модулями (auth, catalog, reviews, feed, …) и возможностью позже вынести feed/notifications. В `work/analytic` уже есть рабочий канон BC.

## Decision
Копировать **форму** модуля (`module.go`, `contract`, `events`, `domain` / `application/{command,query}` / `infrastructure/{write,read}` / `delivery/http`) и composition root. Не копировать стек analytic (GORM, golang-migrate, JWT IAM).

Связь BC: sync через `contract`, async через PostgreSQL Outbox + worker.

## Alternatives Considered

### Пакеты без слоёв (`internal/reviews/service.go`)
- Pros: меньше файлов на старте
- Cons: разъедется с analytic, сложнее выносить BC
- Rejected: канон команды — analytic

### Микросервисы сразу
- Rejected: нет нагрузки, которая это окупает

## Consequences
Каждый новый BC создаётся полным скелетом. sqlc queries живут в `infrastructure`, не в domain.
