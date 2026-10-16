# nest-egg 架構

## 1. 目標

- 記錄每筆換匯：實際成交匯率、市場參考匯率、匯差
- 每月加權平均換匯成本，比較這個月是否比上個月買貴
- 依今日匯率算目前市值，與成本比較得出未實現損益
- 之後加入台股、美股

### 範例

假設今天 USD/TWD 市場匯率是 31.80：

| | 美金 | 成交匯率 | 付出台幣 |
|---|---|---|---|
| 9 月 | 1,000 | 32.10 | 32,100 |
| 10 月 | 1,000 | 32.42 | 32,420 |
| **合計** | 2,000 | | **64,520** |

- 10 月比 9 月每 1 美元多付 0.32 台幣，約貴 1.0%
- 目前市值 2,000 × 31.80 = 63,600，未實現損益 **−920 台幣**

## 2. 名詞

| 名詞 | 意思 | 來源 |
|---|---|---|
| 成交匯率 `trade_rate` | 銀行實際換給你的匯率（牌告賣出價） | 使用者從交易明細輸入 |
| 市場參考匯率 `market_rate` | API 給的中間價 | 系統自動帶入 |
| 匯差 | 成交匯率和市場匯率的差距，即銀行賺走的部分 | 計算 |
| 成本 | 換匯時付出的台幣（含手續費），之後固定不變 | 交易紀錄 |
| 市值 | 持有外幣 × 今日市場匯率，每天變動 | 計算 |

## 3. 技術選型

| 層 | 選擇 | 理由 |
|---|---|---|
| 後端路由 | 標準函式庫 `net/http` | Go 1.22+ 支援 `GET /api/x/{id}` 路由 |
| 資料庫 | PostgreSQL + pgx/v5 | `NUMERIC` 精確存金額 |
| SQL | sqlc | 從 SQL 產生型別安全的 Go 程式碼 |
| Migration | goose | 管理資料表版本 |
| 金額 | `github.com/shopspring/decimal` | 避免浮點數誤差 |
| Log | `log/slog` | 標準函式庫結構化 log |
| 排程 | goroutine + `time.Ticker` | 每天抓一次匯率，不需要 cron 套件 |
| 測試 | `testing` + `net/http/httptest` | 標準函式庫 |
| 前端建置 | Vite + React + TypeScript（pnpm） | 登入後使用的系統不需要 SEO，SPA 就夠；Next.js 自帶伺服器會和 Go 重疊 |
| 資料請求 | TanStack Query | 快取、loading 與錯誤狀態 |
| 表單 | React Hook Form + Zod | 表單驗證 |
| 圖表 | Recharts | 每月成本圖、匯率走勢圖 |

sqlc 與 decimal 的整合：在 `sqlc.yaml` 用 override 把 `pg_catalog.numeric` 對應到 `github.com/shopspring/decimal.Decimal`；pgx 可搭配 `github.com/jackc/pgx-shopspring-decimal` 註冊型別（實作時確認）。

## 4. 目錄結構

```
nest-egg/
├── backend/
│   ├── cmd/server/main.go     # 程式進入點：讀設定、連 DB、註冊路由、啟動排程
│   ├── internal/
│   │   ├── auth/              # 會員：Authboss 設定、users 的 storer、/api/auth/me、RequireUser
│   │   ├── currency/          # 幣別
│   │   ├── fx/                # 換匯交易：handler.go / service.go / repository.go
│   │   ├── rate/              # 匯率：RateProvider interface、排程、rate_snapshots
│   │   ├── stock/             # 第 4 階段
│   │   └── platform/          # config、db（連線、migration、sqlc 產生的 dbgen）、health（健康檢查）、middleware（access log）
│   ├── db/
│   │   ├── migrations/        # goose
│   │   └── queries/           # sqlc 的 .sql
│   ├── sqlc.yaml
│   ├── .air.toml              # air 熱重載設定
│   ├── Dockerfile             # 開發用映像（air）
│   └── go.mod                 # module nest-egg/backend
├── frontend/
│   ├── Dockerfile             # 開發用映像（Vite）
│   └── src/{api,pages,components,hooks,test}  # routes.tsx 定義路由
├── docs/
│   └── architecture.md
├── docker-compose.yml         # 開發環境：db、backend、frontend 一起啟動
├── Makefile
├── CLAUDE.md
└── README.md
```

**依功能分 package，不依層分。** 

`internal/` 是 Go 的特殊目錄：裡面的 package 只能被本 module 內的程式 import。

## 5. 資料表

