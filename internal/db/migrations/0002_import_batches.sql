-- Защита от случайного повторного импорта транзакций: коллектор аккумулирует
-- дневные агрегаты, поэтому второй импорт того же файла удвоил бы выручку.
-- Запоминаем sha256 содержимого каждого импортированного CSV.
CREATE TABLE import_batches (
    hash        CHAR(64) PRIMARY KEY,
    rows_count  INT NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
