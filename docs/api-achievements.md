# Achievements (для фронта)

Справочник ачивок + прогресс пользователя. Иконки живут на фронте по текстовому `code` (как shop cosmetics).

## Профиль

### `GET /v1/profiles/{id}/achievements`

Auth: Bearer **опционален**.

```json
{
  "achievements": [
    {
      "code": "first_review",
      "title": "First review",
      "description": "Publish your first review",
      "progress": 100,
      "current": 1,
      "target": 1,
      "unlockedAt": "2026-10-20T12:00:00Z"
    }
  ]
}
```

| Поле | Смысл |
|---|---|
| `code` | ключ иконки на фронте |
| `progress` | 0–100, `min(100, current * 100 / target)` |
| `unlockedAt` | RFC3339 UTC; omit если ещё не открыта |

## Засчитывание

| Событие | Метрики |
|---|---|
| публикация обзора | `reviews_published`, `blocks_written`, `reviews_type_{mediaType}`, `types_explored` (при первом типе) |
| лайк | `likes_given` (кто лайкнул), `likes_received` (автор) |
| подписка | `followers` (на кого подписались) |
| покупка в магазине | `shop_purchases` |
| начисление очков (не за ачивку) | `points_earned` |

При 100%: unlock + `rewardPoints` в баланс + outbox `unlock.granted` (пуши — позже).

## Админ

Заголовок `X-Admin-Token`.

| Метод | Путь |
|---|---|
| `GET` | `/v1/admin/achievements` |
| `POST` | `/v1/admin/achievements` |
| `PATCH` | `/v1/admin/achievements/:id` |

`POST` body:

```json
{
  "code": "first_review",
  "title": "First review",
  "description": "Publish your first review",
  "metric": "reviews_published",
  "target": 1,
  "rewardPoints": 5,
  "sortOrder": 10,
  "isActive": true
}
```

`PATCH`: `title`, `description`, `target`, `rewardPoints`, `sortOrder`, `isActive` (частично). `code` / `metric` не меняются.
