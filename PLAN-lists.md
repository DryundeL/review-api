# План реализации: топы и подборки

Источник требований: `SPEC-lists.md`. План составлен по спеку и текущему коду;
код не менялся. Решения, принятые на уточнении, — в разделе «Принятые решения».

## Что выяснилось в коде

- Модуля `media` нет: загрузки, проверки magic bytes и S3 не реализованы.
  Статику `/storage/...` раздаёт внешний nginx из volume `/app/storage`
  (`compose.yaml`). Модуль `media` придётся написать с нуля, это отдельный срез.
- Модерации через администратора нет: обзоры снимаются автоматически по порогу
  жалоб (`create_report.go`). Для списков автоскрытие запрещено, нужен admin API.
- Причины жалоб (`report_reasons`) и типы (`review_types`) принадлежат `reviews`.
  Есть публичный метод `RequireActiveTypes`, а для причин его ещё нет.
- Каталог: в `media_objects` есть `type`, `title`, `poster_url` (внешний https).
  Публичного контракта «получить произведение по id» у `catalog` нет.
- Поиск по названию сделан через `ILIKE` с `escapeLike` (catalog, search).
  Курсорная пагинация — по образцу feed и sitemap.
- `ratelimit.Middleware(rdb, limit, window)` — нужно проверить, по какому ключу
  он считает запросы. Для квоты загрузок нужен ключ по пользователю.
- Последняя миграция — `00032`, новые начинаются с `00033`.

## Принятые решения

1. **Новый BC `media`, хранение на локальном диске.** Порт `ImageStore`,
   реализация пишет в `/app/storage/images/<uuid>.<ext>`, URL — `/storage/images/...`
   (модуль общий, поэтому префикс `images`, а не `lists`).
   S3 позже станет другой реализацией того же порта.
2. **Метаданные вырезаются без перекодирования.** JPEG: удалить APP1–APP15
   (EXIF, XMP, IPTC), оставить APP0 и ICC (APP2 `ICC_PROFILE`). PNG: удалить
   `tEXt`, `zTXt`, `iTXt`, `eXIf`, `tIME`. WebP: удалить чанки `EXIF` и `XMP `
   и сбросить флаги в `VP8X`. Размеры JPEG и PNG проверяются через
   `image.DecodeConfig`, WebP — разбором заголовка `VP8`/`VP8L`/`VP8X`.
   Новых зависимостей нет.
3. **Новый BC `lists`.** Зависит от `reviews` (типы, причины жалоб),
   `catalog` (произведения), `users` (публичное имя), `media` (изображения) —
   только через `contract` и публичные методы `module.go`.
4. **Опубликованный объект правится на клиенте до «Сохранить».** На сервере одна
   текущая версия. Черновик автосохраняется тем же `PUT`. У опубликованного
   клиент шлёт `PUT` только по кнопке. Риск: незаконченная правка опубликованного
   объекта теряется при закрытии вкладки; спек этого не запрещает.
5. **Модерация через admin API (`X-Admin-Token`).** Список открытых жалоб,
   скрытие и восстановление, закрытие жалобы. Закрытая жалоба перестаёт быть
   активной, после этого можно пожаловаться снова.
6. **Каталожный элемент можно переименовать и сменить ему фото.**
   `media_object_id` сохраняется. Сервер подставляет данные при добавлении
   и по действию «Обновить из каталога».
7. **Постер каталога копируется как строка URL**, без скачивания: нет SSRF.
   Риск: если внешний CDN удалит файл, картинка пропадёт.
8. **Квоты задаются в конфиге.** Загрузки: 120 в час на пользователя.
   Не удалённые шаблоны: до 50 на пользователя, результаты: до 200.
   Worker удаляет непривязанные загрузки старше 24 часов.

## Собственные решения (без вопроса, можно пересмотреть)

- Шаблон и результат удаляются мягко (`deleted_at`). Строка нужна,
  чтобы применять модерационное скрытие шаблона к его результатам и не ломать
  внешние ключи снапшотов. Для API удалённый объект — 404 для всех.
- Версии шаблона хранятся неизменяемыми снапшотами и создаются лениво:
  в момент создания результата, если для текущей `revision` снапшота ещё нет.
  Автосохранение черновика шаблона версий не порождает.
