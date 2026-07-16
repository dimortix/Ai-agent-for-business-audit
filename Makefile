# Альфа.Пульс — команды разработки и демо
-include .env
ADMIN_TOKEN ?= alfa-admin
BASE_URL    ?= http://localhost:8080

.PHONY: up down build logs test test-web vapid demo crisis psql

## Поднять весь стек (сборка при необходимости)
up:
	docker compose up -d --build

down:
	docker compose down

## Полная пересборка образов
build:
	docker compose build

logs:
	docker compose logs -f app

## Юнит-тесты Go (критическая бизнес-логика)
test:
	go test ./...

## Проверка типов и прод-сборка фронтенда
test-web:
	cd web && npm run build

## Сгенерировать пару VAPID-ключей для Web Push (вставить в .env)
vapid:
	go run ./cmd/tools/vapid

## Демо-сценарий: участники + фикс. расходы + 83 дня транзакций + пересчёт
demo:
	go run ./scripts/gendemo
	curl -sf -X POST -H "X-Admin-Token: $(ADMIN_TOKEN)" -F "file=@data/participants.csv" $(BASE_URL)/api/participants/import
	@echo ""
	cat data/expenses.sql | docker compose exec -T postgres psql -q -U pulse -d pulse
	curl -sf -X POST -H "X-Admin-Token: $(ADMIN_TOKEN)" -F "file=@data/transactions.csv" $(BASE_URL)/api/admin/import-transactions
	@echo ""
	@echo "Демо готово: http://localhost:8080 — вход +79001234567 (код в ответе/логах: make logs)"

## Дополнительные участники: пекарня (зелёный), барбершоп (жёлтый+разрыв),
## фудтрак (рост), цветочный (контроль A) — поверх основного демо
demo-extra:
	go run ./scripts/gendemo
	curl -sf -X POST -H "X-Admin-Token: $(ADMIN_TOKEN)" -F "file=@data/participants.csv" $(BASE_URL)/api/participants/import
	@echo ""
	cat data/expenses_extra.sql | docker compose exec -T postgres psql -q -U pulse -d pulse
	curl -sf -X POST -H "X-Admin-Token: $(ADMIN_TOKEN)" -F "file=@data/transactions_extra.csv" $(BASE_URL)/api/admin/import-transactions
	@echo ""
	@echo "Добавлены: +79011111111 (пекарня), +79022222222 (барбершоп), +79033333333 (фудтрак), +79044444444 (цветы, A)"

## Кризис: докатить 7 «провальных» дней → ИЖБ < 40 → push/Telegram-уведомление
crisis:
	curl -sf -X POST -H "X-Admin-Token: $(ADMIN_TOKEN)" -F "file=@data/transactions_crisis.csv" $(BASE_URL)/api/admin/import-transactions
	@echo ""
	@echo "Кризис импортирован: индекс здоровья должен упасть ниже 40 (см. дашборд и уведомления)"

## Сбросить суточный антиспам тревожных уведомлений (для повторного демо пуша)
reset-alarm:
	docker compose exec redis sh -c "redis-cli KEYS 'notify:last:*' | xargs -r redis-cli DEL"

## Очистить метрики/прогнозы/советы/операции/отпечатки импорта (для чистого повторного demo)
reset-data:
	docker compose exec -T postgres psql -q -U pulse -d pulse -c \
		"TRUNCATE daily_metrics, predictions, recommendations, import_batches, transactions, one_off_expenses RESTART IDENTITY;"

## Бэкап БД в backups/pulse-<дата>.sql.gz
backup:
	mkdir -p backups
	docker compose exec -T postgres pg_dump -U pulse -d pulse | gzip > backups/pulse-$$(date +%Y%m%d-%H%M%S).sql.gz
	@ls -lh backups/ | tail -1

## Восстановление: make restore FILE=backups/pulse-....sql.gz
restore:
	@test -n "$(FILE)" || (echo "использование: make restore FILE=backups/....sql.gz" && exit 2)
	gunzip -c $(FILE) | docker compose exec -T postgres psql -q -U pulse -d pulse

## psql внутри контейнера
psql:
	docker compose exec postgres psql -U pulse -d pulse
