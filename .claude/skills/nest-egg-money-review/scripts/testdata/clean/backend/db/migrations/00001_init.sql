-- +goose Up
-- 金額與匯率一律 NUMERIC，不用 float 或 double precision
CREATE TABLE currencies (
    code       CHAR(3)  PRIMARY KEY,
    name       TEXT     NOT NULL,
    minor_unit SMALLINT NOT NULL
);

CREATE TABLE rate_snapshots (
    base       CHAR(3)       NOT NULL REFERENCES currencies(code),
    quote      CHAR(3)       NOT NULL REFERENCES currencies(code),
    rate_date  DATE          NOT NULL,
    rate       NUMERIC(20,8) NOT NULL CHECK (rate > 0),
    fetched_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    PRIMARY KEY (base, quote, rate_date)
);

INSERT INTO currencies (code, name, minor_unit) VALUES
    ('TWD', '新台幣', 2),
    ('USD', '美元', 2);