- У тира цвет только `#RRGGBB`. Лимит тела JSON-запроса — 128 КБ, ответ 413.
- Повторная загрузка картинки не дедуплицируется. Удалённые шаблоны картинки
  не освобождают: файлы, на которые ссылается хоть один снапшот, неизменяемы
  и не удаляются.
- Новые события в outbox не нужны: интеграции с лентой, геймификацией
  и уведомлениями спек исключает. Sitemap не трогаем.

## Модель данных

### media (миграция `00033_media_images.sql`)

`media_images`: `id`, `owner_id → users`, `storage_key` (уникальный),
`content_type` (`image/jpeg|png|webp`), `size_bytes ≤ 307200`, `width`, `height`,
`created_at`, `attached_at` (NULL — кандидат на GC).

### lists (миграция `00034_lists.sql`)

**Рабочее состояние шаблона**

- `list_templates`: `id`, `author_id`, `type_code → review_types`,
  `format` (`tiers|ranked|collection`, по умолчанию `tiers`), `title`,
  `description`, `status` (`draft|published`), `visibility` (`public|unlisted`,
  по умолчанию `public`), `revision`, `first_published_at`, `published_at`,
  `moderation_hidden_at`, `deleted_at`, `created_at`, `updated_at`.
  CHECK на длины в `char_length` (150 / 2000) — это защита на уровне БД;
  основная валидация остаётся в domain.
- `list_template_items`: `template_id`, `id`, `position`, `title`,
  `media_object_id → media_objects ON DELETE SET NULL`,
  `image_id → media_images` либо `image_url` (копия постера), `CHECK` ровно
  одного источника, если картинка задана. Частичный `UNIQUE (template_id,
  media_object_id) WHERE media_object_id IS NOT NULL`.
- `list_template_tiers`: `template_id`, `id`, `position`, `name`, `color`.

**Неизменяемые снапшоты**

- `list_template_versions`: `id`, `template_id`, `revision`
  (`UNIQUE (template_id, revision)`), `type_code`, `format`, `title`,
  `author_id`, `created_at`.
- `list_template_version_items`: `PRIMARY KEY (version_id, item_id)`,
  `position`, `title`, `media_object_id` (`ON DELETE SET NULL`),
  `image_id` или `image_url`.
- `list_template_version_tiers`: `PRIMARY KEY (version_id, tier_id)`,
  `position`, `name`, `color`.

**Результаты**

- `list_results`: `id`, `author_id`, `template_id`, `version_id`, `title`,
  `description`, `status`, `visibility`, `revision`, `first_published_at`,
  `published_at`, `moderation_hidden_at`, `deleted_at`, timestamps.
- `list_result_entries`: `result_id`, `item_id`, `tier_id NULL`, `position`.
  Составной FK `(version_id, item_id) → list_template_version_items`
  (для этого `version_id` хранится и в `entries`, плюс CHECK на совпадение
  с результатом через FK `(result_id, version_id) → list_results (id, version_id)`).
  Аналогично `(version_id, tier_id) → tiers`.
  `UNIQUE (result_id, item_id)`. Принадлежность элемента и тира шаблону
  гарантирует БД, а не только код.

**Жалобы**

- `list_reports`: `id`, `target_kind` (`template|result`), `target_id`,
  `reporter_id`, `reason_code → report_reasons`, `status` (`open|resolved`),
  `resolved_at`, `created_at`. Частичный `UNIQUE (target_kind, target_id,
  reporter_id) WHERE status='open'`.

**Индексы для разделов**

- `(first_published_at DESC, id DESC) WHERE status='published'
  AND visibility='public' AND deleted_at IS NULL AND moderation_hidden_at IS NULL`
  на шаблонах и результатах; фильтры по `type_code` и `format` поверх них.
  `ILIKE` без trgm, как в search; trgm — когда задержки станут заметны.
- `(author_id, updated_at DESC)` для «Мои».

## Правила видимости (одна функция в read-слое)

Объект доступен зрителю, если `deleted_at IS NULL` и выполнено одно из:

- зритель — автор, и объект не скрыт модератором (скрытый автор видит
  со статусом `hidden`, без публичного доступа — **уточнить на фронте**);
- `status='published'`, объект не скрыт модератором, а для результата
  ещё и шаблон не скрыт модератором (`JOIN list_templates t ... t.moderation_hidden_at IS NULL`,
  включая удалённые и черновые шаблоны).

В разделе и профиле дополнительно нужно `visibility='public'`.

