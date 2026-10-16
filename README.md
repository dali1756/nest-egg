# nest-egg

個人資產追蹤系統：記錄外幣換匯的實際成交匯率、比較每月平均成本、計算未實現損益，之後加入台股、美股。架構與設計規則見 [docs/architecture.md](docs/architecture.md)。

## 啟動開發環境

只需要 Docker Desktop。第一次使用先建立 `.env`，填入資料庫密碼：

```bash
cp .env.example .env
```

之後每次只要：

```bash
docker-compose up
```

| 服務     | 位址                  | 說明                                                        |
| -------- | --------------------- | ----------------------------------------------------------- |
| frontend | http://localhost:5173 | Vite 開發伺服器，`/api` 轉給後端；存檔後畫面即時更新（HMR） |
| backend  | http://localhost:8080 | Go API；存檔後 air 自動重新編譯、重啟                       |
| db       | `localhost:5433`      | PostgreSQL 18，帳密見 `.env`；容器之間用 `db:5432`          |

常用指令：

| 指令                                               | 用途                                     |
| -------------------------------------------------- | ---------------------------------------- |
| `docker-compose up -d`                             | 在背景啟動                               |
| `docker-compose logs -f backend`                   | 看後端 log                               |
| `docker-compose down`                              | 停止並移除容器，資料庫資料保留           |
| `docker-compose down -v`                           | 連 volume 一起刪除，**資料庫資料會清空** |
| `docker-compose up --build`                        | 改了 Dockerfile 之後重建映像             |
| `docker-compose exec db psql -U admin -d nest_egg` | 進資料庫下 SQL                           |

## 確認後端真的連上資料庫

1. `docker-compose ps`：db 顯示 `(healthy)`。
2. `docker-compose logs backend`：有 `database connected ... user=admin`。後端啟動時會先 ping 資料庫，帳密或主機設定錯誤會直接啟動失敗並印出原因。
3. `curl -i http://localhost:8080/api/health`：回 `200` 與 `{"status":"ok","database":"up"}`。每次請求都會實際 ping 資料庫。
4. 反向測試：`docker-compose stop db` 後再 curl 一次，應該變成 `503` 與 `{"status":"unavailable","database":"down"}`；`docker compose start db` 後恢復 `200`，後端不用重啟。

以上步驟可以一次自動跑完（會先 `docker compose up -d`；反向測試會暫停 db 幾秒，結束時自動恢復）：

```bash
bash .claude/skills/nest-egg-verify/scripts/verify-stack.sh
```

## 會員與登入

- 第一次使用先到 http://localhost:5173/register 建立帳號（email + 密碼），註冊後直接登入。
- 密碼 8～64 個字元，只能用英文字母、數字與符號（不可有空白），大寫、小寫、數字、符號各至少一個；資料庫只存 bcrypt 雜湊。
- 登入後的首頁暫時是「系統狀態」頁，之後會換成正式首頁。
- 24 小時沒有操作會自動登出；不論有沒有在用，登入滿 7 天要重新登入。
- 同一帳號連續輸錯 3 次密碼會被封鎖，只能到資料庫解鎖：
  ```bash
  docker-compose exec db psql -U admin -d nest_egg -c "UPDATE users SET locked = 0 WHERE email = 'me@example.com';"
  ```
- 登入後右上角有「變更密碼」，要輸入目前的密碼；變更後其他裝置會全部登出。
- 註冊、登入、忘記密碼、重設密碼、變更密碼同一個 IP 每分鐘最多約 10 次，超過會顯示「嘗試次數過多，請稍後再試」。
- 「忘記密碼」要等接上 SMTP 才開放，目前畫面會顯示尚未開放（見 [docs/architecture.md](docs/architecture.md) 第 10 節）。
- 會員功能是自己實作的（`backend/internal/auth`），設計見 [docs/architecture.md](docs/architecture.md)。API：`POST /api/auth/register`、`POST /api/auth/login`、`POST /api/auth/logout`、`GET /api/auth/me`、`POST /api/auth/password/forgot`、`POST /api/auth/password/reset`、`POST /api/auth/password/change`。`/api/health` 不需要登入。
- 跨站的 POST 會被擋下（回 403，Go 內建的 `http.CrossOriginProtection`）。

## 資料庫 migration 與 sqlc

- 資料表定義在 `backend/db/migrations/`（goose 格式），後端啟動時自動套用尚未執行的 migration。
- SQL 查詢寫在 `backend/db/queries/`，改完後產生 Go 程式碼（sqlc 版本固定在 `go.mod`，第一次執行要編譯約 1 分鐘；產出要進版控）：

```bash
cd backend && go tool sqlc generate
```

## 測試

在專案根目錄執行。`go.mod` 在 `backend/`，所以要加 `-C backend`；單元測試用假物件，不需要開 Docker：

```bash
go -C backend vet ./...
go -C backend test ./...
pnpm -C frontend test
```

本機沒有安裝 Go 的話，改在後端容器裡執行（容器要先啟動）：

```bash
docker-compose exec backend go vet ./...
docker-compose exec backend go test ./...
```

需要資料庫的整合測試要設定 `TEST_DATABASE_URL`，沒設定的話會略過（後端容器裡已經設好）：

```bash
TEST_DATABASE_URL='postgres://admin:<密碼>@127.0.0.1:5433/nest_egg?sslmode=disable' go -C backend test ./...
```

## 不透過 Docker 跑後端或前端（看個人選用）

資料庫一樣用 Docker（`docker-compose up -d db`），程式改在本機跑：

```bash
# 後端：air 會讀 backend/.env（第一次先 cp .env.example .env 並填入密碼）
cd backend && go run github.com/air-verse/air@v1.67.4

# 前端：預設把 /api 轉到 http://localhost:8080
cd frontend && pnpm install && pnpm dev
```

本機的 pnpm 版本要和 `frontend/package.json` 的 `packageManager`（12.10.1）一致；用 Homebrew 安裝的執行 `brew upgrade pnpm`。
