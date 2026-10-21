# nest-egg 架構

## 1. 目標

開放註冊的個人資產總覽：每個人只記錄、只看得到自己的資產。

- **記錄每一筆交易：** 現金存入、提領、換匯、買進與賣出股票。持股、均價、餘額都由系統依交易計算。
- **資產總覽：**
  - 股票區：像銀行 app 一樣顯示市值、均價（含手續費）、損益。
  - 現金區：依幣別分開顯示；外幣也有均價與損益（從換匯紀錄算出，見下方範例）。
  - 總額：以當天匯率折合台幣。
- **換匯成本：** 記錄每筆換匯的成交匯率與市場參考匯率、匯差，以及每月加權平均成本，比較這個月是否比上個月買貴。
- **價格與匯率每天更新一次**；總額每天會隨價格浮動。
- **美股以台幣交割：** 從台幣帳戶扣款，手續費也是台幣。股票交易本身記銀行的換算匯率，不經過美金現金，也不另外產生換匯紀錄。

### 開發階段

| 步驟 | 內容 | 完成後看得到 |
| ---- | ---- | ------------ |
| 會員 | 註冊、登入、session、鎖定、忘記與變更密碼 | 登入後的首頁 |
| 1 基礎架構 | `/api/` 預設需要登入、統一錯誤處理、request ID、panic 處理、「資料屬於使用者」規則與檢查 | 每個回應都有 `X-Request-ID` |
| 2 幣別與每日匯率 | `currencies`、央行收盤匯率（2008 年起的歷史）、每天更新存進 `rate_snapshots`、週末與假日往回找 | 沒有畫面；登入後開 `/api/rates?base=USD&quote=TWD` 看到最近一個營業日的匯率 |
| 3 現金與換匯 | 存入、提領、換匯 | 總資產頁第一版：現金區（各幣別、外幣均價與損益）與台幣總額 |
| 4 台股 | 買進、賣出、每日收盤價，買賣連動台幣現金 | 股票區：市值、均價、損益 |
| 5 美股 | 台幣交割、每日收盤價（資料來源待查） | 美股加入股票區，損益含匯差 |

各步驟的資料表與 API 細節在各自的 OpenSpec change 決定，第 5、6 節是草稿。

### 範例

假設今天 USD/TWD 市場匯率是 31.80：

|          | 美金  | 成交匯率 | 付出台幣   |
| -------- | ----- | -------- | ---------- |
| 9 月     | 1,000 | 32.10    | 32,100     |
| 10 月    | 1,000 | 32.42    | 32,420     |
| **合計** | 2,000 |          | **64,520** |

- 10 月比 9 月每 1 美元多付 0.32 台幣，約貴 1.0%
- 目前市值 2,000 × 31.80 = 63,600，未實現損益 **−920 台幣**

## 2. 名詞

| 名詞                       | 意思                                       | 來源                 |
| -------------------------- | ------------------------------------------ | -------------------- |
| 成交匯率 `trade_rate`      | 銀行實際換給你的匯率（牌告賣出價）         | 使用者從交易明細輸入 |
| 市場參考匯率 `market_rate` | 中央銀行公布的新臺幣/美元銀行間收盤匯率；週末、假日用之前最近一個營業日的 | 系統依交易日帶入     |
| 匯率日期 `rate_date`       | 市場參考匯率實際是哪一天的，可能早於交易日 | 系統帶入             |
| 匯差                       | 成交匯率和市場匯率的差距，即銀行賺走的部分 | 計算                 |
| 成本                       | 換匯時付出的台幣（含手續費），之後固定不變 | 交易紀錄             |
| 市值                       | 持有外幣 × 今日市場匯率，每天變動          | 計算                 |

## 3. 技術選型

