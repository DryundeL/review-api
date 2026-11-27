# Смешанные топы — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** шаблон топа с типом `"mixed"` принимает произведения каталога любых типов и ручные элементы.

**Architecture:** внутри модуля lists тип остаётся строкой, `domain.TypeMixed = "mixed"`. Только хранилище
переводит `"mixed"` ↔ NULL (`NULLIF` при записи, `COALESCE` при чтении), поэтому FK на `review_types`
сохраняется. Фильтр `type=mixed` превращается в `IS NULL` во вкладках lists и в ленте.

**Tech Stack:** Go, Echo, pgx/v5, goose.

**Spec:** `docs/superpowers/specs/2026-11-27-mixed-list-templates-design.md`

## Global Constraints

- Зарезервированное значение — ровно `mixed` (строчными).
- Смена типа разрешена только до первой публикации (существующее `ReasonImmutable`).
- Пользовательский ввод — только аргументами запроса, никакой конкатенации значений в SQL.
- Тесты: `set -a && . ./.env && set +a && TEST_DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@localhost:5432/review_test?sslmode=disable" go test -race ./...`
  (Postgres в контейнере `postgres`; если остановлен — `docker start postgres`, после тестов остановить). Креды не выводить.

## Review Focus

1. Неактивный тип в справочнике не должен блокировать смешанный шаблон: `RequireActive` для `mixed` не вызывается.
2. Переход смешанный → `manga` до публикации с произведением типа `anime_series` — 422 `items.mediaObjectId`/`mismatch` (Task 2).
3. Снимок версии смешанного шаблона пишется с NULL и читается как `"mixed"` в результате и в ленте (Task 3, 4).
4. `type=manga` не возвращает смешанные; `type=mixed` не возвращает обзоры в ленте (Task 3, 4).
5. Произведение удалено из каталога (`media_object_id` → NULL): элемент без `mediaType`, чтение не падает (Task 3).

---

### Task 1: Миграция

**Files:**
- Create: `migrations/00036_mixed_list_templates.sql`
- Test: `internal/modules/lists/internal/infrastructure/write/schema_integration_test.go` (дополнить `TestListsSchemaIntegration`)

- [ ] **Step 1: тест.** В `TestListsSchemaIntegration`:
  - `INSERT INTO review_types (code, label, sort_order) VALUES ('mixed', 'Смесь', 1)` → ошибка с constraint `review_types_code_reserved_chk`;
  - `INSERT INTO list_templates (id, author_id, type_code) VALUES (…, …, NULL)` проходит.
- [ ] **Step 2:** прогнать `go test -race -run TestListsSchemaIntegration ./internal/modules/lists/...` — FAIL.
- [ ] **Step 3: миграция.**
  - Up:
    - `ALTER TABLE list_templates ALTER COLUMN type_code DROP NOT NULL`;
    - то же для `list_template_versions`;
    - `ALTER TABLE review_types ADD CONSTRAINT review_types_code_reserved_chk CHECK (code <> 'mixed')`.
  - Down:
    - снять CHECK;
    - `SET NOT NULL` на обеих колонках. Если смешанные топы уже есть, Down упадёт. Это осознанно: удалять их молча нельзя.
  - Комментарий в шапке: NULL = смешанный тип.
- [ ] **Step 4:** тест PASS.
- [ ] **Step 5:** commit `feat(lists): allow templates without a type`.

### Task 2: Домен и команды

**Files:**
- Modify: `internal/modules/lists/internal/domain/template.go` (константа)
- Modify: `internal/modules/lists/internal/application/command/create_template.go:52`, `save_template.go:141,154,229-253`
- Test: `internal/modules/lists/internal/application/command/template_test.go`

**Interfaces:**
- Produces: `domain.TypeMixed = "mixed"` (string const), используется Task 3.

- [ ] **Step 1: тест `TestMixedTemplate`.** `fakeTypes` без ключа `mixed` (только `game`):
  - Create `TypeCode:"mixed"` → без ошибки, `tpl.TypeCode == "mixed"`;
  - Save с элементами: произведение `game`, произведение `movie`, ручной `manual("Пицца")` → без ошибки;
  - Save с `TypeCode: ptr("game")` при тех же элементах → `wantField(t, err, "items.mediaObjectId", "mismatch")`;
  - Save типизированного `game`-черновика с `TypeCode: ptr("mixed")` и произведением `movie` → без ошибки;
  - публикация смешанного проходит.
