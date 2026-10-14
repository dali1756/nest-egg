#!/bin/bash
# 端對端檢查開發環境：瀏覽器 --> Vite proxy --> Go --> PostgreSQL。
#
# 用法：bash .claude/skills/nest-egg-verify/scripts/verify-stack.sh
#   環境變數：BACKEND_URL（預設 http://127.0.0.1:8080）、FRONTEND_URL（預設 http://127.0.0.1:5173）、
#             WAIT_TIMEOUT（等待 HTTP 回應的秒數，預設 90）
#   回傳：0 全部通過、1 有步驟失敗（失敗時立即停止）。
#
# 反向測試會暫停 db；script 結束時（含失敗、Ctrl-C）一定把 db 啟動回來。
# 絕不執行 docker compose down（-v 會清空資料庫）。
set -u

here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=lib.sh
. "$here/lib.sh"

BACKEND_URL=${BACKEND_URL:-http://127.0.0.1:8080}
FRONTEND_URL=${FRONTEND_URL:-http://127.0.0.1:5173}
WAIT_TIMEOUT=${WAIT_TIMEOUT:-90}
export BACKEND_URL

TOTAL=8
step_no=0
tmp=$(mktemp -d)
# 必須在主 shell 設定：第 8 步若被包進 pipe（| tee）、$( ) 或 ( )，旗標只會設在 subshell，
# trap 看不到 --> 失敗時印出「全部通過」且 db 停著。驗收測試 test_step8_failure_restores_db 會抓這個錯。
db_stopped_by_us=0

# 結束時一定執行：反向測試停掉的 db 要啟動回來
cleanup() {
  if [ "$db_stopped_by_us" -eq 1 ]; then
    echo "還原：重新啟動 db" >&2
    docker compose start db >/dev/null 2>&1 || echo "  db 啟動失敗，請手動執行 docker compose start db" >&2
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

# step <說明>：印出步驟編號與說明，等結果印在同一行
step() {
  step_no=$((step_no + 1))
  printf '[%d/%d] %s ... ' "$step_no" "$TOTAL" "$1"
}

ok() { echo "ok"; }

# fail_step <提示>：標記目前步驟失敗並結束
fail_step() {
  echo "失敗"
  [ -s "$tmp/err" ] && cat "$tmp/err" >&2
  [ -n "${1:-}" ] && echo "  ${1}" >&2
  exit 1
}

# check <失敗提示> <指令...>：指令失敗時結束，並印出指令的錯誤訊息與提示
check() {
  local hint=$1
  shift
  : >"$tmp/err"
  "$@" 2>"$tmp/err" || fail_step "$hint"
}

db_health() {
  docker compose ps --format '{{.Health}}' db 2>/dev/null
}

# wait_for_db_healthy TIMEOUT_SECONDS
wait_for_db_healthy() {
  local waited=0
  while [ "$(db_health)" != healthy ]; do
    if [ "$waited" -ge "$1" ]; then
      echo "  db 沒有在 ${1} 秒內恢復 healthy，目前狀態：$(db_health)" >&2
      return 1
    fi
    sleep 1
    waited=$((waited + 1))
  done
}

backend_logged_connected() {
  docker compose logs backend 2>/dev/null | grep -q 'database connected' && return 0
  echo "  backend log 找不到 database connected" >&2
  return 1
}

step "前置檢查（docker、.env）"
root=$(git rev-parse --show-toplevel 2>/dev/null) || fail_step "請在 nest-egg 專案目錄內執行"
cd "$root" || fail_step
docker info >/dev/null 2>&1 || fail_step "連不到 docker daemon，請先啟動 Docker Desktop"
[ -f .env ] || fail_step "專案根目錄沒有 .env，請先執行 cp .env.example .env 並填入資料庫密碼"
ok

step "啟動環境（docker compose up -d --wait）"
docker compose up -d --wait --wait-timeout 180 >"$tmp/err" 2>&1 || fail_step "看 docker compose logs 找原因"
ok

step "db 狀態為 healthy"
[ "$(db_health)" = healthy ] || fail_step "db 狀態：$(db_health)"
ok

step "後端 health（${BACKEND_URL}/api/health）"
check "後端沒有在 ${WAIT_TIMEOUT} 秒內回 200；看 docker compose logs backend（第一次啟動 air 要編譯，可能比較久）" \
  wait_for_status "${BACKEND_URL}/api/health" 200 "$WAIT_TIMEOUT"
check "" expect_body_eq '{"status":"ok","database":"up"}'
ok

step "backend log 有 database connected"
check "看 docker compose logs backend" backend_logged_connected
ok

step "API smoke test（smoke/*.sh）"
for smoke in "$here"/smoke/*.sh; do
  : >"$tmp/err"
  bash "$smoke" 2>"$tmp/err" || fail_step "失敗的 smoke test：smoke/$(basename "$smoke")"
done
ok

step "前端與 Vite proxy（${FRONTEND_URL}）"
check "Vite 沒有在 ${WAIT_TIMEOUT} 秒內回應；看 docker compose logs frontend" \
  wait_for_status "${FRONTEND_URL}/" 200 "$WAIT_TIMEOUT"
check "首頁不是 Vite 的 index.html" expect_body_has '<div id="root">'
request GET "${FRONTEND_URL}/api/health"
check "Vite proxy 轉不到後端；檢查 frontend 的 API_PROXY_TARGET" expect_status 200
ok

step "反向測試（停止 db --> 503 --> 啟動 db --> 200）"
# 這一段不可包進 pipe、$( ) 或 ( )（原因見 db_stopped_by_us 的註解）
db_stopped_by_us=1
docker compose stop db >"$tmp/err" 2>&1 || fail_step "docker compose stop db 失敗"
check "停掉 db 後 health 沒有變成 503" wait_for_status "${BACKEND_URL}/api/health" 503 20
check "" expect_body_eq '{"status":"unavailable","database":"down"}'
docker compose start db >"$tmp/err" 2>&1 || fail_step "docker compose start db 失敗"
db_stopped_by_us=0
check "" wait_for_db_healthy "$WAIT_TIMEOUT"
check "db 恢復後 health 沒有回到 200（後端應自動重連，不用重啟）" \
  wait_for_status "${BACKEND_URL}/api/health" 200 "$WAIT_TIMEOUT"
ok

echo "全部通過"
