# Feed API (для фронта)

Лента главной: опубликованные обзоры с бесконечным скроллом (keyset cursor).

## `GET /v1/feed`

### Query

| Параметр | Default | Значения | Заметки |
|---|---|---|---|
| `sort` | `new` | `new`, `views`, `likes` | новизна / просмотры / лайки |
| `mode` | `all` | `all`, `following` | `following` — только авторы из подписок |
| `type` | — | код типа тайтла (`anime_series`, `manga`, …) | опциональный фильтр `mediaObject.type` |
| `cursor` | — | opaque string | со следующей страницы; **не парсить** |
| `limit` | `20` | `1`–`50` | размер страницы |

### Auth

| mode | Auth |
|---|---|
| `all` | не нужен |
| `following` | `Authorization: Bearer <jwt>` обязателен, иначе `401` |

### Response `200`

Тот же list-item, что у профиля (`GET /v1/profiles/:id/reviews` / `GET /v1/profile/reviews`): без `blocks`, со счётчиками и `mediaObject`. Дополнительно объект `author` с ником TG.

```json
{
  "items": [
    {
      "id": "uuid",
      "authorId": "uuid",
      "author": {
        "id": "uuid",
        "username": "ann"
      },
      "mediaObject": {
        "id": "uuid",
        "title": "Title",
        "type": "anime_series",
        "posterUrl": "https://..."
      },
      "title": "Заголовок обзора",
      "rating": 8,
      "status": "published",
      "hasSpoilers": false,
      "likesCount": 12,
      "dislikesCount": 1,
      "viewsCount": 100
    }
  ],
  "nextCursor": "eyJzIjoibmV3Ii..."
}
```

- Всегда только `status: "published"` — фильтра published на клиенте нет.
- `nextCursor`: `string` или `null`. Если `null` — конец ленты.
- `author.username` — ник Telegram **без** `@`. Может быть `""` — UI fallback / плейсхолдер.
- `author.id` === `authorId` — `GET /v1/profiles/{id}`, обзоры, подписка. См. [api-social.md](api-social.md).

### Скролл / подгрузка

1. Первый запрос: без `cursor`, `limit=20` (или свой).
2. Пока `nextCursor != null` — следующий запрос с **тем же** `sort` / `mode` / `type` и `cursor=<nextCursor>`.
3. При смене `sort` / `mode` / `type` — сбросить список и `cursor`, запросить с нуля.
4. Не подставлять cursor от другого `sort` — бэкенд вернёт `422` (`field: cursor`).

### Ошибки

| HTTP | Когда |
|---|---|
| `401` | `mode=following` без/с битым JWT |
| `422` | неверный `sort` / `mode` / битый или чужой `cursor` (`details.field`) |

### Примеры

```http
GET /v1/feed
GET /v1/feed?sort=views&limit=20
GET /v1/feed?sort=likes&type=manga&cursor=...
GET /v1/feed?mode=following&sort=new
Authorization: Bearer ...
```