```sql
CREATE TABLE currencies (
    code       CHAR(3)  PRIMARY KEY,   -- ISO 4217：TWD、USD、JPY
    name       TEXT     NOT NULL,
    minor_unit SMALLINT NOT NULL       -- 小數位數：USD 2、JPY 0
);

-- 換匯交易：付出 pay 換到 buy。買美金：buy=USD、pay=TWD；賣回台幣則相反
CREATE TABLE fx_trades (
    id           BIGSERIAL PRIMARY KEY,
    traded_at    TIMESTAMPTZ   NOT NULL,
    buy_currency CHAR(3)       NOT NULL REFERENCES currencies(code),
    buy_amount   NUMERIC(20,4) NOT NULL CHECK (buy_amount > 0),
    pay_currency CHAR(3)       NOT NULL REFERENCES currencies(code),
    pay_amount   NUMERIC(20,4) NOT NULL CHECK (pay_amount > 0),
    fee          NUMERIC(20,4) NOT NULL DEFAULT 0 CHECK (fee >= 0),  -- 以 pay_currency 計
    trade_rate   NUMERIC(20,8) NOT NULL,  -- 銀行實際成交匯率（使用者輸入，1 外幣 = X 台幣）
    market_rate  NUMERIC(20,8),           -- 交易日的市場參考匯率，存值不用 FK；查不到為 NULL
    note         TEXT,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CHECK (buy_currency <> pay_currency)
);

-- 每日市場匯率（排程每天抓一次）
CREATE TABLE rate_snapshots (
    base       CHAR(3)       NOT NULL REFERENCES currencies(code),
    quote      CHAR(3)       NOT NULL REFERENCES currencies(code),
    rate_date  DATE          NOT NULL,   -- 台北時區的日期
    rate       NUMERIC(20,8) NOT NULL CHECK (rate > 0),
    source     TEXT          NOT NULL,
    fetched_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    PRIMARY KEY (base, quote, rate_date)
);
```

### 每月加權平均換匯成本

```sql
SELECT date_trunc('month', traded_at AT TIME ZONE 'Asia/Taipei') AS month,
       SUM(pay_amount + fee) / SUM(buy_amount)                  AS avg_cost_rate,
       SUM(buy_amount)                                          AS total_bought
FROM fx_trades
WHERE buy_currency = 'USD' AND pay_currency = 'TWD'
GROUP BY month
ORDER BY month;
```

- 同一個月買好幾次時，平均成本要用「台幣總額 ÷ 外幣總額」，不能把各筆匯率相加再平均。
- `AT TIME ZONE 'Asia/Taipei'` 確保台灣時間 10/1 早上的交易算在 10 月，而不是 UTC 的 9 月。

### 匯差

買外幣時銀行多收的比例：`(trade_rate - market_rate) / market_rate`。`market_rate` 為 NULL 時不計算。賣外幣時方向相反（成交匯率越低越吃虧）。

### 第 4 階段預定

`stock_trades`：symbol、market（TW/US）、side、quantity、price、currency、fee、tax、traded_at。

## 6. API

JSON 中的金額與匯率一律用字串。

| 方法與路徑 | 用途 | 階段 |
|---|---|---|
| `GET /api/health` | 健康檢查：每次實際 ping 資料庫，連不到回 503；不需登入 | 0 |
| `POST /api/auth/register` | 註冊（email、password、confirm_password），成功後直接登入；欄位錯誤 422、email 已存在 409 | 會員 |
| `POST /api/auth/login` | 登入；帳密錯誤 401 | 會員 |
| `POST /api/auth/logout` | 登出 | 會員 |
| `GET /api/auth/me` | 目前使用者 `{"email":"..."}`；未登入 401 | 會員 |
| `GET /api/currencies` | 幣別清單 | 1 |
| `GET /api/fx-trades` | 換匯交易列表（可用 `from`、`to` 篩選日期） | 1 |
| `POST /api/fx-trades` | 新增換匯交易 | 1 |
| `GET /api/fx-trades/{id}` | 單筆交易 | 1 |
| `PUT /api/fx-trades/{id}` | 修改交易 | 1 |
| `DELETE /api/fx-trades/{id}` | 刪除交易 | 1 |
| `GET /api/fx-trades/monthly-cost?buy=USD&pay=TWD` | 每月加權平均成本 | 1 |
| `GET /api/rates/latest?base=USD&quote=TWD` | 最新市場匯率 | 2 |
| `GET /api/rates/history?base=USD&quote=TWD&from=2026-01-01&to=2026-10-31` | 匯率歷史 | 2 |
| `GET /api/portfolio/summary` | 成本、市值、未實現損益 | 2 |

`POST /api/fx-trades` 請求範例：

```json
{
  "traded_at": "2026-10-07T10:30:00+08:00",
  "buy_currency": "USD",
  "buy_amount": "1000",
  "pay_currency": "TWD",
  "pay_amount": "32420",
  "fee": "0",
  "trade_rate": "32.42",
  "note": "10 月定期換匯"
}
```

## 7. 外部資料來源

| 用途 | 來源 | 狀況 |
|---|---|---|
| 匯率 | `https://open.er-api.com/v6/latest/USD` | 免 API key、含 TWD、每天更新一次、給的是中間價。免費端點只提供最新值（歷史匯率屬付費方案，實作前確認）。條款：https://www.exchangerate-api.com/terms |
| 匯率 | 台灣銀行牌告匯率 CSV `https://rate.bot.com.tw/xrt/flcsv/0/day` | 回傳機器人驗證頁，伺服器程式抓不到 |
| 台股 | `https://openapi.twse.com.tw/v1/exchangeReport/STOCK_DAY_ALL` | 免 key，每日收盤資料。日期是民國格式（`1151006` = 2026-10-06），數值全是字串，要自行轉換 |
| 美股 | Alpha Vantage、Finnhub 等 | 要申請 API key，免費方案有呼叫次數限制 |

