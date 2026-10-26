# review-api

Telegram Mini App backend. Modular monolith: форма модуля как `analytic/project`, стек — Go 1.27, Echo, pgx, sqlc (позже), Goose, Redis, slog.

Карта: [CAPABILITY-MAP.md](CAPABILITY-MAP.md).

## Docker и сервер

Настройка пользователя `wapp`, переменных окружения, запуск и откат:
[deploy/README.md](deploy/README.md). Конфигурация сервисов — `compose.yaml`.
Push в `main` автоматически запускает проверки, сборку образа и деплой
через GitHub Actions. В pull request выполняются только проверки и сборка.

## Запуск

```
cp .env.example .env   # DB_*, REDIS_*, TELEGRAM_BOT_TOKEN, TELEGRAM_BOT_USERNAME, JWT_SECRET
go run ./cmd/migrator -command=up
go run ./cmd/api
go run ./cmd/worker
go run ./cmd/bot
```

- `GET /healthz` — процесс жив
- `GET /readyz` — PostgreSQL + Redis
- `GET /metrics` — Prometheus
- `POST /v1/auth/telegram` — Mini App, `Authorization: tma <initData>`, сразу `token`
- `POST /v1/auth/login` — вход из браузера, в ответе `url` на бота
- `POST /v1/auth/login/poll` — `{ "loginId" }`, пока не придёт `token`
- `GET /v1/profile` — `Authorization: Bearer <jwt>`
- `GET /v1/auth/identity` — `Authorization: tma <initData>`
- `GET /v1/review-types` — активные типы обзоров
- `GET /v1/review-statuses` — фиксированные статусы публикации и релиза
- `GET /v1/admin/review-types` — все типы, заголовок `X-Admin-Token`
- `POST /v1/admin/review-types` — добавить тип, тот же заголовок
- `PATCH /v1/admin/review-types/:code` — подпись, порядок, `isActive`, тот же заголовок
- `GET /v1/media-objects?q=&type=&limit=` — тайтлы каталога (поиск по `title` и `originalTitle`)
- `GET /v1/media-objects/:id` — общая информация о произведении, включая `info`
- `GET /v1/media-objects/:id/top-reviews` — до 5 опубликованных обзоров по лайкам (убывание); при равенстве — `created_at DESC, id DESC`. Ответ `{ "items": [...] }` в формате списка обзоров; без обзоров — пустой массив, неизвестный UUID — 404. Авторизация необязательна; с Bearer возвращается голос пользователя. Параметры пагинации не применяются.
- `POST /v1/admin/media-objects/import` — пакетный импорт JSON или CSV, заголовок `X-Admin-Token`
  - JSON: `{ "items": [{ "title": "Матрица", "originalTitle": "The Matrix", "type": "movie", "info": { "genre": "sci-fi", "releaseYear": 1999, "releaseStatus": "completed" } }] }`. Поля `posterUrl`, `externalSource`, `externalId` остаются на верхнем уровне.
  - CSV header: `title,original_title,type,info,poster_url,external_source,external_id`; обязательны `title,type,info`. В `info` — JSON-объект с экранированием по правилам CSV: `"{""genre"":""sci-fi"",""releaseYear"":1999,""releaseStatus"":""completed""}"`.
  - Старый формат импорта (JSON: `genre`, `releaseYear`, `releaseStatus`; CSV: `genre`, `release_year`, `release_status`) временно принимается. Нельзя смешивать `info` с непустыми прежними полями метаданных в одной записи.
  - В ответах каталога и импорта метаданные возвращаются только внутри `info`: `genre` (опциональная строка до 100 символов), `releaseYear` (опциональное целое 1800–2100), `releaseStatus` (обязательное: `announced`, `ongoing`, `completed`, `hiatus`). Это изменение контракта ответа для клиентов.
  - Миграция `00020` переносит существующие метаданные в `media_objects.info` (JSON NOT NULL) и удаляет прежние колонки; откат восстанавливает их. Применяйте миграцию вместе с новой версией API: старая версия с новой схемой несовместима.

## Доступ и приватность профиля

Просмотр опубликованных обзоров (`GET /v1/reviews/:id`), каталога, общей ленты и публичных профилей доступен без токена. Переданный Bearer проверяется; некорректный токен возвращает 401. Черновики видит только автор. Создание и изменение обзоров, голоса, жалобы, подписки, личный профиль, настройки, уведомления и покупки требуют авторизации. Административные маршруты сохраняют отдельную защиту.

