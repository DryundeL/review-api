# Gamification + shop (для фронта)

Очки за активность, Steam-lite магазин косметики. Ассеты (картинки рамок/фонов) живут на фронте по текстовому `code`.

## Начисление очков

| Событие | Кому | Формула | ENV |
|---|---|---|---|
| публикация обзора | автор | `POINTS_REVIEW_PER_BLOCK × blockCount` | `POINTS_REVIEW_PER_BLOCK` (default 10) |
| лайк | кто лайкнул | `POINTS_LIKE_GIVEN` | default 1 |
| лайк | автор обзора | `POINTS_LIKE_RECEIVED` | default 2 |
| подписка | на кого подписались | `POINTS_FOLLOWER` | default 5 |

Self-like не даёт очков. Unlike / unfollow очки не забирают. Правка блоков после публикации не пересчитывает награду.

## Каталог

### `GET /v1/shop/items`

Только активные позиции. Ответ кэшируется в Redis (`gamification:shop:active`, TTL 5m); admin create/patch инвалидирует ключ.

```json
{
  "items": [
    {
      "id": "uuid",
      "code": "frame_gold",
      "kind": "avatar_frame",
      "title": "Gold frame",
      "price": 50,
      "sortOrder": 10
    }
  ]
}
```

`kind`: `background` | `avatar_frame` | `backdrop`.

## Кошелёк / инвентарь

Auth: **Bearer**.

### `GET /v1/gamification/me`

```json
{
  "pointsBalance": 120,
  "ownedCodes": ["frame_gold"],
  "equipped": {
    "background": "bg_stars",
    "avatarFrame": "frame_gold",
    "backdrop": "list_dark"
  }
}
```

### `POST /v1/shop/purchases`

```json
{ "itemId": "uuid" }
```

| Код | Когда |
|---|---|
| `204` | куплено |
| `404` | нет / неактивен |
| `409` `INSUFFICIENT_POINTS` | мало очков |
| `409` `CONFLICT` | уже куплено |

### `PUT /v1/gamification/equipped`

Частичное обновление. Omit — не трогать; `null` или `""` — снять; строка — надеть (только из `ownedCodes`).

```json
{ "avatarFrame": "frame_gold", "background": null }
```

| Код | Когда |
|---|---|
| `200` | ок, тело = текущий `equipped` |
| `422` `NOT_OWNED` | код не куплен |

## Профиль

В `GET /v1/profile` и `GET /v1/profiles/{id}` публично:

| Поле | Смысл |
|---|---|
| `background` | ключ фона (опционально) |
| `avatarFrame` | ключ рамки |
| `backdrop` | ключ подложки списка |

Баланс и owned на чужой карточке **не** отдаются.

В ленте и списках/карточке обзора те же `author.avatarFrame` и `author.backdrop` (без `background`).

## Админ

Заголовок `X-Admin-Token`.

| Метод | Путь |
|---|---|
| `GET` | `/v1/admin/shop-items` |
| `POST` | `/v1/admin/shop-items` |
| `PATCH` | `/v1/admin/shop-items/:id` |

`POST` body:

```json
{
  "code": "frame_gold",
  "kind": "avatar_frame",
  "title": "Gold frame",
  "price": 50,
  "sortOrder": 10,
  "isActive": true
}
```

`PATCH`: `title`, `price`, `sortOrder`, `isActive` (частично).