### RateProvider

```go
type RateProvider interface {
    Latest(ctx context.Context, base string) (map[string]decimal.Decimal, error)
}
```

- 用 interface 包住資料來源：換來源只要換實作，測試時可以放假的資料來源。
- 背景 goroutine 用 `time.Ticker` 每天抓一次，寫進 `rate_snapshots`。
- 新增換匯交易時，從 `rate_snapshots` 查交易日（台北時區）的匯率帶入 `market_rate`；查不到就留 NULL。

## 8. 錯誤處理與 log

只用標準函式庫 `log/slog`，不引入其他 log 套件。目標：出問題時，能用一個 request ID 找到那次請求的完整經過。

### 錯誤往上傳，只在最上層記一次

- 中間層（repository、service、`db.Open` 這類）只用 `fmt.Errorf("說明: %w", err)` 包裝後回傳，不記 log。每層加的前綴會串成完整的錯誤鏈。
- 只在最上層記錯誤 log：
  - 啟動流程：`main()`，`run()` 回傳錯誤後記 log 並 `os.Exit(1)`
  - HTTP 請求：統一的錯誤處理函式（見下表），handler 只回傳 error
  - 背景工作（例如每天抓匯率）：工作本身的最上層
- 預期內、能就地處理的情況不算錯誤，不往上丟。例如查不到匯率時 `market_rate` 留 NULL，視需要記一筆 WARN。

### HTTP 層

標 ✅ 的已完成，其餘在第 1 階段開始寫 API 時加進 `internal/platform/`：

| 項目 | 做法 |
|---|---|
| 統一錯誤處理 | handler 寫成 `func(w http.ResponseWriter, r *http.Request) error`，由一個轉接函式統一記 log、決定狀態碼（用 `errors.Is` 把「找不到」對應成 404 等），並輸出錯誤 JSON |
| 錯誤回應 | 前端只收到通用訊息和 request ID，例如 `{"error":"internal error","request_id":"..."}`；內部錯誤細節只留在後端 log |
| request ID | middleware 為每個請求產生 ID（`crypto/rand.Text()`），放進 context 和 `X-Request-ID` 回應標頭；這個請求的每一行 log 都帶上 `request_id` |
| access log ✅ | 每個請求記一行：`method`、`path`、`status`、`duration_ms`（`platform/middleware/accesslog.go`）；做好 request ID 後再加上 `request_id` |
| panic | middleware 用 `recover` 接住，記一筆 ERROR（含 stack trace）並回 500 |

### 寫法規則

- 訊息用固定字串，變數放欄位：`slog.ErrorContext(ctx, "fetch rates failed", "provider", name, "err", err)`，不要把變數拼進訊息。
- 等級：DEBUG 是開發細節、INFO 是重要事件、WARN 是不正常但已處理、ERROR 是操作失敗。用 `LOG_LEVEL` 控制，開發用 `debug`、正式環境用 `info`。
- 不記密碼、token、API key、完整的 `DATABASE_URL`；財務資料盡量只記 ID，不記金額。
- 一律輸出到 stdout。開發用 `slog.NewTextHandler`；正式環境設 `LOG_FORMAT=json` 改用 `slog.NewJSONHandler`，方便依欄位查詢。`LOG_FORMAT` 預設 `text`，填其他值時啟動失敗。
- 背景工作記開始、結束、耗時與處理筆數；失敗時記一次 ERROR，並帶上工作名稱。

## 9. 開發與部署

- **開發：** `docker-compose up` 一次啟動 PostgreSQL（本機 `localhost:5433`，TZ=Asia/Taipei）、Go 後端（`localhost:8080`，air 熱重載）與 Vite 開發伺服器（`localhost:5173`）；Vite 用 proxy 把 `/api` 轉到後端，不用處理 CORS。啟動與驗證步驟見 README。
- **正式環境：** 可用 `embed` 把前端 build 出來的檔案包進 Go 執行檔，只部署一個檔案。
- **登入：** 用 Authboss（`github.com/aarondl/authboss/v3`）的 auth、register、logout 模組，回應一律 JSON。密碼只存 bcrypt 雜湊；session 存在加密、簽章的 HttpOnly cookie（`SameSite=Lax`、7 天，金鑰由 `SESSION_SECRET` 以 HKDF 推導）；跨站的 POST/PUT/PATCH/DELETE 由 `http.CrossOriginProtection` 擋下。之後的業務 API 套 `auth.RequireUser` 要求登入。
  - 外層 middleware 若包裝 `ResponseWriter`，必須實作 `UnderlyingResponseWriter()`，否則 Authboss 寫 session 時會 panic。
  - 放上網路前還要：HTTPS（`COOKIE_SECURE` 維持預設 true）、登入次數限制（rate limit 或 Authboss 的 lock 模組）；若需要「登出所有裝置」，改成伺服器端 session。
