# nest-egg

註冊制的複合式記帳，**每個人只看得到自己的資料**。分成兩部分：

- **收支：** 每天記下收入與支出的分類、幣別、金額，例如飲食、生活用品、薪水、股息。
- **資產：** 記錄換匯，以及台股、美股、基金、黃金的買賣，算出各幣別的現金餘額、外幣與股票的均價、市值、損益，總額以當天匯率折合台幣。換匯記錄實際成交匯率，用來算外幣的平均成本與未實現損益。

不分帳戶（現金、銀行、信用卡），現金只依幣別分開。

完整架構、資料表、API 草稿與開發階段見 [docs/architecture.md](docs/architecture.md)。

## 技術選型

- 後端：Go、標準函式庫 `net/http`、PostgreSQL + pgx/v5 + sqlc、goose、`shopspring/decimal`、`log/slog`
- 前端：Vite + React + TypeScript（pnpm）、Tailwind CSS、React Router、TanStack Query、React Hook Form + Zod、Recharts、shadcn/ui
- 不用 Next.js（會和 Go 後端重疊成兩個後端）；除非真的需要，不引入 ORM、cron、Redis 等額外套件

## 設計規則

1. **金額與匯率不用 float。** Go 用 `decimal.Decimal`、DB 用 `NUMERIC`、JSON 用字串（`"amount": "32420.00"`）。
2. **每筆換匯記兩個金額，並存市場參考匯率。** 使用者照銀行明細填外幣金額與台幣金額；成交匯率 `trade_rate` 由台幣金額 ÷ 外幣金額算出，不由使用者輸入、也不存。`market_rate` 是交易日的市場參考匯率（系統依規則 3 帶入）。`market_rate` 存數值，不用外鍵指向共用的匯率紀錄。外幣的收支紀錄也存 `market_rate`，作為外幣收入的成本。存的是記錄當下查得到的那一筆：平日白天通常是前一個營業日的匯率，之後不自動更新。修改紀錄時，只有日期或幣別改變才重新查詢。
3. **某一天的匯率取當天或以前最近一筆，並記下實際的匯率日期。** 週末、國定假日沒有公布匯率，最多往回找 10 天，再找不到才留 NULL（`market_rate`）。不得用該日之後的匯率或預設值頂替。一律透過 `internal/rate` 的同一條查詢（`RateOnOrBefore`），不另外寫。
4. **日期一律以 `Asia/Taipei` 計算。** Go 用 `internal/platform/taipei`（日期是台北時區的午夜，從資料庫讀出的 `DATE` 先經過 `taipei.FromDBDate`），禁止用 `time.Now().Truncate(24*time.Hour)` 取今天（那是 UTC 午夜）；SQL 用 `AT TIME ZONE 'Asia/Taipei'`；`main` 加 `import _ "time/tzdata"`。交易與收支的日期存 `DATE`（台北日期），不存時間戳；同一天有多筆時，依建立順序決定先後。
5. **幣別是資料。** 用 `currencies` 表加外鍵，不用 CHECK 或程式碼寫死幣別清單。
6. **匯率報價慣例：1 外幣 = X 台幣**（同銀行牌告）。v1 每筆換匯有一邊必須是 TWD。換匯沒有手續費欄位（銀行的成本已含在牌告匯率裡），股票等其他交易的手續費一律以台幣計。
7. **持有成本用移動平均。** 依日期與建立順序逐筆計算：
   - 買進或外幣收入：成本 += 付出 + 手續費（外幣收入用 `market_rate` 換算），數量 += 取得。
   - 賣出或提領：依當下均價扣成本（成本 −= 均價 × 數量），均價不變。
   - 每月加權平均換匯成本另外算：SUM(台幣金額) ÷ SUM(外幣金額)，只看當月的買進（`side = 'buy'`）。
   - 兩者都不可以把各筆匯率相加再平均。餘額可以是負數；外幣數量為負的期間，均價與損益算不出來，要回 null。
8. **只有後端呼叫外部 API。** 資料來源包成 `RateProvider` interface，背景 goroutine 每天抓一次存進 `rate_snapshots`；前端只呼叫自己的 `/api`。
9. **資產資料屬於使用者。** 使用者只從 `auth.CurrentUser(ctx)` 取得，不從網址、query、body、header 讀 `user_id`。使用者資料表都有 `user_id`，每條查詢都用它篩選，查單筆、修改、刪除用 `id AND user_id`。別人的資料和不存在一樣回 404，回應不含 `user_id`。每個使用者資料 API 都要有「A 讀、改、刪不到 B」的測試。所有 API 路徑列在 `cmd/server/routes.go`，業務 API 寫在 `protected` 區塊（`/api/` 預設需要登入）。