- [ ] **Step 2:** `go test -race -run TestMixedTemplate ./internal/modules/lists/internal/application/command/` — FAIL (`typeCode`/`invalid`).
- [ ] **Step 3: реализация.**
  - `RequireActive` вызывается только если тип `!= domain.TypeMixed` (create и save);
  - `checkWorkTypes` сразу возвращает nil для `domain.TypeMixed`.
- [ ] **Step 4:** PASS, плюс весь пакет command и domain.
- [ ] **Step 5:** commit `feat(lists): accept any catalog work in a mixed template`.

### Task 3: Хранилище lists, `mediaType`, фильтр

**Files:**
- Modify: `internal/modules/lists/internal/infrastructure/write/templates.go:49,73,126`, `write/results.go:43`
- Modify: `internal/modules/lists/internal/infrastructure/read/templates.go:31,63`, `read/results.go:26,66`, `read/lists.go:31,46,139`
- Modify: `internal/modules/lists/internal/domain/template.go:23` (`Item.MediaType string` — только для чтения, при записи игнорируется)
- Modify: `internal/modules/lists/internal/delivery/http/json.go:28,172` (`MediaType string \`json:"mediaType,omitempty"\``)
- Test: `internal/modules/lists/internal/infrastructure/write/lists_integration_test.go`

**Interfaces:**
- Consumes: `domain.TypeMixed`.
- Produces: в БД NULL ↔ в Go `"mixed"`; `domain.Item.MediaType`.

- [ ] **Step 1: тест `TestMixedTemplateIntegration`.** Опубликовать смешанный ranked-шаблон с произведениями `game` и `movie` и ручным элементом, создать и опубликовать результат.
  - `SELECT type_code FROM list_templates` и `list_template_versions` → NULL;
  - чтение шаблона и результата → `TypeCode == "mixed"`, элементы: `MediaType` `game`, `movie`, `""` у ручного;
  - `query.NewListHandler` с `Type:"mixed"` → оба (шаблон и результат) в своих вкладках, с `Type:"game"` → их нет;
  - `DELETE FROM media_objects` для `movie` → повторное чтение результата без ошибки, у элемента `MediaType == ""`.
- [ ] **Step 2:** FAIL.
- [ ] **Step 3: реализация.**
  - Запись: `NULLIF($n, 'mixed')` для `type_code`.
  - Чтение: `COALESCE(type_code, 'mixed')`.
  - Элементы: `LEFT JOIN media_objects mo ON mo.id = i.media_object_id`, `COALESCE(mo.type::text, '')`.
  - Фильтр в `read/lists.go`: при `q.TypeCode == domain.TypeMixed` — `typeCol + " IS NULL"`, иначе как сейчас.
  - `itemJSON` получает `mediaType`.
- [ ] **Step 4:** PASS + весь `./internal/modules/lists/...`.
- [ ] **Step 5:** commit `feat(lists): store mixed templates and expose item media types`.

### Task 4: Лента и документация

**Files:**
- Modify: `internal/modules/feed/internal/infrastructure/read/repository.go`:
  - `listBranch.selectSQL`: `COALESCE(lv.type_code, 'mixed')`;
  - `buildList`: фильтр типа делается выражением на ветку.
- Modify: `docs/frontend-lists.md`, `docs/frontend-feed.md`
- Test: `internal/http/routes/feed_lists_integration_test.go`

- [ ] **Step 1: тест.** В `TestFeedWithListsHTTPIntegration` опубликовать результат смешанного шаблона:
  - `?type=mixed` → только он, `list.typeCode == "mixed"`;
  - `?type=game` → без него;
  - `?kind=reviews&type=mixed` → пусто.
- [ ] **Step 2:** FAIL.
- [ ] **Step 3: реализация.** В `branch` заменить `typeCol` на `typeFilter func(arg string) string`:
  - у обзоров — `mo.type = $n`;
  - у топов — `lv.type_code = $n`;
  - при `p.Type == "mixed"`: у топов `lv.type_code IS NULL` (без аргумента), у обзоров — `FALSE`.

  Значение `"mixed"` — константа в feed ports (`ports.TypeMixed`), не импорт из lists.
- [ ] **Step 4:** PASS + полный прогон `go test -race ./...`.
- [ ] **Step 5: документация.**
  - `frontend-lists.md`:
    - `typeCode` принимает `"mixed"`;
    - в смешанном шаблоне разрешены любые произведения;
    - у элементов новое поле `mediaType`;
    - фильтр `type=mixed`.
  - `frontend-feed.md`: `type=mixed` и `list.typeCode` может быть `"mixed"`.
- [ ] **Step 6:** commit `feat(feed): filter mixed list results` и `docs: describe mixed list templates`.
