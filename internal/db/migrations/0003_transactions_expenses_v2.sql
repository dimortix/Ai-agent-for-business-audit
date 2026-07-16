-- V2: сырые транзакции (лента операций), категории расходов, разовые расходы.

-- Лента последних операций: коллектор теперь сохраняет каждую строку CSV.
-- Время внутри дня синтезируется детерминированно (в CSV эквайринга только дата).
CREATE TABLE transactions (
    id             BIGSERIAL PRIMARY KEY,
    participant_id UUID NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    paid_at        TIMESTAMPTZ NOT NULL,
    amount         DECIMAL(12,2) NOT NULL,
    type           VARCHAR(10) NOT NULL CHECK (type IN ('income','return')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_transactions_pid_paid ON transactions (participant_id, paid_at DESC);

-- Категория регулярного платежа (rent/salary/supplies/taxes/loan/utilities/other)
ALTER TABLE fixed_expenses ADD COLUMN category VARCHAR(20) NOT NULL DEFAULT 'other';

-- Разовые расходы, которые предприниматель вносит сам («Мои расходы»)
CREATE TABLE one_off_expenses (
    id             BIGSERIAL PRIMARY KEY,
    participant_id UUID NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    date           DATE NOT NULL,
    amount         DECIMAL(12,2) NOT NULL,
    description    VARCHAR(200) NOT NULL,
    category       VARCHAR(20) NOT NULL DEFAULT 'other',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_one_off_pid_date ON one_off_expenses (participant_id, date DESC);