| 層        | 選擇                                | 理由                                                                                                                                                                        |
| --------- | ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 後端路徑  | 標準函式庫 `net/http`               | Go 1.22+ 支援 `GET /api/x/{id}` 路徑                                                                                                                                        |
| 資料庫    | PostgreSQL + pgx/v5                 | `NUMERIC` 精確存金額                                                                                                                                                        |
| SQL       | sqlc                                | 從 SQL 產生型別安全的 Go 程式碼                                                                                                                                             |
| Migration | goose                               | 管理資料表版本                                                                                                                                                              |
| 金額      | `github.com/shopspring/decimal`     | 避免浮點數誤差                                                                                                                                                              |
| Log       | `log/slog`                          | 標準函式庫結構化 log                                                                                                                                                        |
| 排程      | goroutine + `time.Timer`            | 每天台北 17:30 抓匯率，還沒公布就每小時重試到 22:30（第 7 節），不需要 cron 套件                                                                                            |
| 測試      | `testing` + `net/http/httptest`     | 標準函式庫                                                                                                                                                                  |
| 前端建置  | Vite + React + TypeScript（pnpm）   | 登入後使用的系統不需要 SEO，SPA 就夠；Next.js 自帶伺服器會和 Go 重疊                                                                                                        |
| 樣式      | Tailwind CSS（`@tailwindcss/vite`） | 顏色、圓角、陰影、字級定義在 `src/index.css` 的 `@theme`，元件直接寫 utility class；重複的樣式抽成元件（`Button`、`TextLink`、`Notice`、`PageMessage`），不另外寫 CSS class |
| 資料請求  | TanStack Query                      | 快取、loading 與錯誤狀態                                                                                                                                                    |
| 表單      | React Hook Form + Zod               | 表單驗證                                                                                                                                                                    |
| 圖表      | Recharts                            | 每月成本圖、匯率走勢圖                                                                                                                                                      |

sqlc 的型別對應（`sqlc.yaml`）：`numeric` → `decimal.Decimal`、`date` → `time.Time`。pgx 透過 `decimal.Decimal` 本身的 `sql.Scanner`／`driver.Valuer` 就能讀寫（含陣列），不需要 `pgx-shopspring-decimal`。`DATE` 讀出來是 UTC 午夜，要用 `taipei.FromDBDate` 換成台北日期。

## 4. 目錄結構

