#!/bin/bash
# verify-stack.sh 的驗收測試：對真的 docker compose 環境執行，含故障模擬與中斷測試。
# 用法：bash .claude/skills/nest-egg-verify/scripts/verify-stack.acceptance.sh
# 注意：會啟動整個開發環境，並暫停 db 數秒；不會執行 docker compose down，也不會動到 .env。
set -u

here=$(cd "$(dirname "$0")" && pwd)
script="$here/verify-stack.sh"
root=$(git -C "$here" rev-parse --show-toplevel) || exit 2
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
failures=0

pass() { echo "ok   $1"; }
fail() { echo "FAIL $1"; failures=$((failures + 1)); }

# run_verify <工作目錄> [環境變數...]：執行 verify-stack.sh，結果放在 out、code
run_verify() {
  local dir=$1
  shift
  out=$(cd "$dir" && env "$@" bash "$script" 2>&1)
  code=$?
}

# -a：停止的容器也要列出來（狀態 exited）
db_state() {
  (cd "$root" && docker compose ps -a --format '{{.State}}' db 2>/dev/null)
}

test_missing_env() {
  local repo="$tmp/no-env"
  mkdir -p "$repo"
  git -C "$repo" init -q
  run_verify "$repo"
  if [ "$code" -eq 1 ] && echo "$out" | grep -q '\[1/8\]' && echo "$out" | grep -q 'cp .env.example .env' &&
    ! echo "$out" | grep -q '\[2/8\]'; then
    pass "缺少 .env：第 1 步失敗並提示 cp .env.example .env"
  else
    fail "缺少 .env：code=${code}，輸出："
    echo "$out"
  fi
}

test_backend_unreachable() {
  run_verify "$root" BACKEND_URL=http://127.0.0.1:9 WAIT_TIMEOUT=3
  if [ "$code" -eq 1 ] && echo "$out" | grep -q '\[4/8\].*失敗' && ! echo "$out" | grep -q '\[5/8\]'; then
    pass "後端沒有回應：第 4 步失敗並停止"
  else
    fail "後端沒有回應：code=${code}，輸出："
    echo "$out"
  fi
}

test_frontend_unreachable() {
  run_verify "$root" FRONTEND_URL=http://127.0.0.1:9 WAIT_TIMEOUT=3
  if [ "$code" -eq 1 ] && echo "$out" | grep -q '\[7/8\].*失敗' && ! echo "$out" | grep -q '\[8/8\]'; then
    pass "前端沒有回應：第 7 步失敗，沒有進行反向測試"
  else
    fail "前端沒有回應：code=${code}，輸出："
    echo "$out"
  fi
}

test_full_pass() {
  run_verify "$root"
  local states
  states=$(cd "$root" && docker compose ps --format '{{.Service}}={{.State}}' | sort | tr '\n' ' ')
  if [ "$code" -eq 0 ] && echo "$out" | grep -q '\[8/8\].*ok' && echo "$out" | grep -q '全部通過' &&
    [ "$states" = "backend=running db=running frontend=running " ]; then
    pass "環境正常：8 步全部通過，結束後所有服務仍在執行"
  else
    fail "環境正常：code=${code}，服務狀態：${states}，輸出："
    echo "$out"
  fi
}

# 第 8 步停掉 db 之後失敗：BACKEND_URL 指向永遠回 200 的假後端，「等 503」就會逾時。
# 確認 script 回 1、不印「全部通過」，並由 EXIT trap 把真的 db 啟動回來。
# 這能抓到「第 8 步被包進 pipe、$( ) 或 ( )，db_stopped_by_us 只設在 subshell」的錯誤。
# 假後端用 python3（Xcode Command Line Tools 內附，macOS 的 git 也需要它）。
test_step8_failure_restores_db() {
  if ! command -v python3 >/dev/null; then
    fail "第 8 步失敗：找不到 python3，請先執行 xcode-select --install"
    return
  fi
  local www="$tmp/fake-backend" port=18080 i
  mkdir -p "$www/api"
  printf '{"status":"ok","database":"up"}' >"$www/api/health"
  python3 -m http.server "$port" --bind 127.0.0.1 --directory "$www" >/dev/null 2>&1 &
  local srv=$!
  for i in $(seq 1 50); do
    [ "$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/api/health")" = 200 ] && break
    sleep 0.1
  done
  run_verify "$root" BACKEND_URL="http://127.0.0.1:${port}"
  kill "$srv" 2>/dev/null
  wait "$srv" 2>/dev/null
  local state
  state=$(db_state)
  if [ "$code" -eq 1 ] && echo "$out" | grep -q '\[8/8\].*失敗' && ! echo "$out" | grep -q '^全部通過' &&
    echo "$out" | grep -q '還原：重新啟動 db' && [ "$state" = running ]; then
    pass "第 8 步停掉 db 後失敗：回傳 1、不印全部通過，並由 trap 把 db 啟動回來"
  else
    fail "第 8 步失敗：code=${code}，db 狀態=${state}，輸出："
    echo "$out"
  fi
}

# 反向測試一開始（[8/8] 出現）就送 SIGINT（Ctrl-C）。script 會先等 docker compose stop db 跑完
# 才處理訊號，所以結束時 db 一定是被它停掉的狀態，必須由 EXIT trap 啟動回來。
# 非互動 shell 的背景程式預設忽略 SIGINT，所以用 set -m 讓它能收到。
test_interrupt_restores_db() {
  set -m
  (cd "$root" && exec bash "$script") >"$tmp/interrupt.log" 2>&1 &
  local pid=$! i
  for i in $(seq 1 1800); do
    grep -q '\[8/8\]' "$tmp/interrupt.log" && break
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.1
  done
  if ! grep -q '\[8/8\]' "$tmp/interrupt.log"; then
    set +m
    kill "$pid" 2>/dev/null
    fail "中斷測試：script 沒有進到反向測試，輸出："
    cat "$tmp/interrupt.log"
    return
  fi
  kill -INT "$pid"
  wait "$pid"
  local code=$?
  set +m
  local state
  state=$(db_state)
  if [ "$code" -ne 0 ] && [ "$state" = running ] && grep -q '還原：重新啟動 db' "$tmp/interrupt.log" &&
    ! grep -q '全部通過' "$tmp/interrupt.log"; then
    pass "反向測試中按 Ctrl-C：script 中止，並由 trap 把 db 啟動回來"
  else
    fail "中斷測試：code=${code}，db 狀態=${state}，輸出："
    cat "$tmp/interrupt.log"
  fi
}

# 可以指定只跑某些測試：verify-stack.acceptance.sh test_interrupt_restores_db
if [ $# -gt 0 ]; then
  for t in "$@"; do "$t"; done
else
  test_missing_env
  test_full_pass
  test_backend_unreachable
  test_frontend_unreachable
  test_step8_failure_restores_db
  test_interrupt_restores_db
fi

if [ "$failures" -gt 0 ]; then
  echo "$failures 個測試失敗"
  exit 1
fi
echo "全部通過"
