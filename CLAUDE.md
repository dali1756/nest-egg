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
- 錯誤只在最上層記 log 一次（`main()`、HTTP 統一錯誤處理、背景工作的最上層），中間層只包裝後回傳。
- log 用 `log/slog`：訊息用固定字串、變數放欄位；不記密碼、token、完整的 `DATABASE_URL`。request ID、access log、錯誤回應格式見 [docs/architecture.md](docs/architecture.md) 第 8 節。
- 測試用 `testing` + `net/http/httptest`。

## 檢查指令

- `go.mod` 在 `backend/`，在專案根目錄執行要加 `-C backend`。單元測試用假物件，不需要開 Docker。
- 改了後端程式碼，commit 前執行：
  ```bash
  go -C backend vet ./...
  go -C backend test ./...
  ```
- 新寫的測試要確認抓得到 bug：暫時改壞被測的那一行，測試應該失敗，確認後改回來。

## 開發用 skill

- 改了金額、匯率、日期、幣別或 log 相關的程式碼後，在 commit 或 `/opsx:verify` 之前，用 `nest-egg-money-review` 檢查設計規則。
- 整個開發環境的端對端檢查由使用者執行 `/nest-egg-verify`；Claude 不自行啟動或停止容器。
- 自訂 skill 放在 `.claude/skills/nest-egg-*/` 並進版控；`.claude/` 的其他內容不進版控。

## 打包成 docker 測試資料庫有無連線上
```bash
docker-compose up
```
輸入網址`http://localhost:5173`預期會看到`後端API`、`資料庫`皆顯示正常。