Поля результата, зависящие от шаблона:

- `template.link` отдаётся, только если шаблон `published + public`,
  не удалён и не скрыт;
- `canCreateOwn` — по тому же условию;
- `template.title` и `author.displayName` берутся из снапшота и `users`
  всегда (без черновых данных).

Обложка вычисляется при чтении: первое место в `ranked`, первый элемент
верхнего непустого тира в `tiers`, первый по `position` в `collection`.
У шаблона обложка — первый элемент по `position`.

## Предлагаемый API

Префикс `/v1`. Гостевое чтение — с `optionalAuth`, запись — с `auth`.

| Маршрут | Назначение |
|---|---|
| `POST /media/images` (multipart, `auth`, квота) | Загрузка: 201 `{id, url, width, height}`; 413/415/422. |
| `GET /list-templates?q=&type=&format=&cursor=` | Вкладка «Шаблоны», только `public`. |
| `GET /list-results?q=&type=&format=&cursor=` | Вкладка «Топы пользователей». |
| `GET /profiles/:id/list-templates`, `/list-results` | Профиль, с `profileAccess`. |
| `GET /me/list-templates?status=`, `/me/list-results?status=` | «Мои»: `draft|published|unlisted|hidden`. `no-store`. |
| `POST /list-templates` `{typeCode, format?, title?}` | Черновик, 201. |
| `GET /list-templates/:id` | Просмотр; автору — полный, с `revision`. |
| `PUT /list-templates/:id` `{revision, title, description, typeCode?, format?, tiers[], items[]}` | Полная замена, 409 при конфликте ревизии, 422 при смене type/format после первой публикации. |
| `POST /list-templates/:id/items/:itemId/refresh-from-catalog` | Обновить название и постер из каталога. |
| `POST /list-templates/:id/publish` `{revision, visibility}` | Публикация, проверка полноты. |
| `POST /list-templates/:id/unpublish` | Возврат в черновик. |
| `PATCH /list-templates/:id/visibility` `{visibility}` | Только для опубликованных. |
| `POST /list-templates/:id/copy` | Новый черновик, 201. |
| `DELETE /list-templates/:id` | Мягкое удаление, 204. |
| `POST /list-templates/:id/results` | Новый черновик результата на снапшоте текущей ревизии. |
| `GET/PUT/DELETE /list-results/:id`, `POST …/publish`, `…/unpublish`, `PATCH …/visibility`, `POST …/copy` | То же для результатов. `PUT` принимает `{revision, title, description, entries[{itemId, tierId?, position}]}`. |
| `POST /list-templates/:id/reports`, `POST /list-results/:id/reports` `{reasonCode}` | 201, 409 при активной жалобе. |
| `GET /admin/list-reports?status=open&cursor=` | Admin. |
| `POST /admin/list-templates/:id/hide`, `/unhide`, то же для `list-results` | Admin. |
| `POST /admin/list-reports/:id/resolve` | Admin. |

«Составить свой» на странице результата — это `POST /list-templates/:templateId/results`;
`templateId` отдаётся, только если `canCreateOwn=true`. Сервер всё равно
проверяет условия сам.

Элемент в `PUT` шаблона: `{id?, mediaObjectId?, title, imageId?}`.

- Без `id` и с `mediaObjectId` — новый каталожный элемент. Сервер берёт
  название и постер из каталога и проверяет `type == template.type_code`
  и уникальность. `title`/`imageId` клиента применяются поверх, если переданы.
- С существующим `id` — правка. `mediaObjectId` неизменяем, `image_url` постера
  клиент задать не может: только `imageId` своей загрузки.
- Ручной элемент — `title` + `imageId`.
- `id` генерирует сервер; неизвестный `id` даёт 422.

## Валидация (domain)

- Строки: `strings.TrimSpace`, длина через `utf8.RuneCountInString`;
  управляющие символы запрещены. Лимиты: шаблон/результат 150, описание 2000,
  элемент 150, тир 30.
- Черновик: всё необязательно, но лимиты длин, ≤ 50 элементов, 1–10 тиров,
  цвет и уникальность действуют всегда.
- Публикация шаблона: непустое название, ≥ 1 элемента, у каждого название
  и картинка, у каждого тира название.
- Результат: каждый `itemId` есть в снапшоте, нет дублей. Для `tiers` нужен
  `tierId` из снапшота, позиции внутри тира 0..n−1. Для `ranked` `tierId`
  отсутствует, позиции 1..n без пропусков. Для `collection` позиции —
  только порядок. Публикация — ≥ 1 записи.
