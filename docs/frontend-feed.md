# Лента: контракт для фронтенда

`GET /v1/feed` — общая лента. В ней смешаны опубликованные **обзоры** и публичные **результаты топов**
(тир-листы, топы, подборки). Шаблонов в ленте нет.

Вход необязателен. С `Authorization: Bearer …` в карточках приходят `isFollowing` и `viewerIsLike`.
Для `mode=following` вход обязателен, без него — 401.

## Параметры

| Параметр | Значения | По умолчанию |
|---|---|---|
| `kind` | `all`, `reviews`, `lists` | `all` |
| `sort` | `new`, `views`, `likes` | `new` |
| `mode` | `all`, `following` (только авторы, на которых подписан пользователь) | `all` |
| `type` | код типа (`manga`, …): у обзора — тип произведения, у топа — тип шаблона | любой |
| `limit` | 1–50 | 20 |
| `cursor` | `nextCursor` из предыдущего ответа | — |

- **Сортировки.** `new` — новые первыми. Время обзора — создание, время топа — первая публикация.
  `views` и `likes` — по счётчику, при равенстве — новые первыми. Обзоры и топы сортируются вместе.
- **Курсор.** Привязан к `sort`: курсор другой сортировки — 422 `cursor`. При смене любого параметра
  начинайте без `cursor`. Если `nextCursor` равен `null`, это последняя страница.
- **Неверное значение** `kind`, `sort` или `mode` — 422 `VALIDATION_ERROR`, `details.field` — имя параметра.
- **Какие топы попадают:** опубликованные, `public`, не удалённые и не скрытые модератором
  (ни сам результат, ни его шаблон). `unlisted` в ленту не попадает.

## Ответ

```ts
interface FeedPage { items: FeedItem[]; nextCursor: string | null }

interface FeedItem {
  kind: "review" | "list";
  id: string;              // id обзора или результата топа
  authorId: string;
  author: {
    id: string; username: string; photoUrl?: string; isFollowing: boolean;
    avatarFrame?: string; backdrop?: string;
  };
  title: string;
  likesCount: number;
  dislikesCount: number;
  viewsCount: number;
  viewerIsLike?: boolean;  // true — лайк, false — дизлайк, нет поля — не голосовал или гость
  hasSpoilers: boolean;    // у топов всегда false

  // только kind = "review"
  mediaObject?: { id: string; title?: string; type?: string; posterUrl?: string };
  rating?: number;

  // только kind = "list"
  list?: {
    typeCode: string;
    format: "tiers" | "ranked" | "collection";
    coverUrl?: string;     // считает сервер, правила — в frontend-lists.md, раздел 4
    templateTitle: string;
  };
}
```

**Навигация по карточке:** `review` ведёт на страницу обзора (`GET /reviews/:id`), `list` — на страницу
результата (`GET /list-results/:id`, [frontend-lists.md](frontend-lists.md), раздел 3.3).

## Что изменилось

- **Новое поле `kind`** в каждой карточке. **Рендерите карточку по `kind`.** Без фильтра
  в ленте теперь приходят и топы. У них нет `mediaObject` и `rating`.
- **`rating`** приходит только у обзоров.
- **Только обзоры:** `kind=reviews` даёт прежнюю выдачу.
- **Реакции топов** описаны в [frontend-lists.md](frontend-lists.md), раздел 3.8.