```
nest-egg/
├── backend/
│   ├── cmd/server/main.go     # 程式進入點：讀設定、連 DB、組裝 handler、middleware、啟動 HTTP 與背景工作
│   ├── cmd/server/routes.go   # 所有 API 路徑：public（不需登入）與 protected（需要登入）
│   ├── internal/
│   │   ├── auth/              # 會員：註冊、登入、session、鎖定、忘記密碼、RequireUser；authtest/ 是測試用的記憶體 Store
│   │   ├── currency/          # 幣別：currencies 表、GET /api/currencies
│   │   ├── fx/                # 換匯交易：handler.go / service.go / repository.go
│   │   ├── rate/              # 匯率：RateProvider、央行 provider、每日排程（Updater）、rate_snapshots、GET /api/rates
│   │   ├── stock/             # 第 4、5 步：台股、美股
│   │   └── platform/          # config、db（連線、migration、sqlc 產生的 dbgen）、health（健康檢查）、middleware（access log、panic）、
│   │                          # apperr（錯誤型別）、httpx（統一錯誤處理、JSON 讀寫）、requestid（request ID、帶 request_id 的 log）、ratelimit、
│   │                          # taipei（台北日期）
│   ├── db/
│   │   ├── migrations/        # goose
│   │   └── queries/           # sqlc 的 .sql
│   ├── sqlc.yaml
│   ├── .air.toml              # air 熱重載設定
│   ├── Dockerfile             # 開發用映像（air）
│   └── go.mod                 # module nest-egg/backend
├── frontend/
│   ├── Dockerfile             # 開發用映像（Vite）
│   └── src/{api,pages,components,hooks,validation,test}  # routes.tsx 定義路徑；validation 放和後端共用規則的 Zod schema
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

### 資料屬於使用者

每個人只看得到自己的資產，這是最重要的資安要求（CLAUDE.md 設計規則第 9 條）：

- **使用者只從登入狀態取得：** handler 用 `auth.CurrentUser(ctx)`，MUST NOT 從路徑、query、body、header 讀 `user_id`。
- **資料表：** 屬於使用者的資料表都有 `user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE`。`currencies`、`rate_snapshots` 這類所有人共用的資料沒有 `user_id`。
- **查詢：** 每條 sqlc 查詢都用 `user_id` 篩選，包括列表與彙總；查單筆、修改、刪除用 `WHERE id = $1 AND user_id = $2`，影響 0 列時回 `apperr.ErrNotFound`。
- **別人的資料和不存在一樣回 404**，不回 403，不讓別人知道這個 id 存在。
- **回應不含 `user_id`**，也不帶出其他使用者的資料。
- **路徑：** 所有 API 列在 `cmd/server/routes.go`。業務 API 寫在 `protected` 區塊；`/api/` 底下除了公開路徑（health、註冊、登入、登出、忘記與重設密碼）一律先經過 `RequireUser`（`newRouter`）。`main_test.go` 的路徑表測試確認每條需要登入的路徑沒登入時回 401。
- **測試：** 每個使用者資料 API 都有 A/B 隔離測試：A 建立的資料，B 的列表看不到；B 用 A 的 id 查、改、刪都得到 404，A 的資料不變。
- **自動檢查：** `nest-egg-money-review` 的 `user-id-from-request` 規則抓「從請求讀 `user_id`」；查詢有沒有篩選要人工檢查（`references/judgement.md` 第 10 項）。

`currencies`、`rate_snapshots` 已定案（第 2 步，`backend/db/migrations/00005`、`00006`）；`fx_trades` 是草稿，欄位在第 3 步的 change 決定。

```sql
-- 幣別，所有使用者共用。新增幣別用新的 migration 插入資料。初始資料：TWD（新臺幣，2）、USD（美元，2）
CREATE TABLE currencies (
    code       CHAR(3)  PRIMARY KEY CHECK (code ~ '^[A-Z]{3}$'),  -- ISO 4217
    name       TEXT     NOT NULL,
    minor_unit SMALLINT NOT NULL  -- ISO 4217 的小數位數
);

-- 換匯交易：付出 pay 換到 buy。買美金：buy=USD、pay=TWD；賣回台幣則相反
CREATE TABLE fx_trades (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    traded_at    TIMESTAMPTZ   NOT NULL,
    buy_currency CHAR(3)       NOT NULL REFERENCES currencies(code),
    buy_amount   NUMERIC(20,4) NOT NULL CHECK (buy_amount > 0),
    pay_currency CHAR(3)       NOT NULL REFERENCES currencies(code),
    pay_amount   NUMERIC(20,4) NOT NULL CHECK (pay_amount > 0),
    fee          NUMERIC(20,4) NOT NULL DEFAULT 0 CHECK (fee >= 0),  -- 以 pay_currency 計
    trade_rate   NUMERIC(20,8) NOT NULL,  -- 銀行實際成交匯率（使用者輸入，1 外幣 = X 台幣）
    market_rate      NUMERIC(20,8),       -- 交易日的市場參考匯率，存值不用 FK；往回 10 天都查不到為 NULL
    market_rate_date DATE,                -- market_rate 實際的日期（週末、假日會早於交易日）
    note         TEXT,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CHECK (buy_currency <> pay_currency)
);

