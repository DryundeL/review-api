# Settings + notification prefs (для фронта)

Настройки приложения и per-type уведомления. Auth: **Bearer обязателен**.

`GET /v1/profile` → `languageCode` — снимок языка Telegram.
`GET /v1/settings` → `language` — локаль Mini App (меняется пользователем).

## Настройки

### `GET /v1/settings`

```json
{
  "language": "ru",
  "notificationsMuted": false
}
```

### `PATCH /v1/settings`

Частичный body. Пустой body возвращает текущие значения.

```json
{
  "language": "en",
  "notificationsMuted": true
}
```

| Поле | Правила |
|---|---|
| `language` | 2–16 символов `[A-Za-z0-9-]`, как Telegram `language_code` |
| `notificationsMuted` | глобальный mute всех пушей |

**422** — битый `language` (`field: language`).

При первом EnsureUser (логин / бот) создаётся строка settings: `language` = язык Telegram (или `en`), `notificationsMuted` = false. Повторный EnsureUser язык **не** перезаписывает.

Бот `/mute` / `/unmute` пишет в то же поле `notificationsMuted`.

---

## Уведомления по типам

Справочник `notification_types`: `like`, `follow` (title/text).
Prefs — одна строка на юзера с колонками `like_enabled` / `follow_enabled` (defaults lazy при первом GET/PATCH).

### `GET /v1/notification-settings`

```json
{
  "items": [
    {
      "name": "follow",
      "title": "Подписка",
      "text": "Новый подписчик",
      "enabled": true
    },
    {
      "name": "like",
      "title": "Лайк",
      "text": "Новый лайк на ваш обзор",
      "enabled": true
    }
  ]
}
```

### `PATCH /v1/notification-settings`

```json
{
  "items": [
    { "name": "like", "enabled": false }
  ]
}
```

Ответ — полный список после патча (как GET).

**422** — пустой `items`, unknown / duplicate `name` (`field: items` или `name`).

---

## Inbox

Правила доставки:

| Pref | Результат |
|---|---|
| type `enabled: false` | ничего: ни inbox, ни бот |
| type on + `notificationsMuted: true` | строка в inbox, бот молчит |
| type on + mute off | inbox + сообщение от бота |

Текст: `notification_types.text` + динамика (`: @user · «обзор»`).

### `GET /v1/notifications?limit=20&offset=0`

```json
{
  "items": [
    {
      "id": "uuid",
      "type": "like",
      "title": "Лайк",
      "body": "Новый лайк на ваш обзор: @ann · «Neon»",
      "actorId": "uuid",
      "reviewId": "uuid",
      "isRead": false,
      "createdAt": "2026-10-18T12:00:00Z"
    }
  ]
}
```

### `GET /v1/notifications/unread`

```json
{ "count": 3 }
```

### `PATCH /v1/notifications/{id}/read`

**204** — отметить одно прочитанным.

### `PATCH /v1/notifications/read`

**204** — отметить все прочитанными.