Для генерации sitemap доступен публичный `GET /v1/sitemap/:entity` (`media-objects`, `reviews`, `authors`) с курсорной пагинацией всех ID. Черновики и закрытые профили исключены. Контракт и правила обхода описаны в [изменениях API для фронтенда](docs/frontend-api-changes.md#данные-для-sitemap).

`GET /v1/settings` возвращает `isProfilePrivate`; изменение: `PATCH /v1/settings` с `{"isProfilePrivate":true}`. По умолчанию профиль публичный. Страница приватного профиля и `/profiles/:id/reviews`, `/following`, `/followers`, `/achievements` доступны владельцу и одобренным подписчикам; остальным возвращается 404. Опубликованные обзоры остаются в общих лентах и доступны напрямую. Личные email и Telegram ID возвращаются только владельцу. Существующие подписчики сохраняются при включении приватности.

## Запросы на подписку

Все пути ниже начинаются с `/v1`. Изменения и входящие запросы требуют Bearer.

- `PUT /follows/:userId` — публичный профиль: 200, `{"followStatus":"following","isFollowing":true}`; приватный: создаёт запрос, 202, `{"followStatus":"pending","isFollowing":false}`. Это изменение прежнего ответа 204. Повторная подписка или дублирующий запрос возвращает 409.
- `POST /profiles/:id/follow-requests` — тот же механизм подписки: приватному профилю отправляет запрос, публичному создаёт подписку сразу.
- `GET /profiles/:id/follow-status` — `{"followStatus":"none|pending|following","isFollowing":false}`. Значение `isFollowing` равно true только для `following`. Просмотр без Bearer возвращает `none`; маршрут доступен даже при закрытом профиле и раскрывает только отношение текущего пользователя к нему. Неизвестный профиль — 404.
- `GET /profile/follow-requests?limit=20&offset=0` — входящие запросы текущему пользователю: `{"items":[{"userId":"UUID отправителя","username":"...","createdAt":"..."}]}` (также возвращаются `photoUrl`, если задан, и `isFollowing`). Максимальная выборка — 50, максимальный offset — 10000.
- `PUT /profile/follow-requests/:userId` — владелец принимает запрос отправителя, 204. Подписка и событие `user.followed` создаются атомарно; повторное принятие — 404.
- `DELETE /profile/follow-requests/:userId` — владелец отклоняет запрос, 204; подписка не создаётся.
- `DELETE /follows/:userId` — отправитель отменяет ожидающий запрос или удаляет свою подписку, 204; если отношения нет — 404.

`followStatus` также присутствует в ответах профиля вместе с совместимым `isFollowing`. Ожидающие запросы не входят в счётчики подписчиков, персональную ленту и начисление баллов за подписку. При открытии профиля ожидающие запросы автоматически не принимаются: отправитель может повторить подписку, либо владелец принять запрос. После отклонения или отмены статус — `none`.

Новый запрос создаёт событие `user.follow_requested` в той же транзакции. Worker создаёт владельцу уведомление типа `follow_request` («Запрос на подписку») с `actorId` отправителя. Оно доступно через `GET /v1/notifications`; действие принятия выполняется существующим `PUT /v1/profile/follow-requests/:userId`. Дублирующий запрос не порождает новое событие, повторная обработка одного события не дублирует уведомление или сообщение Telegram.

Тип `follow_request` включён по умолчанию и доступен в `GET /v1/notification-settings`. Отключение: `PATCH /v1/notification-settings` с `{"items":[{"name":"follow_request","enabled":false}]}` — отключает этот тип полностью. Общий `allowNotifications=false` в `/v1/settings` отключает только отправку в Telegram, сохраняя уведомление в приложении. Для доставки работает существующий worker; прямых отправок из HTTP-запроса нет.

Перед запуском примените также `00023_follow_request_notifications.sql` (новый тип и его настройка). Откат удаляет уведомления этого типа и его настройку.

Перед запуском примените миграции `00021` (настройка приватности) и `00022` (запросы). Откат `00022` удаляет ожидающие запросы, сохраняя принятые подписки. Новая версия API требует обе миграции.

```
go test ./...
go build ./cmd/api ./cmd/worker ./cmd/bot ./cmd/migrator
```

Интеграционные проверки используют отдельные временные схемы PostgreSQL:

```sh
TEST_DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/TEST_DB?sslmode=disable' go test -race ./...
```
