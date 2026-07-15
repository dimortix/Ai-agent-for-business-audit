# Альфа-Пульс

**Персональный «финансовый доктор» для микробизнеса.** Владелец кофейни открывает
сайт на телефоне и видит: индекс здоровья бизнеса (1–100), прогноз выручки на
14 дней, дату возможного кассового разрыва и конкретные советы «что сделать».
Если индекс падает в красную зону — приходит web-push и сообщение в Telegram.

MVP для пилота банка: участники группы **B** получают полный функционал,
группа **A** — контрольная (только статистика).

## Архитектура

```
┌──────────────┐   HTTP    ┌───────────────────────────────┐
│ React PWA    │◄─────────►│ Go-сервер (chi)               │
│ Vite+Tailwind│  JWT в    │  · REST API + раздача SPA     │
│ Recharts     │  cookie   │  · планировщик пересчёта      │
└──────────────┘           │  · Telegram-бот (long polling)│
                           │  · Web Push (VAPID)           │
       push ▲              └───┬──────────┬───────────┬────┘
            │              POST /forecast │           │
┌───────────┴──┐           ┌──────────────▼──┐  ┌─────▼─────┐
│ sw.ts (PWA)  │           │ ML-сервис Python │  │ PostgreSQL │
└──────────────┘           │ FastAPI + Prophet│  │ + Redis    │
                           └──────────────────┘  └────────────┘
```

- **Прогноз**: Prophet (Python). Если ML-сервис недоступен — Go-fallback:
  аддитивный Хольт-Винтерс (`internal/forecast/hw.go`, сезонность 7 дней).
- **ИЖБ**: формула ТЗ `100·(баланс + прогноз 30д)/(1.2·месячные платежи)`,
  ограничение 1–100, **плюс штрафы** −25 за кассовый разрыв в горизонте и
  −10 за недельный спад выручки >15% (см. «Отклонения от ТЗ»).
- **Кассовый разрыв**: день, когда расчётный баланс опускается ниже «подушки»
  20% месячных платежей (fixed_expenses разворачиваются по due-дням).
- **Советы**: 4 правила (падение чека/выручки/трафика, разрыв) с дедупликацией
  72 часа; отметка «выполнено» — `POST /api/advice/{id}/done`.

## Быстрый старт

Нужны Docker + Docker Compose (для демо-генератора — Go 1.22+).

```bash
cp .env.example .env
make vapid          # сгенерирует VAPID-ключи → вставьте обе строки в .env
make up             # docker compose up -d --build (первая сборка ~5 минут)
make demo           # участники + расходы + 83 дня транзакций + пересчёт
```

Откройте **http://localhost:8080**:

1. Вход: телефон `+79001234567` (кофейня «Демо Кофе», группа B).
2. Код: в dev-режиме показывается прямо на экране входа (поле `debug_code`),
   дублируется в логах `make logs` и в Telegram, если привязан бот.
3. На дашборде: индекс в жёлтой зоне, кассовый разрыв через ~10 дней,
   прогноз Prophet с доверительным интервалом, активные советы.
4. Нажмите «Включить» в карточке уведомлений (разрешите пуши браузеру).

Симуляция кризиса (приёмочный критерий №4 — push при ИЖБ < 40):

```bash
make crisis         # докатывает 7 «провальных» дней → индекс в красной зоне
```

→ придёт push-уведомление (и Telegram, если привязан).

Контрольная группа: вход `+79007654321` — только статистика, без прогнозов.

### Telegram-бот (опционально)

1. Создайте бота у @BotFather, токен в `.env` → `TELEGRAM_BOT_TOKEN=...`
2. `docker compose up -d app`
3. В боте: `/start` → «Поделиться номером» → `/status`, `/advice`.
   Теперь OTP-коды входа и тревожные уведомления приходят в Telegram.

## Команды

| Команда | Что делает |
|---|---|
| `make up` / `make down` | поднять/остановить стек |
| `make demo` | сгенерировать и загрузить демо-данные |
| `make crisis` | докатить кризисные дни (индекс < 40 → push) |
| `make test` | юнит-тесты Go (ИЖБ, Хольт-Винтерс, разрыв, правила, парсеры) |
| `make test-web` | typecheck + прод-сборка фронтенда |
| `make vapid` | сгенерировать VAPID-ключи |
| `make reset-alarm` | сбросить суточный антиспам тревог (для повторного демо пуша) |
| `make logs` | логи Go-приложения |
| `make psql` | консоль PostgreSQL |

E2E-прогон в headless Chrome (нужен установленный Chrome):
`cd web && node e2e.mjs` — логин → дашборд → аналитика → советы, скриншоты 375px;
`node e2e-full.mjs` — расширенный чек-лист обеих групп + админка.

## Переменные окружения

| Переменная | Default | Описание |
|---|---|---|
| `DATABASE_URL` | `postgres://pulse:pulse@localhost:5432/pulse?sslmode=disable` | PostgreSQL |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis (OTP, refresh-токены, антиспам) |
| `ML_SERVICE_URL` | `http://localhost:8000` | Python ML-сервис |
| `HTTP_ADDR` | `:8080` | адрес HTTP-сервера |
| `JWT_SECRET` | dev-значение | секрет подписи JWT — **заменить в проде** |
| `ADMIN_TOKEN` | `alfa-admin` | токен админских эндпоинтов |
| `APP_ENV` | `dev` | `dev` — OTP возвращается в ответе; `prod` — нет |
| `TELEGRAM_BOT_TOKEN` | — | пусто → бот выключен |
| `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` | — | пусто → web push выключен |
| `RECALC_INTERVAL` | `1h` | период фонового пересчёта |
| `WEB_DIR` | `web/dist` | собранный фронтенд |

