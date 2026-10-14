# shellcheck shell=bash
# HTTP 檢查共用函式，給 verify-stack.sh 與 smoke/*.sh 使用。
# 用法：. "$(dirname "$0")/lib.sh"（smoke/ 底下用 "$(dirname "$0")/../lib.sh"）
# 只用 bash 3.2 語法；變數後面接中文時要寫成 ${var}。

# request METHOD URL [BODY]：送出請求，結果放在 STATUS（連線失敗為 000）與 BODY。
# 一律回傳 0，讓呼叫端用 expect_* 判斷結果。
request() {
  local method=$1 url=$2
  local out
  out=$(mktemp)
  local args=(-sS -o "$out" -w '%{http_code}' --max-time 10 -X "$method")
  if [ $# -ge 3 ]; then
    args=("${args[@]}" -H 'Content-Type: application/json' --data "$3")
  fi
  STATUS=$(curl "${args[@]}" "$url" 2>/dev/null)
  [ -n "$STATUS" ] || STATUS=000
  BODY=$(cat "$out")
  rm -f "$out"
  return 0
}

# expect_status CODE：STATUS 等於 CODE 回 0；否則在 stderr 印出預期與實際並回 1
expect_status() {
  [ "$STATUS" = "$1" ] && return 0
  echo "  預期狀態碼 ${1}，實際 ${STATUS}；回應：${BODY}" >&2
  return 1
}

# expect_body_eq TEXT：BODY 完全等於 TEXT
expect_body_eq() {
  [ "$BODY" = "$1" ] && return 0
  echo "  預期回應：${1}" >&2
  echo "  實際回應：${BODY}" >&2
  return 1
}

# expect_body_has TEXT：BODY 包含 TEXT
expect_body_has() {
  case "$BODY" in
    *"$1"*) return 0 ;;
  esac
  echo "  回應中找不到：${1}" >&2
  echo "  實際回應：${BODY}" >&2
  return 1
}

# wait_for_status URL CODE TIMEOUT_SECONDS：每秒 GET 一次，直到狀態碼等於 CODE 回 0；
# 逾時回 1，並在 stderr 印出最後一次的狀態碼與回應
wait_for_status() {
  local url=$1 want=$2 timeout=$3 waited=0
  while :; do
    request GET "$url"
    [ "$STATUS" = "$want" ] && return 0
    [ "$waited" -ge "$timeout" ] && break
    sleep 1
    waited=$((waited + 1))
  done
  echo "  等待 ${url} 回 ${want} 逾時（${timeout} 秒），最後一次狀態碼 ${STATUS}；回應：${BODY}" >&2
  return 1
}