- `imageId` принадлежит текущему пользователю и существует. Чужая картинка
  допустима, только если она уже есть в этом шаблоне (копия, повторное сохранение).
- `typeCode` активен (`reviews.RequireActiveTypes`), `reasonCode` активен
  (новый метод `reviews.RequireActiveReportReason`).

## Шаги

Каждый шаг — отдельный зелёный коммит: `go test ./... && go vet ./... && go build ./cmd/...`.

### 1. media: загрузка изображений — ✅ сделано (ветка `feat/lists-media`)

- `internal/modules/media/{module.go, contract/, internal/{domain,application,infrastructure,delivery/http}}`.
- domain: `Sniff(data) (Format, error)` по magic bytes (`FFD8FF`, `\x89PNG\r\n\x1a\n`,
  `RIFF....WEBP`), `StripMetadata(format, data)`, проверка размеров
  (например, ≤ 4096×4096; уточнить на фронте). GIF и SVG отклоняются.
- delivery: `http.MaxBytesReader` на 300 КБ + запас на multipart; одна часть
  `file`; имя файла и `Content-Type` игнорируются.
- infrastructure: `LocalStore` пишет во временный файл в той же директории,
  делает `fsync` и `rename` (атомарно). Путь собирается только из `uuid`
  и расширения детектированного формата — path traversal исключён.
  Права `0644`, директория из конфига (`MEDIA_STORAGE_DIR`, `MEDIA_PUBLIC_PREFIX`).
- contract: `ImageRef{ID, URL, Width, Height}`, методы
  `RequireOwnedImages(ctx, owner, ids)` и `MarkAttached(ctx, ids)`.
- Квота загрузок: Redis-счётчик по `user_id` (адаптировать `platform/ratelimit`
  или сделать ключевую функцию).
- worker: GC непривязанных загрузок старше 24 часов (сначала строка, потом файл;
  если файл уже удалён, ошибкой это не считается).
- Тесты: каждый формат, GIF/SVG/HTML под видом `.jpg`, 300 КБ и 300 КБ + 1,
  обрезанный файл, EXIF с GPS вырезан (golden-файлы в `testdata`),
  повреждённые длины чанков и сегментов.
- **Проверить вручную:** nginx отдаёт `/storage/images/*` с корректным
  `Content-Type` и `X-Content-Type-Options: nosniff`. Конфиг nginx
  лежит вне репозитория, нужно согласовать.

### 2. Контракты существующих модулей — ✅ сделано

- `catalog`: `MediaObjectRef(ctx, id) (Ref{ID, Type, Title, PosterURL}, error)`.
- `reviews`: `RequireActiveReportReason(ctx, code)`.
- `users`: публичное имя для read-side берём JOIN'ом `users` в read-репозитории,
  как в feed. Если это нарушает договорённость «таблицы принадлежат модулю»
  для чтения — проверить, как делает feed, и следовать тому же.
- Обновить `CAPABILITY-MAP.md`: модули `media` и `lists`, их зависимости.

### 3. lists: миграция и domain — ✅ сделано

- Миграция `00034` (схема выше), down — `DROP` новых таблиц.
- domain: `Template` (`SaveDraft`, `Publish`, `Unpublish`, `SetVisibility`,
  `CanChangeTypeOrFormat`), `Result` (`SaveDraft`, `Publish`, …),
  `Format`, `Visibility`, `ComputeCover`.
- Unit-тесты на все правила валидации и границы (Unicode, emoji,
  пробелы по краям, 150/151, 50/51 элементов, 0/1/10/11 тиров).

### 4. lists: шаблоны (команды и запросы) — ✅ сделано

- Создание, сохранение с проверкой ревизии
  (`UPDATE … WHERE id=$1 AND author_id=$2 AND revision=$3`),
  публикация, снятие, видимость, удаление, копия, обновление из каталога.
- Сохранение — в одной TX: заменить `items` и `tiers` (`DELETE` + batch `INSERT`),
  поднять `revision`, отметить картинки привязанными.
- Квота шаблонов проверяется в TX, с `SELECT … FOR UPDATE` на строку
  пользователя или advisory lock по `author_id` (чтобы гонка не обошла лимит).
