CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL    PRIMARY KEY,
    email         VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name          VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE TABLE IF NOT EXISTS categories (
    id         BIGSERIAL    PRIMARY KEY,
    user_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       VARCHAR(100) NOT NULL,
    type       VARCHAR(10)  NOT NULL CHECK (type IN ('income', 'expense')),
    color      VARCHAR(7)   NOT NULL DEFAULT '#6B7280',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_categories_user_name UNIQUE (user_id, name)
);

CREATE TABLE IF NOT EXISTS transactions (
    id          BIGSERIAL     PRIMARY KEY,
    user_id     BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id BIGINT                 REFERENCES categories(id) ON DELETE SET NULL,
    amount      NUMERIC(15,2) NOT NULL CHECK (amount > 0),
    currency    VARCHAR(3)    NOT NULL DEFAULT 'PLN',
    type        VARCHAR(10)   NOT NULL CHECK (type IN ('income', 'expense')),
    description TEXT          NOT NULL DEFAULT '',
    date        DATE          NOT NULL DEFAULT CURRENT_DATE,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS attachments (
    id             BIGSERIAL    PRIMARY KEY,
    transaction_id BIGINT       NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    user_id        BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_name      VARCHAR(255) NOT NULL,
    file_path      VARCHAR(512) NOT NULL,
    file_size      BIGINT       NOT NULL,
    content_type   VARCHAR(100) NOT NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS api_logs (
    id          BIGSERIAL    PRIMARY KEY,
    user_id     BIGINT                REFERENCES users(id) ON DELETE SET NULL,
    method      VARCHAR(10)  NOT NULL,
    path        VARCHAR(512) NOT NULL,
    status_code INT          NOT NULL,
    duration_ms BIGINT       NOT NULL,
    user_agent  TEXT         NOT NULL DEFAULT '',
    ip_address  VARCHAR(45)  NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);
CREATE INDEX IF NOT EXISTS idx_categories_user_id   ON categories (user_id);
CREATE INDEX IF NOT EXISTS idx_categories_user_type ON categories (user_id, type);
CREATE INDEX IF NOT EXISTS idx_transactions_user_id     ON transactions (user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_user_date   ON transactions (user_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_category_id ON transactions (category_id);
CREATE INDEX IF NOT EXISTS idx_transactions_type        ON transactions (user_id, type);
CREATE INDEX IF NOT EXISTS idx_transactions_description ON transactions USING gin (description gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_attachments_transaction_id ON attachments (transaction_id);
CREATE INDEX IF NOT EXISTS idx_attachments_user_id        ON attachments (user_id);
CREATE INDEX IF NOT EXISTS idx_api_logs_user_id    ON api_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_api_logs_created_at ON api_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_logs_path       ON api_logs (path);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE OR REPLACE VIEW user_balance AS
SELECT
    user_id,
    currency,
    COALESCE(SUM(CASE WHEN type = 'income'  THEN amount ELSE 0 END), 0) AS total_income,
    COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0) AS total_expense,
    COALESCE(SUM(CASE WHEN type = 'income'  THEN amount
                      WHEN type = 'expense' THEN -amount
                      ELSE 0 END), 0)                                   AS balance
FROM transactions
GROUP BY user_id, currency;

CREATE OR REPLACE VIEW api_stats AS
SELECT
    path,
    method,
    COUNT(*)                        AS request_count,
    AVG(duration_ms)                AS avg_duration_ms,
    COUNT(CASE WHEN status_code >= 400 THEN 1 END) AS error_count,
    DATE_TRUNC('hour', created_at)  AS hour_bucket
FROM api_logs
GROUP BY path, method, DATE_TRUNC('hour', created_at);

INSERT INTO users (email, password_hash, name) VALUES
    ('alice@example.com',
     '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/lewrZYU1uqFE2ymXS',
     'Alice Kowalska'),
    ('bob@example.com',
     '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/lewrZYU1uqFE2ymXS',
     'Bob Nowak')
ON CONFLICT DO NOTHING;

INSERT INTO categories (user_id, name, type, color) VALUES
    (1, 'Wynagrodzenie',  'income',  '#22C55E'),
    (1, 'Freelance',      'income',  '#10B981'),
    (1, 'Jedzenie',       'expense', '#EF4444'),
    (1, 'Transport',      'expense', '#F97316'),
    (1, 'Rozrywka',       'expense', '#8B5CF6'),
    (1, 'Czynsz',         'expense', '#EC4899'),
    (1, 'Zdrowie',        'expense', '#14B8A6'),
    (1, 'Oszczednosci',   'income',  '#3B82F6')
ON CONFLICT DO NOTHING;

INSERT INTO categories (user_id, name, type, color) VALUES
    (2, 'Wynagrodzenie', 'income',  '#22C55E'),
    (2, 'Jedzenie',      'expense', '#EF4444'),
    (2, 'Transport',     'expense', '#F97316')
ON CONFLICT DO NOTHING;

INSERT INTO transactions (user_id, category_id, amount, currency, type, description, date) VALUES
    (1, 1,  6500.00, 'PLN', 'income',  'Wynagrodzenie maj 2026',        '2026-05-01'),
    (1, 2,  1200.00, 'PLN', 'income',  'Projekt freelance',             '2026-05-05'),
    (1, 3,   320.50, 'PLN', 'expense', 'Zakupy spozywcze',              '2026-05-06'),
    (1, 4,   180.00, 'PLN', 'expense', 'Paliwo',                        '2026-05-08'),
    (1, 6,  2000.00, 'PLN', 'expense', 'Czynsz za maj',                 '2026-05-10'),
    (1, 5,    89.99, 'PLN', 'expense', 'Netflix i Spotify',             '2026-05-12'),
    (1, 3,   215.30, 'PLN', 'expense', 'Restauracja',                   '2026-05-14'),
    (1, 7,   150.00, 'PLN', 'expense', 'Wizyta u lekarza',              '2026-05-16'),
    (1, 8,   500.00, 'PLN', 'income',  'Konto oszczednosciowe',         '2026-05-20'),
    (1, NULL, 45.00, 'PLN', 'expense', 'Rozne',                         '2026-05-22');

INSERT INTO transactions (user_id, category_id, amount, currency, type, description, date) VALUES
    (2, 9,  5500.00, 'PLN', 'income',  'Wynagrodzenie maj 2026', '2026-05-01'),
    (2, 10,  450.00, 'PLN', 'expense', 'Jedzenie maj',           '2026-05-15'),
    (2, 11,  200.00, 'PLN', 'expense', 'Komunikacja miejska',    '2026-05-15');
