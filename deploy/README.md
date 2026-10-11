# Деплой от пользователя wapp

Каталог проекта: `/home/wapp/apps/review-api`. Требуются Docker Engine,
Docker Compose с поддержкой `up --wait`, Linux `flock` и доступ к registry.

## Автодеплой

Каждый push в `main` запускает `.github/workflows/deploy.yml`:
проверка форматирования, `go vet`, тесты с race detector, сборка Docker-образа
на GitHub, доставка по SSH и запуск с проверкой готовности.
Pull request запускает проверки и сборку без доступа к серверу.
Также доступен ручной запуск workflow для ветки `main`.

В GitHub Actions Secrets используются `DEPLOY_SSH_KEY` (отдельный ключ для
`wapp`) и `DEPLOY_KNOWN_HOSTS` (проверенный SSH host key сервера).
Секреты приложения остаются в `/home/wapp/apps/review-api/.env`.
Доступ к изменению `main` и workflow означает доступ к деплою приложения.

Образы имеют тег полного SHA коммита. Конфигурация каждого релиза хранится в
`/home/wapp/apps/review-api/releases/<SHA>`, `current` указывает на последний
успешно запущенный релиз. Деплои из GitHub выполняются последовательно;
общая блокировка также защищает от одновременного ручного обновления.

После включения CI не запускайте старую копию проекта из корневого каталога
для обновления production: сделайте push в `main`.
Для проверки работающего релиза:

```sh
cd /home/wapp/apps/review-api/current
APP_VERSION=$(cat .release-version) docker compose ps
curl --fail http://127.0.0.1:8080/readyz
```

При неудачной проверке готовности workflow завершается ошибкой. Автоматический
откат схемы БД не выполняется. Для отката на сохранённый совместимый релиз:

```sh
cd /home/wapp/apps/review-api/releases/<PREVIOUS_SHA>
DOCKER_CONTEXT=rootless DEPLOY_LOCK_FILE=/home/wapp/apps/review-api/.deploy.lock sh deploy/deploy.sh --no-build
ln -sfn "$PWD" /home/wapp/apps/review-api/current
```

Не удаляйте образы, нужные для отката. Старые релизы и образы периодически
удаляйте после проверки резервных копий и стабильности обновления.

## Доступ к Docker

Предпочтителен rootless Docker. Администратор устанавливает `uidmap` и
`dbus-user-session`, включает `loginctl enable-linger wapp`. Затем в SSH-сессии
пользователя `wapp`:

```sh
dockerd-rootless-setuptool.sh install
docker context use rootless
docker info
```

При ошибке предварительных проверок исправьте указанные зависимости/настройки
Ubuntu по https://docs.docker.com/engine/security/rootless/troubleshoot/.
Не отключайте AppArmor целиком.

Альтернатива для уже установленного системного Docker:
`sudo usermod -aG docker wapp`, затем новая SSH-сессия. Членство в группе
`docker` фактически предоставляет права root на сервере.

## Первый запуск

Разместите исходники в каталоге проекта, затем от `wapp`:

```sh
cd /home/wapp/apps/review-api
umask 077
cp -n deploy/env.example .env
openssl rand -hex 32
nano .env
sh deploy/deploy.sh
```

Запишите сгенерированный пароль в `DB_PASSWORD`, настоящий токен бота в
`TELEGRAM_BOT_TOKEN`, домены в `APP_URL` и `APP_CORS_ORIGINS`.
Не отправляйте секреты в чат и не коммитьте `.env`.
Compose читает `.env`; не выполняйте его через `source`.

API доступен только на `127.0.0.1:8080`. Настройте существующий nginx с HTTPS
для домена API и `proxy_pass http://127.0.0.1:8080`.
Не публикуйте `/metrics` через внешний reverse proxy.
PostgreSQL и Redis доступны только внутри сети Compose; Redis без пароля,
поэтому подключать посторонние контейнеры к этой сети нельзя.

API автоматически применяет встроенные Goose-миграции до старта HTTP.
Worker запускается после успешной `/readyz`. Отдельный migrator предназначен
для обслуживания: `docker compose --profile tools run --rm migrator`.
Не запускайте миграции параллельно с деплоем.

## Обновление и проверка

Обновите исходники до проверенного коммита. Установите `APP_VERSION` в `.env`
в его SHA, чтобы сохранить предыдущий образ для отката, затем:

```sh
sh deploy/deploy.sh
curl --fail http://127.0.0.1:8080/readyz
docker compose logs --tail=100 app worker
```

Скрипт сначала собирает образ, затем останавливает worker и обновляет API.
Если API не готов, скрипт завершается ошибкой, worker остаётся остановленным.
Это не zero-downtime деплой. Проверка worker подтверждает запуск процесса,
но не успешную обработку outbox — проверяйте его логи.

Перед миграциями делайте резервную копию PostgreSQL:

```sh
umask 077
docker compose exec -T postgres pg_dump -U review -d review -Fc > "$HOME/backups/review-$(date +%Y%m%d-%H%M%S).dump"
```

Копируйте резервные копии за пределы сервера и проверяйте восстановление.
Не используйте `docker compose down -v`: это удалит данные.
Изменение `DB_PASSWORD` в `.env` не меняет пароль уже созданной БД.

Для отката выберите прежний `APP_VERSION` и совместимый compose.yaml,
затем выполните `docker compose up -d --no-build --wait app worker`.
Откат образа не откатывает схему БД; несовместимые миграции требуют отдельного
плана восстановления. Не удаляйте предыдущие образы до проверки релиза.

## Подключение review-hub-api.com

Необязательный этап: пока домен не используется, установите
`APP_URL=http://localhost:8080` и подключайтесь с компьютера через SSH-туннель:

```sh
ssh -N -L 8080:127.0.0.1:8080 -i ~/.ssh/my_server wapp@194.146.223.221
```

Проверка на компьютере: `curl --fail http://localhost:8080/readyz`.

Направьте DNS A-запись домена на `194.146.223.221`. Если есть AAAA-запись,
она также должна вести на этот сервер. Администратор выполняет:

```sh
sudo install -m 644 /home/wapp/apps/review-api/deploy/nginx.conf /etc/nginx/sites-available/review-hub-api.com
sudo ln -s /etc/nginx/sites-available/review-hub-api.com /etc/nginx/sites-enabled/review-hub-api.com
sudo nginx -t && sudo systemctl reload nginx
sudo apt-get install -y certbot python3-certbot-nginx
sudo certbot --nginx -d review-hub-api.com --redirect
sudo certbot renew --dry-run
```

Откройте TCP 80/443 в firewall, сохранив доступ по SSH. Конфигурация nginx
выше предназначена для первоначального выпуска сертификата; настройка
завершена только после успешного HTTPS и проверки обновления сертификата.