- Копия шаблона, скрытого модератором, запрещена (403 или 404 — выбрать
  единообразно, предлагаю 404).

### 5. lists: результаты — ✅ сделано

- Создание: шаблон `published`, не удалён, не скрыт; снапшот по текущей
  ревизии (`INSERT … ON CONFLICT (template_id, revision) DO NOTHING`, затем `SELECT`);
  название по умолчанию из шаблона.
- Сохранение: полная замена `entries`, валидация через domain + FK в БД.
- Публикация: запрещена, если шаблон скрыт модератором; удаление шаблона
  и снятие его с публикации не мешают.
- Копия: тот же `version_id`; запрещена, если шаблон скрыт модератором.

### 6. lists: разделы, профиль, «Мои» — ✅ сделано

- Keyset-курсор `(first_published_at, id)`, размер страницы 20 (уточнить).
- Фильтры `type` и `format`: для результатов берутся из снапшота.
- Поиск — `ILIKE` с `escapeLike`, ограничить длину `q` (например, 100 символов).
- Профиль: только `public`, через существующий `profileAccess`.

### 7. Жалобы и модерация

- Жалоба только на объект, доступный зрителю и опубликованный. Автор
  на свой объект жаловаться не может (по образцу обзоров — проверить).
- Admin: список жалоб с курсором; `hide`/`unhide` выставляют или очищают
  `moderation_hidden_at`; `resolve` закрывает жалобу.
- Аудит: `slog` с `admin action`, `target`, `id`, без пользовательских данных.
- Восстановление: прежние статус и видимость сохраняются сами,
  потому что скрытие — отдельное поле.

### 8. HTTP, wiring, документация

- `internal/app/modules.go`: подключить `media` и `lists`.
- `BodyLimit` 128 КБ на JSON-запись. `Cache-Control: no-store` на ответы
  автору и на «Мои».
- Ответы гостю не содержат `revision`, статуса черновика, `telegram_id`
  и других приватных полей.
- `docs/frontend-lists.md`: контракт, коды ошибок, рекомендация автосохранения
  (debounce ~1,5 с), правила `canCreateOwn` и ссылки на шаблон, превью.

## Тесты (критерии готовности из спека)