-- 每日市場參考匯率，所有使用者共用。只存來源真正公布的日子，週末與假日沒有資料列
CREATE TABLE rate_snapshots (
    base       CHAR(3)       NOT NULL REFERENCES currencies(code),
    quote      CHAR(3)       NOT NULL REFERENCES currencies(code),
    rate_date  DATE          NOT NULL,  -- 匯率本身的台北日期，不是抓取日期
    rate       NUMERIC(20,8) NOT NULL CHECK (rate > 0),  -- 1 base = rate 個 quote
    source     TEXT          NOT NULL,  -- 資料來源代碼，例如 cbc
    fetched_at TIMESTAMPTZ   NOT NULL DEFAULT now(),  -- 最後一次新增或修正的時間
    PRIMARY KEY (base, quote, rate_date),
    CHECK (base <> quote)
);
```

### 週末與假日的匯率

官方參考匯率只在營業日公布。某一天的市場參考匯率取**該日當天或以前最近一筆**，最多往回 10 天（春節連假約 9 天），並記下實際的匯率日期；再找不到才是 NULL。不用該日之後的匯率：交易當下不可能知道之後的價格。這是 CLAUDE.md 設計規則 3，查詢在 `db/queries/rates.sql` 的 `RateOnOrBefore`：

```sql
SELECT rate_date, rate, source FROM rate_snapshots
WHERE base = $1 AND quote = $2 AND rate_date <= $3 AND rate_date >= $3 - 10
ORDER BY rate_date DESC LIMIT 1;
```

- 換匯：`market_rate` 與 `market_rate_date` 都存下來。週末透過銀行 app 換匯，用的是銀行自訂的非營業時間報價，匯差通常比平日大，這是實際多付的成本。
- 總資產頁：用今天（台北）查到的匯率折合台幣，畫面標出匯率日期與資料來源。

### 每月加權平均換匯成本

```sql
SELECT date_trunc('month', traded_at AT TIME ZONE 'Asia/Taipei') AS month,
       SUM(pay_amount + fee) / SUM(buy_amount)                  AS avg_cost_rate,
       SUM(buy_amount)                                          AS total_bought
