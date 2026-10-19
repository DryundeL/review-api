# Capability Map: Review Mini App

Статус: утверждена 2026-09-19.

Индекс модулей. Спеки: `SPEC-<id>.md`. Форма кода: `.cursor/rules/architecture.mdc`. Стек: `.cursor/rules/stack.mdc`.

## Intent

Telegram Mini App для обзоров цифровых тайтлов. Сначала свой круг, каркас сразу под публичный рост. Друзьям — полный MVP (визивиг, медиа, голоса, геймификация, метка релиза), не текстовый срез.

## Модули

| Module id | Тип | Responsibility | Depends on |
|---|---|---|---|
| platform | infra, не BC | pgx pool, Redis, Goose, outbox, eventbus, Echo, slog, OTel, Prometheus, health | — |
| auth | BC | Вход из браузера через подтверждение в боте и JWT; проверка Telegram `initData` | platform |
| users | BC | Профиль, аватар, `popularity_score`, EnsureUser по Telegram identity | auth (contract), media |
| media | BC | Upload, magic-bytes, лимиты, S3-compatible, без SVG | auth |
| catalog | BC | Тайтлы (`media_objects`): название, type, год, poster, `release_status`, поиск; пакетный импорт только admin | auth, media, reviews (коды типов) |
| reviews | BC | Черновик/публикация, блоки (`review_blocks`, ≤10), голоса (`review_votes`), просмотры (`review_views`), жалобы (`review_reports` + `report_reasons`, автоснятие по порогу). Справочник `review_types`. Статусы — конфиг модуля | catalog, media, users |
| social | BC | Подписки (`follows`) | users |
| feed | BC | Read-side лента: курсор, фильтр типов, режимы `following` / `popular` | reviews, social, users |
| search | BC | Read-side поиск: trends-табы (media / authors / types) + общий текстовый поиск (`@username`, название обзора, media object) | reviews, catalog, social, users |
| gamification | BC | Очки + Steam-lite магазин косметики (`shop_items`, inventory, equipped keys). Consumers `review.published` / `review.voted` / `user.followed`. Unlock'и за типы — позже | reviews, users, social |
| notifications | BC | Пуши в Telegram. Скелет сразу, хендлеры после ядра | users, social, reviews |

`cmd/bot` — процесс, не BC. `cmd/api`, `cmd/worker`, `cmd/migrator` — процессы.

## Порядок сборки

```
platform → auth → users, media → catalog → reviews → social → feed, search, gamification → notifications
```

Циклов нет. `auth` не импортирует `users`: кладёт `TelegramIdentity` в context через `auth/contract`. `users` читает контракт и делает EnsureUser в своём middleware, который вешает composition root.

## События (Outbox)

| Событие | Продюсер | Консьюмеры |
|---|---|---|
| `review.published` | reviews | feed, gamification, users (popularity), notifications |
| `review.unpublished` | reviews | — (позже notifications; feed/search читают `status`) |
| `review.voted` | reviews | feed, users, notifications |
| `review.viewed` | reviews | feed (батч, не каждый хит) |
| `user.followed` | social | feed, notifications |
| `media_object.created` | catalog | — (аудит/поиск) |
| `unlock.granted` | gamification | notifications, users — позже (unlock'и не в текущем срезе) |

Синхронно между BC — только `contract` (чтение, ACL). Side effects после write — только outbox в той же TX, что aggregate.

## Продуктовые инварианты

- Лента: два режима `following` и `popular`, фильтр типов в обоих. Курсор, page size 10.
- `release_status` принадлежит `catalog` (произведение). Обзор предлагает обновление.
- Без веб-скрапинга. Позже AniList/TMDB в то же поле.
- Косметика профиля (фон / рамка / подложка списка) — через очки и магазин; unlock'и за типы обзоров — позже.
- Один опубликованный обзор на пару `(user, media_object)`. Правка — PATCH.
- `type` тайтла задаётся при импорте и не меняется через публичный API.
- Типы MVP, стартовый набор `review_types`: `manga`, `manhwa`, `manhua`, `light_novel`, `comic`, `book`, `series`, `movie`, `dorama`, `anime_series`, `anime_movie`, `cartoon_series`, `cartoon_movie`, `game`. Код `light_novel` — ранобэ, как в клиенте. Админ может добавить тип. Статусы публикации и релиза не расширяются через API.

## Вне скоупа

Kafka/NATS, скрапинг, комментарии, свободные теги, несколько опубликованных обзоров на один тайтл, микросервисы.

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
| SPEC-gamification.md | нет (срез: [docs/api-gamification.md](docs/api-gamification.md)) |
| SPEC-notifications.md | нет |
