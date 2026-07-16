-- Альфа.Пульс: начальная схема (по ТЗ + расширения, помеченные комментариями)

CREATE TABLE participants (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone            VARCHAR(20) UNIQUE NOT NULL,
    account_id       VARCHAR(50) UNIQUE NOT NULL, -- UNIQUE: коллектор ищет участника по счёту
    name             VARCHAR(200) NOT NULL DEFAULT '', -- расширение: имя бизнеса для UI/бота
    group_type       CHAR(1) NOT NULL CHECK (group_type IN ('A','B')),
    telegram_chat_id BIGINT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE daily_metrics (
    participant_id    UUID NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    date              DATE NOT NULL,
    total_revenue     DECIMAL(12,2) NOT NULL,
    return_amount     DECIMAL(12,2) NOT NULL DEFAULT 0,
    transaction_count INT NOT NULL DEFAULT 0,
    avg_check         DECIMAL(10,2) NOT NULL DEFAULT 0,
    PRIMARY KEY (participant_id, date)
    -- PK (participant_id, date) уже даёт нужный индекс по participant_id+date
);

CREATE TABLE fixed_expenses (
    participant_id   UUID NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    description      VARCHAR(200) NOT NULL,
    amount           DECIMAL(12,2) NOT NULL,
    due_day_of_month INT CHECK (due_day_of_month BETWEEN 1 AND 31),
    PRIMARY KEY (participant_id, description)
);

CREATE TABLE predictions (
    participant_id    UUID NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    calculated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    forecast_date     DATE NOT NULL,
    predicted_revenue DECIMAL(12,2),
    predicted_lower   DECIMAL(12,2), -- расширение: нижняя граница интервала
    predicted_upper   DECIMAL(12,2), -- расширение: верхняя граница интервала
    model_used        VARCHAR(20) NOT NULL DEFAULT 'hw', -- расширение: prophet | hw
    health_index      INT CHECK (health_index BETWEEN 1 AND 100),
    cash_gap_date     DATE,
    PRIMARY KEY (participant_id, calculated_at, forecast_date)
);

CREATE INDEX idx_predictions_pid_calc ON predictions (participant_id, calculated_at DESC);

CREATE TABLE recommendations (
    id               SERIAL PRIMARY KEY,
    participant_id   UUID NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    rule_code        VARCHAR(50) NOT NULL,
    message          TEXT NOT NULL,
    was_action_taken BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_recommendations_pid_created ON recommendations (participant_id, created_at DESC);
CREATE INDEX idx_recommendations_active ON recommendations (participant_id, rule_code)
    WHERE was_action_taken = FALSE;

-- Расширение: подписки Web Push (в схеме ТЗ их негде хранить)
CREATE TABLE push_subscriptions (
    participant_id UUID NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    endpoint       TEXT NOT NULL,
    p256dh         TEXT NOT NULL,
    auth           TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (participant_id, endpoint)
);
