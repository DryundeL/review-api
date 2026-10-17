# Social + профиль (для фронта)

Подписки и карточка профиля. Всё по внутреннему UUID (`authorId` / `author.id`).

## Поток: лента → профиль → подписка

```
1. GET /v1/feed
   → authorId / author.id (uuid), author.username

2. GET /v1/profiles/{userId}          # чужой/свой по uuid (Bearer опционален)
   или GET /v1/profile                # только свой (Bearer обязателен)

3. GET /v1/profiles/{userId}/reviews?limit=20

4. PUT /v1/follows/{userId}           # подписаться (Bearer)
   DELETE /v1/follows/{userId}        # отписаться (Bearer)

5. GET /v1/feed?mode=following        # лента подписок (Bearer)
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
  "followersCount": 12,
  "followingCount": 5,
  "isFollowing": false,
  "isMe": true,
  "telegramId": 42,
  "languageCode": "ru",
  "isPremium": false
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
  "followersCount": 12,
  "followingCount": 5,
  "isFollowing": true,
  "isMe": false
}
```

| Поле | Смысл |
|---|---|
| `followersCount` | сколько подписчиков |
| `followingCount` | на скольких подписан |
| `isFollowing` | подписан ли **текущий** юзер на этого (без auth / на себя → `false`) |
| `isMe` | это мой профиль |

**404** — нет юзера / битый uuid.

Список подписчиков/подписок на карточке нет — только счётчики. Списки: `GET /follows/followers|following` (свои).

---

## Подписки

Все эндпоинты ниже требуют `Authorization: Bearer <jwt>`.

### Подписаться

```http
PUT /v1/follows/{userId}
```

| Код | Когда |
|---|---|
| `204` | ок |
| `401` | нет/битый JWT |
| `409` | уже подписан |
| `422` | сам на себя / битый uuid / юзера нет |

После успеха на UI: `isFollowing = true`, `followersCount++`.

### Отписаться

```http
DELETE /v1/follows/{userId}
```

| Код | Когда |
|---|---|
| `204` | ок |
| `404` | подписки не было |
| `401` | нет JWT |

### Списки (свои)

```http
GET /v1/follows/following?limit=20
GET /v1/follows/followers?limit=20
```

```json
{
  "items": [
    { "userId": "uuid", "createdAt": "2026-10-17T12:00:00Z" }
  ]
}
```

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