FROM fx_trades
WHERE user_id = $1 AND buy_currency = 'USD' AND pay_currency = 'TWD'
GROUP BY month
ORDER BY month;
```

- 同一個月買好幾次時，平均成本要用「台幣總額 ÷ 外幣總額」，不能把各筆匯率相加再平均。
- `AT TIME ZONE 'Asia/Taipei'` 確保台灣時間 10/1 早上的交易算在 10 月，而不是 UTC 的 9 月。

### 匯差

買外幣時銀行多收的比例：`(trade_rate - market_rate) / market_rate`。`market_rate` 為 NULL 時不計算。賣外幣時方向相反（成交匯率越低越吃虧）。

### 之後的步驟（草稿）

- **第 3 步 現金與換匯：** 現金存入、提領的資料表在該步驟的 change 設計；`fx_trades` 如上。現金餘額 = 存入 − 提領 ± 換匯 ± 股票買賣。
- **第 4、5 步 股票：** `stock_trades`：user_id、symbol、market（TW/US）、side、quantity、price、currency、fee、tax、traded_at。美股台幣交割另外記銀行換算匯率（`settle_rate`）與實際扣款的台幣金額，手續費以台幣計；均價與損益的算法（例如以台幣總成本 ÷ 股數為均價、損益含匯差）在第 5 步確認。

## 6. API

JSON 中的金額與匯率一律用字串。

| 方法與路徑                                                                | 用途                                                                                                                                         | 階段 |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | ---- |
| `GET /api/health`                                                         | 健康檢查：每次實際 ping 資料庫，連不到回 503；不需登入                                                                                       | 0    |
| `POST /api/auth/register`                                                 | 註冊（email、password、confirm_password），成功回 201 `{"email":"..."}` 並直接登入；欄位錯誤 422、email 已存在 409                           | 會員 |
| `POST /api/auth/login`                                                    | 登入，成功回 200 `{"email":"..."}`；帳密錯誤 401、帳號鎖定 423                                                                               | 會員 |
| `POST /api/auth/logout`                                                   | 登出，刪除伺服器端 session，回 204                                                                                                           | 會員 |
| `GET /api/auth/me`                                                        | 目前使用者 `{"email":"..."}`；未登入 401                                                                                                     | 會員 |
| `POST /api/auth/password/forgot`                                          | 申請重設密碼（email），不論 email 有沒有註冊都回 202；還沒設定寄信時回 503                                                                   | 會員 |
| `POST /api/auth/password/reset`                                           | 重設密碼（token、password、confirm_password），成功回 204；token 無效或過期 400、欄位錯誤 422                                                | 會員 |
| `POST /api/auth/password/change`                                          | 變更密碼（current_password、password、confirm_password），需登入；成功回 204，其他裝置全部登出；目前密碼錯誤 403、帳號鎖定 423、欄位錯誤 422 | 會員 |
| `GET /api/currencies`                                                     | 幣別清單 `{"currencies":[{"code","name","minor_unit"}]}`，依代碼排序                                                                        | 2    |
| `GET /api/fx-trades`                                                      | 換匯交易列表（可用 `from`、`to` 篩選日期）                                                                                                   | 3    |
| `POST /api/fx-trades`                                                     | 新增換匯交易                                                                                                                                 | 3    |
| `GET /api/fx-trades/{id}`                                                 | 單筆交易                                                                                                                                     | 3    |
| `PUT /api/fx-trades/{id}`                                                 | 修改交易                                                                                                                                     | 3    |
| `DELETE /api/fx-trades/{id}`                                              | 刪除交易                                                                                                                                     | 3    |
| `GET /api/fx-trades/monthly-cost?buy=USD&pay=TWD`                         | 每月加權平均成本                                                                                                                             | 3    |
| `GET /api/rates?base=USD&quote=TWD&date=2026-10-17`                       | 某一天的市場參考匯率（第 5 節「週末與假日的匯率」），回傳 `rate`、`rate_date`、`source`；`date` 不填是台北的今天；查不到 404，幣別不存在、日期格式錯誤或在未來 422 | 2    |
| `GET /api/rates/history?base=USD&quote=TWD&from=2026-01-01&to=2026-10-31` | 匯率歷史（走勢圖）                                                                                                                           | 之後 |
| `GET /api/portfolio/summary`                                              | 成本、市值、未實現損益（第 3 步先做現金，第 4、5 步加入股票）                                                                                | 3    |

「階段」對應第 1 節「開發階段」的步驟。幣別與匯率是所有使用者共用的資料，仍然需要登入，回應不含 `user_id`。除了 health 與會員的公開路徑，所有 API 都需要登入，而且只回傳自己的資料（第 5 節「資料屬於使用者」）。錯誤回應格式見第 8 節。

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
| ---- | ---- | ---- |
| 匯率（**使用中**） | 中央銀行「新臺幣/美元 銀行間收盤匯率」`https://cpx.cbc.gov.tw/api/OpenData/FTDOpenData_Day` | 免 key，一次回傳 2008-01-02 至今全部（約 190 KB），格式 `[{"日期":"20261019","NTD_USD":"32.445"}]`。只有 USD。工作日 16:00～17:00 公布當日收盤；週末、國定假日沒有資料，補班的週六有。授權：政府資料開放授權條款第 1 版，**顯示匯率的畫面要註明「資料來源：中央銀行」** |
| 匯率（備案） | FinMind `TaiwanExchangeRate` | 2006 年起、19 種幣別的臺銀牌告買賣價，免 token 每小時 300 次。免費方案限非商業用途、要標示 FinMind；有幾筆 -1 的錯誤值。週日 00:00～03:00 維護 |
| 匯率（備案） | Frankfurter v2 `api.frankfurter.dev/v2/rates` | TWD 1981 年起、可商用。預設值是多個央行混合計算，週末的值是推算的；`providers=AMCM` 只回實際公布的日子 |
| 匯率（不用） | open.er-api.com | 歷史資料要付費 |
| 匯率（不用） | 臺灣銀行牌告匯率 `rate.bot.com.tw` | 伺服器程式拿到的是機器人驗證頁 |
| 匯率（不用） | fawazahmed0 currency-api | 歷史只到 2024-03，有缺日，來源不明 |
| 匯率（不用） | FRED `DEXTAUS` | 條款禁止儲存與商用；每週公布，落後 3～10 天 |
| 台股 | `https://openapi.twse.com.tw/v1/exchangeReport/STOCK_DAY_ALL` | 免 key，每日收盤資料。日期是民國格式（`1151006` = 2026-10-06），數值全是字串，要自行轉換。FinMind `TaiwanStockPrice` 也是候選 |
| 美股 | FinMind `USStockPrice`、Alpha Vantage、Finnhub 等 | 第 5 步再選。Alpha Vantage、Finnhub 要申請 API key，免費方案有呼叫次數限制 |

