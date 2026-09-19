# Capability Map: Review Mini App

Статус: утверждена 2026-09-19.

Индекс модулей. Спеки: `SPEC-<id>.md`. Форма кода: `.cursor/rules/architecture.mdc`. Стек: `.cursor/rules/stack.mdc`.

## Intent

Telegram Mini App для обзоров цифровых тайтлов. Сначала свой круг, каркас сразу под публичный рост. Друзьям — полный MVP (визивиг, медиа, голоса, геймификация, метка релиза), не текстовый срез.

## Модули

| Module id | Тип | Responsibility | Depends on |
|---|---|---|---|
| platform | infra, не BC | pgx pool, Redis, Goose, outbox, eventbus, Echo, slog, OTel, Prometheus, health | — |
| auth | BC | Проверка Telegram `initData` (HMAC + max age), `TelegramIdentity` в context, middleware | platform |
| users | BC | Профиль, аватар, `popularity_score`, EnsureUser по Telegram identity | auth (contract), media |
| media | BC | Upload, magic-bytes, лимиты, S3-compatible, без SVG | auth |
| catalog | BC | Произведения, типы-теги, обложка, `release_status`, поиск, создание UGC | auth, media, users |
| reviews | BC | Черновик/публикация, JSON-тело, голоса, просмотры, primary type | catalog, media, users |
| social | BC | Подписки | users |
| feed | BC | Read-side лента: курсор, фильтр типов, режимы `following` / `popular` | reviews, social, users |
| gamification | BC | Unlock'и, подсказки в ЛК. Consumer `ReviewPublished` | reviews, users |
| notifications | BC | Пуши в Telegram. Скелет сразу, хендлеры после ядра | users, social, reviews |

`cmd/bot` — процесс, не BC. `cmd/api`, `cmd/worker`, `cmd/migrator` — процессы.

## Порядок сборки

```
platform → auth → users, media → catalog → reviews → social → feed, gamification → notifications
```

Циклов нет. `auth` не импортирует `users`: кладёт `TelegramIdentity` в context через `auth/contract`. `users` читает контракт и делает EnsureUser в своём middleware, который вешает composition root.

## События (Outbox)

| Событие | Продюсер | Консьюмеры |
|---|---|---|
| `review.published` | reviews | feed, gamification, users (popularity), notifications |
| `review.voted` | reviews | feed, users, notifications |
| `review.viewed` | reviews | feed (батч, не каждый хит) |
| `user.followed` | social | feed, notifications |
| `work.created` | catalog | — (аудит/поиск) |
| `unlock.granted` | gamification | notifications, users |

Синхронно между BC — только `contract` (чтение, ACL). Side effects после write — только outbox в той же TX, что aggregate.

## Продуктовые инварианты

- Лента: два режима `following` и `popular`, фильтр типов в обоих. Курсор, page size 10.
- `release_status` принадлежит `catalog` (произведение). Обзор предлагает обновление.
- Без веб-скрапинга. Позже AniList/TMDB в то же поле.
- Аватарка за 3 разных `primary type`. Рамка аватара за все 13 типов.
- Один опубликованный обзор на пару `(user, work)`. Правка — PATCH.
- `primaryType` неизменяем после публикации.
- Типы MVP (закрытый enum): `manga`, `manhwa`, `manhua`, `ranobe`, `comic`, `book`, `series`, `movie`, `dorama`, `anime_series`, `anime_movie`, `cartoon_series`, `cartoon_movie`.

## Вне скоупа

Kafka/NATS, скрапинг, комментарии, тип `games`, свободные теги, несколько опубликованных обзоров на один тайтл, микросервисы.

## Спеки

| Файл | Статус |
|---|---|
| [SPEC-platform.md](SPEC-platform.md) | утверждена, срез 1 реализован |
| [SPEC-auth.md](SPEC-auth.md) | утверждена, срез 1 реализован |
| SPEC-users.md | нет |
| SPEC-media.md | нет |
| SPEC-catalog.md | нет |
| SPEC-reviews.md | нет |
| SPEC-social.md | нет |
| SPEC-feed.md | нет |
| SPEC-gamification.md | нет |
| SPEC-notifications.md | нет |