## API (кратко)

Авторизация: JWT в http-only cookie (access 15 мин / refresh 7 дней с ротацией).

| Метод | Путь | Описание |
|---|---|---|
| POST | `/api/auth/request-code` | `{"phone":"+79..."}` → OTP в Telegram (dev: + `debug_code`) |
| POST | `/api/auth/verify` | `{"phone","code"}` → cookie-пара токенов |
| POST | `/api/auth/refresh` · `/api/auth/logout` | ротация / выход |
| GET | `/api/dashboard` | ИЖБ, прогноз 14 дней, разрыв, факт недели |
| GET | `/api/analytics?from&to` | история метрик |
| GET | `/api/advice?status=active\|done\|all` | советы |
| POST | `/api/advice/{id}/done` | отметить выполненным |
| GET | `/api/push/vapid-key` · POST `/api/push/subscribe` | web push |
| POST | `/api/participants/import` 🔑 | CSV `phone,account_id,group_type,name` |
| POST | `/api/admin/import-transactions` 🔑 | CSV `account_id,date,amount,type` + автопересчёт |
| POST | `/api/admin/recalculate/{id}` 🔑 | внеочередной пересчёт |
| GET | `/api/admin/participants` 🔑 | сводка по участникам |

🔑 — требуется заголовок `X-Admin-Token`. CSV ≤ 10 МБ, построчная валидация,
типы операций: `income`/`приход`, `return`/`возврат`; даты `2006-01-02` или `02.01.2006`.

Импорт транзакций **накапливает** дневные агрегаты (повторная загрузка того же
файла удвоит суммы — это UPSERT-аккумулятор по ТЗ).

CLI-коллектор для крона: `docker compose exec app /app/bin/collector -file /app/data/transactions.csv`.

## Тесты

```bash
make test     # 35+ юнит-тестов: Хольт-Винтерс, ИЖБ, детектор разрыва,
              # правила рекомендаций, парсинг CSV/телефонов, ML-клиент (httptest)
```

## Структура проекта

```
cmd/server        основной сервер (API, SPA, бот, планировщик)
cmd/collector     CLI-импорт CSV для крона
cmd/tools/vapid   генератор VAPID-ключей
internal/
  config, db      конфиг из env; пул pgx + мини-раннер миграций (go:embed)
  models          структуры данных (деньги — shopspring/decimal)
  repository      весь SQL (pgx/v5)
  service         бизнес-логика: коллектор, прогноз, ИЖБ, разрыв, правила,
                  нотификатор, планировщик
  forecast        fallback Хольт-Винтерс (без внешних библиотек)
  auth            OTP (Redis), JWT + ротация refresh, middleware
  handler         HTTP-обработчики + раздача SPA
  bot, push       Telegram-бот, Web Push (VAPID)
ml-service        FastAPI + Prophet (Docker)
web               React 18 + Vite 6 + Tailwind 4 + Recharts, PWA (injectManifest)
scripts/gendemo   генератор демо-данных
scripts/genicons  генератор PWA-иконок (stdlib image/png)
```

## Сознательные отклонения от ТЗ

1. **Фронтенд — React SPA** (Vite + Tailwind + Recharts) вместо Go-шаблонов:
   выбор заказчика; Go раздаёт собранный `web/dist`, PWA сохранена.
2. **ИЖБ со штрафами**: базовая формула ТЗ легко даёт 100 даже при разрыве
   через неделю. Добавлены поправки: −25 (разрыв в горизонте 14 дней),
   −10 (недельный спад выручки >15%). Форма ТЗ сохранена как база.
3. `predictions` расширена колонками `predicted_lower/upper` (интервал на
   графике) и `model_used` (`prophet`/`hw` — видно, какая модель сработала).
4. Новая таблица `push_subscriptions` (в схеме ТЗ подписки хранить негде).
5. `cmd/notifier` не выделен — уведомления горутиной в server (ТЗ разрешает).
6. Админ-авторизация — заголовок `X-Admin-Token` (в ТЗ механизм не задан).
7. Dev-режим возвращает OTP в ответе `request-code` (иначе демо без
   настроенного Telegram-бота невозможно); в `APP_ENV=prod` выключено.
8. `participants.name` — имя бизнеса для UI/бота; `account_id` — UNIQUE.

## Приёмочные критерии ТЗ — как проверить

| # | Критерий | Проверка |
|---|---|---|
| 1 | `docker-compose up` поднимает всё; вход участником B; дашборд | Быстрый старт выше |
| 2 | Импорт CSV с 90 днями → ИЖБ, прогноз, рекомендации | `make demo` (83 дня) + `make crisis` (ещё 7) |
| 3 | `/status` в Telegram-боте | раздел «Telegram-бот» |
| 4 | Push при ИЖБ < 40 | `make crisis` |
| 5 | Мобильная вёрстка 375px | адаптив, нижняя навигация, PWA |
# Ai-agent-for-business-audit