以上是 2026-10-20 用 curl 實測與閱讀條款的結果。

### RateProvider

```go
type RateProvider interface {
    Name() string                               // 存進 rate_snapshots.source，例如 "cbc"
    Fetch(ctx context.Context) ([]Rate, error)  // 這個來源目前能提供的所有匯率
}
```

- 用 interface 包住資料來源：換來源只要換實作，測試時用假的資料來源，不打真的 API。
- 每次都取回全部資料：伺服器關機幾天、某次抓取失敗、央行修正過去的數值，下一次成功時都會自動補上或更新。
- 整批驗證後才寫入：資料是空的、日期重複或在未來、數值 ≤ 0、格式不對時整批不寫，已存的資料不動。寫入是單一 statement，新的日期新增、數值變了才更新。
- 背景工作 `rate.Updater`：伺服器啟動時抓一次，之後每天台北 17:30；失敗或還沒有今天的匯率，每小時重試到 22:30。週末與假日當天的匯率不會出現，會一路重試到 22:30，可以接受。
- 關機順序（`cmd/server/main.go` 的 `runServer`）：先停 HTTP，再取消背景工作並等它結束，最後才關資料庫連線。
- 新增換匯交易時，用第 5 節「週末與假日的匯率」的查詢帶入 `market_rate` 與 `market_rate_date`。

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

都已完成。middleware 由外到內：`requestid` → `AccessLog` → `Recover` → `SecurityHeaders` → `CrossOriginProtection` → `MaxBytesHandler` → mux（`cmd/server/main.go` 的 `withMiddleware`；路徑在 `cmd/server/routes.go`）。

| 項目           | 做法                                                                                                                                                                                                                                                                          |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 統一錯誤處理   | 業務 handler 寫成 `func(w http.ResponseWriter, r *http.Request) error`，用 `httpx.Handler` 轉接：`apperr.ErrNotFound` → 404、`*apperr.ValidationError` → 422（含 `fields`）、`httpx.DecodeJSON` 的錯誤 → 400 或 413、其他 → 500 並記一次 ERROR。service 層的輸入問題（小數位數、幣別不存在、query 格式）一律用 `ValidationError`；新的錯誤種類（例如 409）等需要時再加進 `apperr` |
| 錯誤回應       | 前端只收到通用訊息和 request ID，例如 `{"error":"internal error","request_id":"..."}`；內部錯誤細節只留在後端 log。會員 API 的 500 也帶 `request_id`                                                                                                                         |
| JSON 請求      | `httpx.DecodeJSON` 拒絕不認得的欄位（例如把 `fee` 拼成 `fees`）與 JSON 之後的多餘內容，回 400                                                                                                                                                                              |
| request ID     | `platform/requestid` 為每個請求產生 ID（`crypto/rand.Text()`），放進 context 和 `X-Request-ID` 回應標頭；不採用請求帶來的 `X-Request-ID`。`requestid.NewLogHandler` 包住 slog handler，這個請求透過 context 記的每一行 log 都自動帶上 `request_id`                          |
| access log     | 每個請求記一行：`method`、`path`（不含 query string）、`status`、`duration_ms`、`client_ip`、`request_id`（`platform/middleware/accesslog.go`）。`client_ip` 與次數限制用同一套判斷（`TRUSTED_PROXIES`、IPv6 記 /64）                                                       |
| panic          | `platform/middleware/recover.go` 用 `recover` 接住，記一筆 ERROR（含 stack trace）並回 500；`http.ErrAbortHandler` 照原樣往上拋                                                                                                                                               |
| 請求大小與逾時 | `http.MaxBytesHandler` 限制 body 1 MB；`http.Server` 設 `ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout`、`IdleTimeout`（`cmd/server/main.go`）                                                                                                                             |
| 安全性 header  | 每個回應都帶 CSP（`default-src 'none'`）、`X-Frame-Options: DENY`、`nosniff`、`Referrer-Policy: no-referrer`、`Cache-Control: no-store`；`COOKIE_SECURE=true`（HTTPS）時加 HSTS（`platform/middleware/securityheaders.go`）。之後用 embed 提供前端頁面時，HTML 要另外設定 CSP |

