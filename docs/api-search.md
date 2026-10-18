# Search API (для фронта)

Экран поиска как в Twitter Explore:

- пустой инпут → **trends** по табам (`media` / `authors` / `types`);
- ввод текста → **общий** поиск сразу по никам, обзорам и тайтлам.

## UI-поток

```
1. Открыли поиск (q пустой)
   → GET /v1/search/trends?tab=media|authors|types
   → три таба Explore, переключение только меняет tab

2. Пользователь ввёл строку (debounce ~300ms)
   → GET /v1/search?q=...
   → один ответ: users + reviews + mediaObjects
   → trends скрыть; пустые секции не показывать

3. Клик:
   · user        → GET /v1/profiles/{id}  (см. api-social.md)
   · review      → карточка обзора по id
   · mediaObject → GET /v1/media-objects/{id}
   · type (trends) → GET /v1/feed?type=manga  (см. api-feed.md)
```

Auth **опционален**. С Bearer заполняется `isFollowing` у авторов/юзеров.

---

## Trends (пустой поиск)

### `GET /v1/search/trends`

| Параметр | Default | Значения | Заметки |
|---|---|---|---|
| `tab` | `media` | `media`, `authors`, `types` | активный таб |
| `limit` | `10` | `1`–`20` | только для `media` и `authors`; у `types` всегда все активные типы |

Ответ: `{ "tab": "...", "items": [...] }`.

### `tab=media` — топ тайтлов по числу опубликованных обзоров

```json
{
  "tab": "media",
  "items": [
    {
      "id": "uuid",
      "title": "Title",
      "type": "anime_series",
      "posterUrl": "https://...",
      "reviewsCount": 42
    }
  ]
}
```

### `tab=authors` — топ авторов по числу подписчиков

```json
{
  "tab": "authors",
  "items": [
    {
      "id": "uuid",
      "username": "ann",
      "firstName": "Ann",
      "photoUrl": "https://t.me/i/userpic/320/....jpg",
      "followersCount": 120,
      "isFollowing": false
    }
  ]
}
```

- `username` — ник Telegram **без** `@`. Может быть `""`.
- `isFollowing` без auth / на себя → `false`.

### `tab=types` — типы по числу опубликованных обзоров

```json
{
  "tab": "types",
  "items": [
    {
      "code": "manga",
      "label": "Манга",
      "reviewsCount": 50
    }
  ]
}
```

Порядок: `reviewsCount DESC`, затем `sort_order` справочника.

### Примеры

```http
GET /v1/search/trends
GET /v1/search/trends?tab=authors&limit=10
Authorization: Bearer ...
GET /v1/search/trends?tab=types
```

---

## Текстовый поиск (общий)

### `GET /v1/search`

| Параметр | Default | Заметки |
|---|---|---|
| `q` | — | **обязателен**; 1–100 символов после нормализации |
| `limit` | `20` | `1`–`50`; лимит **на каждую** секцию |

Нормализация `q` на бэкенде:

1. `trim`
2. снять один ведущий `@` (`@ann` → `ann`)
3. поиск case-insensitive substring (`ILIKE`)

Что ищется в одном запросе:

| Секция | Поле |
|---|---|
| `users` | `username` (ник TG) |
| `reviews` | `title` опубликованного обзора |
| `mediaObjects` | `title` / `originalTitle` тайтла |

### Response `200`

```json
{
  "users": [
    {
      "id": "uuid",
      "username": "ann",
      "firstName": "Ann",
      "photoUrl": "https://t.me/i/userpic/320/....jpg",
      "followersCount": 12,
      "isFollowing": true
    }
  ],
  "reviews": [
    {
      "id": "uuid",
      "title": "Почему это шедевр",
      "authorId": "uuid",
      "username": "ann",
      "photoUrl": "https://t.me/i/userpic/320/....jpg",
      "mediaObject": {
        "id": "uuid",
        "title": "Title",
        "type": "anime_series",
        "posterUrl": "https://..."
      },
      "rating": 9,
      "likesCount": 12,
      "viewsCount": 100
    }
  ],
  "mediaObjects": [
    {
      "id": "uuid",
      "title": "Title",
      "type": "manga",
      "posterUrl": "https://..."
    }
  ]
}
```

Пустые секции — `[]`, не `null`.

Сортировка:

- `users` — точное совпадение ника выше, затем по username
- `reviews` — по лайкам, затем по дате
- `mediaObjects` — по `created_at` DESC

### Примеры

```http
GET /v1/search?q=@ann
GET /v1/search?q=маг&limit=20
Authorization: Bearer ...
```

---

## Ошибки

| HTTP | Когда |
|---|---|
| `401` | передан битый Bearer |
| `422` | нет/пустой `q`, слишком длинный `q`, неверный `tab` у trends (`details.field`) |

---

## Подсказки UI

1. Табы (`media` / `authors` / `types`) только у **trends** при пустом инпуте.
2. При вводе `q` — один экран с тремя секциями; пустые скрывать.
3. Плейсхолдер: `Поиск @ника, обзоров, тайтлов`.
4. Не парсить `@` на клиенте для API — бэкенд сам снимает.
5. `GET /v1/media-objects?q=` (catalog) — отдельный поиск каталога; для Explore — этот модуль.
