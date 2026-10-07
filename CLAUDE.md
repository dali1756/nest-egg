# nest-egg

個人資產追蹤系統，核心是**外幣換匯成本**：記錄每筆換匯的實際成交匯率，比較每月平均成本，計算目前市值與未實現損益；之後加入台股、美股。

完整架構、資料表、API 草稿與開發階段見 [docs/architecture.md](docs/architecture.md)。

## 技術選型

- 後端：Go、標準函式庫 `net/http`、PostgreSQL + pgx/v5 + sqlc、goose、`shopspring/decimal`、`log/slog`
- 前端：Vite + React + TypeScript（pnpm）、React Router、TanStack Query、React Hook Form + Zod、Recharts、shadcn/ui
- 不用 Next.js（會和 Go 後端重疊成兩個後端）；除非真的需要，不引入 ORM、cron、Redis 等額外套件

## 設計規則

1. **金額與匯率不用 float。** Go 用 `decimal.Decimal`、DB 用 `NUMERIC`、JSON 用字串（`"amount": "32420.00"`）。
2. **每筆換匯同時存兩種匯率。** `trade_rate` 是銀行實際成交匯率（使用者輸入），`market_rate` 是當日 API 參考匯率（系統帶入）。`market_rate` 存數值，不用外鍵指向共用的匯率紀錄。
3. **查不到某日匯率時 `market_rate` 留 NULL**，不得用最新匯率或預設值頂替。
4. **日期一律以 `Asia/Taipei` 計算。** Go 用 `time.LoadLocation("Asia/Taipei")`，禁止用 `time.Now().Truncate(24*time.Hour)` 取今天（那是 UTC 午夜）；SQL 用 `AT TIME ZONE 'Asia/Taipei'`；`main` 加 `import _ "time/tzdata"`。
5. **幣別是資料。** 用 `currencies` 表加外鍵，不用 CHECK 或程式碼寫死幣別清單。
6. **匯率報價慣例：1 外幣 = X 台幣**（同銀行牌告）。v1 每筆換匯有一邊必須是 TWD。
7. **平均成本 = SUM(付出 + 手續費) ÷ SUM(取得)**，不要把各筆匯率相加再平均。
8. **只有後端呼叫外部 API。** 資料來源包成 `RateProvider` interface，背景 goroutine 每天抓一次存進 `rate_snapshots`；前端只呼叫自己的 `/api`。

## 程式碼慣例

- `backend/internal/` 依功能分 package（`currency`、`fx`、`rate`、`stock`），每個 package 內放 `handler.go`、`service.go`、`repository.go`；共用的設定、DB 連線、middleware 放 `platform/`。
- 路由用 Go 1.22+ 寫法：`mux.HandleFunc("GET /api/fx-trades/{id}", h.get)`。
- 錯誤往上傳時用 `fmt.Errorf("...: %w", err)` 包裝。
- 測試用 `testing` + `net/http/httptest`。

