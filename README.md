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

| 服務 | 位址 | 說明 |
|---|---|---|
| frontend | http://localhost:5173 | Vite 開發伺服器，`/api` 轉給後端；存檔後畫面即時更新（HMR） |
| backend | http://localhost:8080 | Go API；存檔後 air 自動重新編譯、重啟 |
| db | `localhost:5433` | PostgreSQL 18，帳密見 `.env`；容器之間用 `db:5432` |

常用指令：

| 指令 | 用途 |
|---|---|
| `docker-compose up -d` | 在背景啟動 |
| `docker-compose logs -f backend` | 看後端 log |
| `docker-compose down` | 停止並移除容器，資料庫資料保留 |
| `docker-compose down -v` | 連 volume 一起刪除，**資料庫資料會清空** |
| `docker-compose up --build` | 改了 Dockerfile 之後重建映像 |
| `docker-compose exec db psql -U admin -d nest_egg` | 進資料庫下 SQL |

## 確認後端真的連上資料庫

1. `docker-compose ps`：db 顯示 `(healthy)`。
2. `docker-compose logs backend`：有 `database connected ... user=admin`。後端啟動時會先 ping 資料庫，帳密或主機設定錯誤會直接啟動失敗並印出原因。
3. `curl -i http://localhost:8080/api/health`：回 `200` 與 `{"status":"ok","database":"up"}`。每次請求都會實際 ping 資料庫。
4. 反向測試：`docker-compose stop db` 後再 curl 一次，應該變成 `503` 與 `{"status":"unavailable","database":"down"}`；`docker compose start db` 後恢復 `200`，後端不用重啟。
5. 瀏覽器開 http://localhost:5173：畫面顯示後端與資料庫的狀態，走的是「瀏覽器 --> Vite proxy --> Go --> PostgreSQL」整條路徑。

以上步驟可以一次自動跑完（會先 `docker compose up -d`；反向測試會暫停 db 幾秒，結束時自動恢復）：

```bash
bash .claude/skills/nest-egg-verify/scripts/verify-stack.sh
```

## 測試

```bash
docker compose exec backend go test ./...
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
