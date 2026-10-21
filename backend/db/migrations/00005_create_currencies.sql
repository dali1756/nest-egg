-- +goose Up
-- 幣別是所有使用者共用的資料（CLAUDE.md 設計規則 5）。新增幣別用新的 migration 插入資料。
CREATE TABLE currencies (
    code       CHAR(3)  PRIMARY KEY CHECK (code ~ '^[A-Z]{3}$'),
    name       TEXT     NOT NULL,
    minor_unit SMALLINT NOT NULL  -- ISO 4217 的小數位數
);

INSERT INTO currencies (code, name, minor_unit) VALUES
    ('TWD', '新臺幣', 2),
    ('USD', '美元', 2);

-- +goose Down
DROP TABLE currencies;