## 程式碼慣例

- `backend/internal/` 依功能分 package（`currency`、`ledger`、`fx`、`rate`、`stock`），每個 package 內放 `handler.go`、`service.go`、`repository.go`；共用的設定、DB 連線、middleware 放 `platform/`。
- 路徑統一寫在 `cmd/server/routes.go`，用 Go 1.22+ 寫法：`protected.Handle("GET /api/fx-trades/{id}", httpx.Handler(h.fx.Get))`。各 package 不自己註冊路徑，handler 方法大寫開頭。
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
  docker-compose exec backend go test -count=1 ./...
  ```
- 最後一行在後端容器裡執行，跑的是需要資料庫的整合測試，例如 SQL 的 `user_id` 篩選、合計、migration 的限制。在主機上沒有 `TEST_DATABASE_URL`，這些測試會自動略過；`ok` 不代表 SQL 有被測到。
  - 測試在開發資料庫裡建立隨機名稱的 schema，結束時刪除。
  - `exec` 只是在執行中的容器裡跑指令，不算啟動容器；容器沒在跑時，請使用者啟動。
- 新寫的測試要確認抓得到 bug：暫時改壞被測的那一行，測試應該失敗，確認後改回來。

## 開發流程

規格用 OpenSpec（`openspec/` 不進版控），開發紀律用 superpowers。兩者重疊的地方以這裡為準：

1. 釐清需求用 `superpowers:brainstorming`。小改動在對話中確認；需要寫成文件時用 `/opsx:propose` 建立 OpenSpec change，不寫 `docs/superpowers/specs/`。
2. change 的 `tasks.md` 就是實作計畫，不另外用 `superpowers:writing-plans` 產生 `docs/superpowers/plans/`。
3. `/opsx:apply` 逐項實作，每項照 `superpowers:test-driven-development` 先寫失敗測試；遇到 bug 或非預期的測試結果用 `superpowers:systematic-debugging`。
4. 宣告完成前：需要時先跑 `nest-egg-money-review`，再照 `superpowers:verification-before-completion` 執行「檢查指令」並貼出結果，最後 `/opsx:verify`。
5. 使用者 commit 後，用 `/opsx:archive` 歸檔 change。

- 除非使用者明確要求，Claude 不 commit、push、建分支或開 worktree。流程走到 commit 步驟時，依「Commit 慣例」列出建議的 commit 訊息後停下。
- 不開 worktree 的原因：`docker-compose.yml` 掛載的是主目錄的 `./backend`、`./frontend`，worktree 裡的修改 air 和 Vite 都看不到，`/nest-egg-verify` 也測不到。

## 開發用 skill

- 改了金額、匯率、日期、幣別、log 或使用者資料相關的程式碼後，在 commit 或 `/opsx:verify` 之前，用 `nest-egg-money-review` 檢查設計規則。
- 整個開發環境的端對端檢查由使用者執行 `/nest-egg-verify`；Claude 不自行啟動或停止容器。
- 自訂 skill 放在 `.claude/skills/nest-egg-*/` 並進版控；`.claude/` 的其他內容不進版控。

## Commit 慣例

- 格式：`<type>(<scope>): <摘要>`，摘要用英文祈使句、不加句點、50 字元以內，例如 `feat(BE): add member auth with Authboss`。
- type：`feat` 新功能、`fix` 修 bug、`refactor` 不改行為的重構、`test` 只動測試、`docs` 只動文件、`chore` 依賴、設定、工具。
- scope：`BE`（`backend/`）、`FE`（`frontend/`）。只動根目錄設定、`docs/`、`.claude/skills/` 時省略 scope，例如 `docs: ...`、`chore: ...`。
- 一個 commit 只放一邊；同時改前後端就拆成兩個，後端先 commit，讓每個 commit 單獨 checkout 都能跑。跟某一邊綁在一起的共用檔跟著那一邊（例如後端新增的環境變數連同 `.env.example`、`docker-compose.yml` 放進 BE commit）。
- 內文用條列寫改了什麼、為什麼；小改動只寫摘要即可。

## 打包成 docker 測試資料庫有無連線上

```bash
docker-compose up
```

登入後輸入網址`http://localhost:5173/status`預期會看到`後端API`、`資料庫`皆顯示正常。
