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
      "title": "Подписки",
      "text": "Уведомления о новых подписчиках",
      "enabled": true
    },
    {
      "name": "like",
      "title": "Лайки",
      "text": "Уведомления о лайках ваших обзоров",
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

Глобальный mute (`notificationsMuted`) перекрывает per-type prefs: даже при `enabled: true` пуш не уйдёт, пока mute включён (когда появятся пуши).
