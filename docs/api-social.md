# Social + профиль (для фронта)

Подписки и карточка профиля. Всё по внутреннему UUID (`authorId` / `author.id`).

## Поток: лента → профиль → подписка

```
1. GET /v1/feed
   → authorId / author.id (uuid), author.username

2. GET /v1/profiles/{userId}          # чужой/свой по uuid (Bearer опционален)
   или GET /v1/profile                # только свой (Bearer обязателен)

3. GET /v1/profiles/{userId}/reviews?limit=20
4. GET /v1/profiles/{userId}/followers?limit=20&offset=0
   GET /v1/profiles/{userId}/following?limit=20&offset=0

5. PUT /v1/follows/{userId}           # подписаться (Bearer)
   DELETE /v1/follows/{userId}        # отписаться (Bearer)

6. GET /v1/feed?mode=following        # лента подписок (Bearer)
```

---

## Профиль

### `GET /v1/profile` (свой)

Auth: **Bearer обязателен**.

```json
{
  "id": "uuid",
  "username": "ann",
  "firstName": "Ann",
  "lastName": "Lee",
  "photoUrl": "https://t.me/i/userpic/320/....jpg",
  "followersCount": 12,
  "followingCount": 5,
  "isFollowing": false,
  "isMe": true,
  "telegramId": 42,
  "languageCode": "ru",
  "isPremium": false,
  "email": "user@example.com"
}
```

### `GET /v1/profiles/{userId}` (по UUID)

Auth: **опционален**. Если есть Bearer и `{userId}` = я → `isMe: true` и те же приватные поля, что у `/profile`.

Чужой (с Bearer):

```json
{
  "id": "uuid",
  "username": "ann",
  "firstName": "Ann",
  "lastName": "Lee",
  "photoUrl": "https://t.me/i/userpic/320/....jpg",
  "followersCount": 12,
  "followingCount": 5,
  "isFollowing": true,
  "isMe": false
}
```

| Поле | Смысл |
|---|---|
| `photoUrl` | ссылка на аватар в Telegram CDN (из initData); может отсутствовать |
| `followersCount` | сколько подписчиков |
| `followingCount` | на скольких подписан |
| `isFollowing` | подписан ли **текущий** юзер на этого (без auth / на себя → `false`) |
| `isMe` | это мой профиль |
| `email` | только свой профиль; нет поля — почта не привязана. Привязка и вход: [api-email.md](api-email.md) |

**404** — нет юзера / битый uuid.

Список подписчиков/подписок на карточке нет — только счётчики. Списки: `GET /v1/profiles/{id}/followers|following`.

---

## Подписки

### Подписаться / отписаться

Auth: **Bearer обязателен**.

```http
PUT /v1/follows/{userId}
DELETE /v1/follows/{userId}
```

| Код | Когда |
|---|---|
| `204` | ок |
| `401` | нет/битый JWT |
| `409` | уже подписан (PUT) |
| `404` | подписки не было (DELETE) |
| `422` | сам на себя / битый uuid / юзера нет |

После успешного PUT на UI: `isFollowing = true`, `followersCount++`.

### Списки подписок / подписчиков

Как и reviews — вложены в профиль (не `/users`: в API ресурс называется `profiles`).

```http
GET /v1/profiles/{id}/followers?limit=20&offset=0
GET /v1/profiles/{id}/following?limit=20&offset=0
```

| Параметр | Default | Заметки |
|---|---|---|
| `limit` | `20` | `1`–`50` |
| `offset` | `0` | `≥ 0`, макс. `10000` |

Auth: **опционален**. С Bearer заполняется `isFollowing` (подписан ли **текущий** юзер на человека в строке). Без auth / на себя в списке → `false`.

Свой список: `id` из `GET /v1/profile`.

```json
{
  "items": [
    {
      "userId": "uuid",
      "username": "ann",
      "photoUrl": "https://t.me/i/userpic/320/....jpg",
      "isFollowing": true,
      "createdAt": "2026-10-17T12:00:00Z"
    }
  ]
}
```

| Код | Когда |
|---|---|
| `200` | ок |
| `404` | профиля нет |
| `422` | битый `id` / `limit` / `offset` |

### Лента подписок

```http
GET /v1/feed?mode=following&sort=new
Authorization: Bearer ...
```

См. [api-feed.md](api-feed.md).

---

## UI

1. Экран профиля всегда можно открыть как `/profiles/{userId}`; если `isMe` — свой UI (кнопка подписки скрыта).
2. «Подписаться» → смотри `isFollowing`; `PUT` / `DELETE` по `userId`.
3. Повторный `PUT` → `409` — уже подписан.