### 寫法規則

- 訊息用固定字串，變數放欄位：`slog.ErrorContext(ctx, "fetch rates failed", "provider", name, "err", err)`，不要把變數拼進訊息。
- 等級：DEBUG 是開發細節、INFO 是重要事件、WARN 是不正常但已處理、ERROR 是操作失敗。用 `LOG_LEVEL` 控制，開發用 `debug`、正式環境用 `info`。
- 不記密碼、token、API key、完整的 `DATABASE_URL`；財務資料盡量只記 ID，不記金額。
- 一律輸出到 stdout。開發用 `slog.NewTextHandler`；正式環境設 `LOG_FORMAT=json` 改用 `slog.NewJSONHandler`，方便依欄位查詢。`LOG_FORMAT` 預設 `text`，填其他值時啟動失敗。
- 背景工作記開始、結束、耗時與處理筆數；失敗時記一次 ERROR，並帶上工作名稱。

## 9. 開發與部署

- **開發：** `docker-compose up` 一次啟動 PostgreSQL（本機 `localhost:5433`，TZ=Asia/Taipei）、Go 後端（`localhost:8080`，air 熱重載）與 Vite 開發伺服器（`localhost:5173`）；Vite 用 proxy 把 `/api` 轉到後端，不用處理 CORS。啟動與驗證步驟見 README。
- **正式環境：** 可用 `embed` 把前端 build 出來的檔案包進 Go 執行檔，只部署一個檔案。
- **登入：** 自己實作在 `internal/auth`，不用第三方會員套件，回應一律 JSON。跨站的 POST/PUT/PATCH/DELETE 由 `http.CrossOriginProtection` 擋下。業務 API 寫在 `cmd/server/routes.go` 的 `protected` 區塊，`/api/` 底下的非公開路徑一律先經過 `RequireUser`；handler 只用 `auth.CurrentUser(ctx)` 取得使用者（第 5 節「資料屬於使用者」）。安全強度以 OWASP ASVS Level 2 為目標。
  - **次數限制：** 註冊、登入、忘記密碼、重設密碼、變更密碼這些要輸入 email 或密碼的端點，每個 IP 連續 10 次，之後每 6 秒補 1 次，超過回 429 和 `Retry-After`（`platform/ratelimit`；在 `cmd/server/routes.go` 用 `h.limit(...)` 套用）。IPv6 以 /64 為單位。預設只看連線本身的位址，不採用 `X-Forwarded-For`（使用者可以自己加這個 header）；放在反向代理後面時，把代理的 IP 或 CIDR 設進 `TRUSTED_PROXIES`，才會從 header 由右往左跳過代理取出使用者 IP。開發環境經過 Vite proxy，所有請求共用同一個額度。計數存在記憶體，重啟後歸零；之後如果跑多台後端，要改用共用的儲存或在反向代理做限制。
  - **密碼：** 只存 bcrypt 雜湊（cost 12）；舊的雜湊 cost 比較低時，使用者下次登入成功會自動換成 cost 12。規則：8～64 個字元，只能用英文字母、數字、符號（可列印 ASCII，不含空白），大寫、小寫、數字、符號各至少一個。前後端規則一致：`internal/auth/password.go`、`frontend/src/validation/password.ts`，兩邊的測試案例也一致。
  - **Session：** 存在 `sessions` 表，cookie 只放 128 bits 的隨機 token（`crypto/rand.Text()`），資料庫只存 token 的 SHA-256。cookie 是 HttpOnly、`SameSite=Lax`；`COOKIE_SECURE=true` 時加 Secure，名稱改成 `__Host-nest_egg_session`，子網域無法塞同名 cookie 進來。24 小時沒有需要登入的請求就失效；不論有沒有在用，登入滿 7 天一定失效。`last_seen_at` 最多每分鐘更新一次。登出會刪掉資料庫裡的 session，複製出去的 cookie 也跟著失效。
  - **鎖定：** 同一帳號連續 3 次密碼錯誤，`users.locked` 設成 1，之後登入一律回 423。只能手動解鎖：`UPDATE users SET locked = 0 WHERE email = '...';`。登入成功會把失敗次數歸零。鎖定只擋登入，不會讓已經登入的 session 失效。
  - **變更密碼：** 要輸入目前的密碼，錯誤次數和登入共用同一個鎖定計數。成功後其他裝置全部登出、未使用的重設 token 作廢，目前這台裝置換一個新的 session token。
  - **防探測：** 帳號不存在時也跑一次 bcrypt，回應時間和密碼錯誤一樣。email 只接受 ASCII，避免用全形等長得很像的字元註冊成另一個帳號。刻意接受的取捨：註冊時 email 已存在回 409、帳號鎖定回 423，這兩種情況仍然看得出 email 有註冊。
  - **忘記密碼：** token 存在 `password_reset_tokens`（只存雜湊），30 分鐘內有效、只能用一次，同一個使用者只保留最新的一個。前端讀到 token 後會把它從網址列拿掉。重設成功後所有裝置登出、失敗次數歸零，但不會解鎖。還沒接 SMTP，見第 10 節待辦。
  - **輸入上限：** email 254 字元、密碼 72 bytes、token 64 字元，超過的在查資料庫前就擋掉。
  - **log：** 只記 `user_id`，不記 email、密碼、token。
  - 放上網路前還要：HTTPS（`COOKIE_SECURE` 維持預設 true）；有反向代理就設定 `TRUSTED_PROXIES`，並確認代理會在 `X-Forwarded-For` 最後面加上使用者 IP。