- Unit: domain lists, sniff и strip в media.
- HTTP (`*_test.go` рядом с handler'ами): гость получает 401 на запись;
  чужой результат или шаблон — 404 на `PUT`/`DELETE`; невалидный тип — 422;
  чужой `itemId` или `tierId`, дубликаты — 422; неверный формат картинки — 415;
  размер — 413; лимиты — 422; без `format` создаётся `tiers`.
- Интеграционные PostgreSQL (`TEST_DATABASE_URL`, по образцу
  `internal/http/routes/*_integration_test.go`):
  - снапшот не меняется после правки шаблона;
  - удаление произведения из каталога (`SET NULL`) сохраняет данные;
  - скрытие шаблона скрывает все результаты, включая старые версии,
    а `unhide` возвращает прежнее состояние;
  - удалённый или снятый шаблон не скрывает результаты, и черновик
    результата можно опубликовать;
  - `unlisted` нет в разделе и профиле, но он открывается по ссылке;
  - закрытый профиль не скрывает публикацию в разделе;
  - гонка квоты, конфликт ревизий, повторная жалоба.

Команды проверки:

```sh
go test ./internal/modules/media/... ./internal/modules/lists/...
go test ./...
go vet ./...
go build ./cmd/...
```

## Открыто (не блокирует старт, решить по ходу)

- Пути и домен публичных веб-страниц — согласовать с фронтом (открытый вопрос
  спека). API от этого не зависит.
- Что видит автор объекта, скрытого модератором: предлагаю показывать ему
  `moderation: hidden` в «Мои» и 404 на публичных маршрутах.
- Максимальные размеры изображения в пикселях и размер страницы раздела.
- Конфигурация внешнего nginx для `/storage/images` (`nosniff`, кеширование).

## Уточнения, принятые при реализации

- Файлы загрузок: `/storage/images/<uuid>.<ext>` (модуль `media` общий).
- Элемент шаблона хранит `image_url` всегда (URL загрузки из `media.AttachImages`
  или копию постера) и `image_id` только для загрузок. Чтение не обращается к `media`.
- `media.AttachImages` вызывается внутри транзакции сохранения шаблона: проверка владельца
  и пометка «привязано» коммитятся или откатываются вместе с шаблоном, а блокировки строк
  исключают гонку с GC.
- Колонка `published_at` не нужна: сортировка идёт по `first_published_at`.
- Позиции в результате начинаются с 1: место в топе, порядок внутри тира, порядок подборки.
  В каждой группе позиции строго 1..n; уникальность дублирует
  `UNIQUE NULLS NOT DISTINCT (result_id, tier_id, position)` (PostgreSQL 15+, в проде 17).
- Жалоба ссылается на цель двумя nullable-FK (`template_id` / `result_id`) с
  `CHECK num_nonnulls(...) = 1` вместо полиморфного `target_id`: работает ссылочная целостность.
- Ошибки валидации: `FieldError{Field, Index, Reason}`, где `Reason` — стабильный токен
  (`required`, `too_long`, `too_many`, `duplicate`, `immutable`, `invalid`, `type_mismatch`).
- Публикация, снятие и видимость ревизию не меняют; её поднимают только сохранение и
  обновление элемента из каталога. Публикация и обновление из каталога принимают `revision`
  (409 при расхождении), чтобы не затереть несохранённое состояние клиента.
- Элемент без `imageId` сохраняет текущую картинку. Повторно привязываются только новые загрузки,
  поэтому чужая картинка в копии шаблона не мешает сохранению.
- Новые каталожные элементы ищутся в каталоге до блокировки строки. Существующие перепроверяются
  на тип только при смене типа черновика.
- Обновление из каталога без постера оставляет текущую картинку. Элемент без ссылки на каталог
  (ручной или удалённый из каталога) даёт 409.
- Квота шаблонов: `pg_advisory_xact_lock` по автору + `COUNT` неудалённых; удалённые место освобождают.
- Автор видит свой шаблон, скрытый модератором (с `moderationHiddenAt`); остальным — 404.
  Удалённый шаблон — 404 всем, включая автора.
- Зависимости модуля `lists` — сами модули `catalog`, `media`, `reviews` через их публичные методы;
  перевод ошибок в `lists/module.go`. Квоты в конфиг выносятся на шаге 8.
- Результат создаётся по любому опубликованному и не скрытому шаблону, открытому зрителю
  (включая `unlisted`). Чужой недоступный шаблон — 404; свой черновик — 409; свой скрытый — `ErrModerated`.
- Шаблон при создании результата блокируется `FOR SHARE`: сохранение шаблона ждёт, пока его
  рабочее состояние копируется в снапшот. Снапшот создаётся `ON CONFLICT DO NOTHING` по
  `(template_id, revision)`, конкурентные создания получают одну версию.
- Публикация и копия результата читают флаг модерации шаблона под `FOR SHARE` (удалённый
  шаблон тоже), поэтому одновременное скрытие модератором упорядочено с ними.
- Квота результатов делит advisory lock автора с квотой шаблонов. Сохранение черновика
  результата при скрытом шаблоне разрешено: его видит только автор.
- `GetResult` отдаёт снапшот, `typeCode` и автора шаблона из снапшота, флаги
  `TemplateModerated` и `TemplatePublic` (основа для `template.link` и `canCreateOwn`).
- Списки: страница 20 по умолчанию, `limit` 1..50, иначе 422. Курсор — base64 JSON
  `{k, t, id}`, где `k` = сущность + область (`public`, `profile`, `mine:<status>`): курсор
  одной вкладки или фильтра статуса не принимается другой. Длина курсора ≤ 256.
- Раздел и профиль сортируются по `(first_published_at, id)`, «Мои» — по `(updated_at, id)`.
- «Мои», фильтр `status`: `draft`, `published` (= public), `unlisted`, `hidden`; пусто — все.
  Скрытый модератором объект (для результата — и при скрытом шаблоне) попадает только в `hidden`.
- Поиск: `q` ≤ 100 символов, без управляющих, `ILIKE` с `escapeLike`, как в catalog и search.
  Регистр кириллицы сравнивается по `LC_CTYPE` базы: в `postgres:17-alpine` из `compose.yaml`
  это `en_US.utf8` (проверено). Тестовая база должна иметь ту же локаль, не `C`.
- `type` только ограничивается по длине (≤ 64): неизвестный тип просто ничего не находит.
  Для результатов `type`, `format` и название шаблона берутся из снапшота.
- Обложка карточки считается в SQL по тем же правилам, что `CoverURL` в domain.
