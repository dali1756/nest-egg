---
name: nest-egg-verify
description: Use when the user runs /nest-egg-verify to check that the nest-egg docker compose dev environment works end to end (browser --> Vite proxy --> Go --> PostgreSQL), after changes to docker-compose.yml, Dockerfiles or .env, or when adding an API smoke test.
disable-model-invocation: true
---

# nest-egg verify

`verify-stack.sh` 依序檢查 8 步，任一步失敗就停止並印出原因。第 8 步會暫停 db 幾秒做反向測試，script 結束時（含失敗、Ctrl-C）一定會把 db 啟動回來。

## 執行

```bash
bash .claude/skills/nest-egg-verify/scripts/verify-stack.sh
```

可用環境變數覆寫：`BACKEND_URL`（預設 `http://127.0.0.1:8080`）、`FRONTEND_URL`（預設 `http://127.0.0.1:5173`）、`WAIT_TIMEOUT`（等待 HTTP 的秒數，預設 90）。

回報每一步的結果。失敗時依下表判斷原因：唯讀的診斷指令（`logs`、`ps`、`curl`）可以直接跑；會改變環境的指令（`ALTER ROLE`、`restart`、`--build`）要先說明再做；刪除 volume 只在使用者明確要求、而且先 `pg_dump` 備份之後才做。

## 失敗時怎麼判斷

| 步驟 | 常見原因 | 怎麼查、怎麼修 |
|---|---|---|
| 1 | Docker Desktop 沒開；沒有 `.env` | 開 Docker Desktop；`cp .env.example .env` 並填密碼 |
| 2 | port 被占用（5433、8080、5173）；映像建置失敗 | `docker compose logs`；`lsof -i :8080`；改過 Dockerfile 要 `docker compose up -d --build` |
| 3 | db 起不來 | `docker compose logs db`。注意：healthcheck 用 `pg_isready`，**不檢查密碼**，這步通過不代表帳密正確 |
| 4，狀態碼 `000` | 後端沒在聽：編譯錯誤，或啟動時連不到 db 就結束 | `docker compose logs backend`。出現 `28P01 password authentication failed` 代表改過 `.env` 的 `POSTGRES_PASSWORD`，但 volume 還是舊密碼（只在第一次初始化時套用）--> 用下方指令改密碼，不要清 volume |
| 4，狀態碼 `503` | 後端在跑，但 ping 不到 db | `docker compose logs db backend` |
| 5 | backend log 沒有 `database connected` | `docker compose logs backend` |
| 6 | API 行為和 smoke test 預期不同 | 看印出的預期與實際回應 |
| 7 | Vite 沒起來，或 proxy 設錯 | `docker compose logs frontend`；compose 裡的 `API_PROXY_TARGET` 應為 `http://backend:8080` |
| 8 | 停掉 db 後沒變 503，或恢復後沒回 200 | 後端 bug（health 沒真的 ping，或連線池沒重連），回報給使用者 |

`28P01` 的修法：把資料庫密碼改成 `.env` 的新密碼。前提是 db 容器已經用新的 `.env` 重建過（verify-stack 第 2 步的 `up -d` 會做這件事）。密碼不會出現在指令列。

```bash
docker compose exec -T db sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v pw="$POSTGRES_PASSWORD"' <<'SQL'
ALTER ROLE CURRENT_USER WITH PASSWORD :'pw';
SQL
docker compose restart backend
```

使用者明確要求清空資料庫時：先備份到專案外面，確認備份不是 0 bytes，再只刪資料庫的 volume（保留 Go、node_modules 快取）。

```bash
mkdir -p ~/nest-egg-backups
docker compose exec -T db sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > ~/nest-egg-backups/nest_egg-$(date +%Y%m%d-%H%M%S).dump
ls -lh ~/nest-egg-backups/
docker compose rm -s -f db
docker volume rm nest-egg_pgdata
docker compose up -d --wait
docker compose restart backend   # 後端在 air 裡，連不上 db 時程式會結束但容器還在，要重啟
# 還原：docker compose exec -T db sh -c 'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < <備份檔>
```

## 新增 API smoke test

1. 在 `scripts/unit.test.sh` 用 `respond` 和假 curl 寫測試：正常回應、錯誤狀態碼、內容不符各一個。執行後確認看到紅燈。
2. 新增 `scripts/smoke/<name>.sh`：`. "$(dirname "$0")/../lib.sh"`，用 `BACKEND_URL`、`request`、`expect_*`。`verify-stack.sh` 第 6 步會自動執行所有 `smoke/*.sh`。
3. smoke test 只呼叫 GET，不在真實資料庫寫入或刪除資料；寫入類 API 用 Go 的 `httptest` 測。
4. 金額、匯率欄位要確認是字串，例如 `expect_body_has '"foreign_amount":"'`。
5. 幣別這類資料清單只檢查必要的項目，不比對整個回應。
6. 執行 `bash .claude/skills/nest-egg-verify/scripts/unit.test.sh` 確認轉綠，再跑 `verify-stack.sh`。

## Gotchas

- 絕不執行 `docker compose down -v`：會清空資料庫 volume，也會刪掉 Go 和 node_modules 的快取 volume。
- 資料庫從主機連用 `127.0.0.1:5433`（`.env` 的 `POSTGRES_PORT`），容器之間用 `db:5432`。
- 用 `127.0.0.1`，不用 `localhost`：compose 只綁 IPv4，`localhost` 可能解析成 `::1`。
- 第一次啟動時 air 要編譯，第 4 步可能等比較久，可以調高 `WAIT_TIMEOUT`。
- `docker compose ps` 不會列出已停止的容器，要加 `-a`。
- 不要把 `verify-stack.sh` 的第 8 步包進 pipe（例如 `| tee` 留 log）、`$( )` 或 `( )`：`db_stopped_by_us` 會只設在 subshell，失敗時反而印出「全部通過」，db 也停著沒恢復。
- 修改 `verify-stack.sh` 前，先在 `scripts/verify-stack.acceptance.sh` 加情境並看到紅燈。它會操作真的 docker、暫停 db，可以指定只跑某個測試：`bash .claude/skills/nest-egg-verify/scripts/verify-stack.acceptance.sh test_full_pass`。
- script 只能用 bash 3.2 語法；變數後面接中文時寫成 `${var}`。
