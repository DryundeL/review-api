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
| `all` | опционален; с Bearer заполняется `author.isFollowing` |
| `following` | `Authorization: Bearer <jwt>` обязателен, иначе `401` |

### Response `200`

Без `status` (в ленте только published). Без `blocks`. Есть `author` с ником и флагом подписки.

```json
{
  "items": [
    {
      "id": "uuid",
      "authorId": "uuid",
      "author": {
        "id": "uuid",
        "username": "ann",
        "photoUrl": "https://t.me/i/userpic/320/....jpg",
        "isFollowing": true,
        "avatarFrame": "frame_gold",
        "backdrop": "list_dark"
      },
      "mediaObject": {
        "id": "uuid",
        "title": "Title",
        "type": "anime_series",
        "posterUrl": "https://..."
      },
      "title": "Заголовок обзора",
      "rating": 8,
      "hasSpoilers": false,
      "likesCount": 12,
      "dislikesCount": 1,
      "viewsCount": 100
    }
  ],
  "nextCursor": "eyJzIjoibmV3Ii..."
}
```

- `status` нет — только у своих обзоров на профиле (`/profile/reviews`).
- `author.isFollowing` — подписан ли текущий юзер на автора. Без auth → всегда `false`. На свои посты → `false`.
- `nextCursor`: `string` или `null`. Если `null` — конец ленты.
- `author.username` — ник Telegram **без** `@`. Может быть `""`.
- `author.photoUrl` — аватар Telegram CDN; может отсутствовать.
- `author.avatarFrame` / `author.backdrop` — ключи экипировки из магазина; могут отсутствовать.
- `author.id` === `authorId` — `GET /v1/profiles/{id}`, подписка. См. [api-social.md](api-social.md).

### Скролл / подгрузка

1. Первый запрос: без `cursor`, `limit=20` (или свой).
2. Пока `nextCursor != null` — следующий запрос с **тем же** `sort` / `mode` / `type` и `cursor=<nextCursor>`.
3. При смене `sort` / `mode` / `type` — сбросить список и `cursor`, запросить с нуля.
4. Не подставлять cursor от другого `sort` — бэкенд вернёт `422` (`field: cursor`).

### Ошибки

| HTTP | Когда |
|---|---|
| `401` | `mode=following` без/с битым JWT; или битый Bearer при `mode=all` |
| `422` | неверный `sort` / `mode` / битый или чужой `cursor` (`details.field`) |

### Примеры

```http
GET /v1/feed
GET /v1/feed?sort=views&limit=20
Authorization: Bearer ...
GET /v1/feed?sort=likes&type=manga&cursor=...
GET /v1/feed?mode=following&sort=new
Authorization: Bearer ...
```