## 10. 待辦

### SMTP：忘記密碼寄信

目前 `auth.Config.Mailer` 是 nil，`POST /api/auth/password/forgot` 回 503，前端顯示「尚未開放」。接上時要做：

- 實作 `auth.Mailer`（SMTP 或寄信服務的 API），主機、帳密、寄件人放環境變數，帳密不進 log。
- 信裡的連結用 `{APP_BASE_URL}/reset-password#token=<token>`，要新增 `APP_BASE_URL` 設定。token 放在 `#` 後面，瀏覽器不會把它送到伺服器，所以不會出現在 Referer 或 access log。前端的重設頁面已經照這個格式讀 token。
- 寄信改成背景執行：現在是同步寄，「有寄信」和「沒寄信」的回應時間不同，可以用來判斷 email 有沒有註冊；SMTP 出錯時也會回 500。
- 加上申請次數限制（同一個 email、同一個 IP），避免被拿來大量寄信騷擾別人。

### 其他

- 定期清掉過期的 `sessions` 和 `password_reset_tokens`：目前只在該使用者登入時清掉他自己過期的 session，沒有再登入的使用者會留下資料列（已經無法使用，只是占空間）。可以做成和 `rate.Updater` 一樣的背景工作，交給 `runServer` 啟動與關閉。
- 正式環境用 embed 提供前端頁面時，HTML 要另外設定 CSP（目前的 CSP 是給 JSON API 用的）。
- 正式環境的資料庫：後端改用只有必要權限的帳號（不要用 `POSTGRES_USER` 那個超級使用者），連線改成 `sslmode=verify-full`。
- 依賴弱點掃描：Go 用 `govulncheck ./...`（`brew install govulncheck`），前端用 `pnpm audit`，每次升級套件後跑一次。
- 接上 SMTP 之後再做：註冊時驗證 email、登入新裝置與變更密碼時寄通知信。
- 「目前登入的裝置」列表，讓使用者看到並個別登出（需要在 `sessions` 多存裝置資訊）。
