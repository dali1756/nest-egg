#!/bin/bash
# lib.sh 與 smoke/*.sh 的單元測試：用 testdata/bin/curl（假 curl）取代真的 server。
# 用法：.claude/skills/nest-egg-verify/scripts/unit.test.sh
set -u

here=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
export PATH="$here/testdata/bin:$PATH"
export FAKE_CURL_LOG="$tmp/curl.log"
export FAKE_CURL_RESPONSES="$tmp/responses"
TAB=$'\t'
failures=0

# shellcheck source=lib.sh
. "$here/lib.sh"

pass() { echo "ok   $1"; }
fail() { echo "FAIL $1"; failures=$((failures + 1)); }

# respond <狀態碼<TAB>內容>...：設定假 curl 依序回應的內容，並清空呼叫紀錄
respond() {
  printf '%s\n' "$@" >"$FAKE_CURL_RESPONSES"
  : >"$FAKE_CURL_LOG"
}

test_request_success() {
  respond "200${TAB}{\"status\":\"ok\",\"database\":\"up\"}"
  STATUS="" BODY=""
  request GET http://127.0.0.1:8080/api/health
  local code=$?
  if [ "$code" -eq 0 ] && [ "$STATUS" = 200 ] && [ "$BODY" = '{"status":"ok","database":"up"}' ] &&
    [ "$(cat "$FAKE_CURL_LOG")" = "GET http://127.0.0.1:8080/api/health " ]; then
    pass "request：成功時 STATUS、BODY 正確，且以 GET 呼叫指定 URL"
  else
    fail "request 成功：code=${code} STATUS=${STATUS} BODY=${BODY} 呼叫紀錄=$(cat "$FAKE_CURL_LOG")"
  fi
}

test_request_connection_refused() {
  respond "000${TAB}"
  STATUS="" BODY="stale"
  request GET http://127.0.0.1:9/api/health
  local code=$?
  if [ "$code" -eq 0 ] && [ "$STATUS" = 000 ] && [ -z "$BODY" ]; then
    pass "request：連線失敗時 STATUS=000、BODY 清空，且不讓 script 中止"
  else
    fail "request 連線失敗：code=${code} STATUS=${STATUS} BODY=${BODY}"
  fi
}

test_request_sends_method_and_body() {
  respond "201${TAB}{}"
  request POST http://127.0.0.1:8080/api/fx-trades '{"buy_amount":"1000"}'
  if [ "$(cat "$FAKE_CURL_LOG")" = 'POST http://127.0.0.1:8080/api/fx-trades {"buy_amount":"1000"}' ] &&
    [ "$STATUS" = 201 ]; then
    pass "request：送出指定的方法與 body"
  else
    fail "request 方法與 body：呼叫紀錄=$(cat "$FAKE_CURL_LOG") STATUS=${STATUS}"
  fi
}

# check_expect <說明> <預期回傳值> <stderr 須包含的文字...> -- <指令...>
check_expect() {
  local desc=$1 want=$2
  shift 2
  local needles=()
  while [ "$1" != "--" ]; do
    needles=("${needles[@]+"${needles[@]}"}" "$1")
    shift
  done
  shift
  "$@" 2>"$tmp/stderr"
  local code=$?
  local err n ok=1
  err=$(cat "$tmp/stderr")
  [ "$code" -eq "$want" ] || ok=0
  if [ "$want" -eq 0 ] && [ -n "$err" ]; then ok=0; fi
  for n in ${needles[@]+"${needles[@]}"}; do
    echo "$err" | grep -qF -- "$n" || ok=0
  done
  if [ "$ok" -eq 1 ]; then
    pass "$desc"
  else
    fail "${desc}：code=${code}（預期 ${want}），stderr=${err}"
  fi
}

test_expect_functions() {
  STATUS=200 BODY='{"status":"ok","database":"up"}'
  check_expect "expect_status 符合回 0、不印訊息" 0 -- expect_status 200
  check_expect "expect_body_eq 符合回 0" 0 -- expect_body_eq '{"status":"ok","database":"up"}'
  check_expect "expect_body_has 包含回 0" 0 -- expect_body_has '"database":"up"'

  STATUS=503 BODY='{"status":"unavailable","database":"down"}'
  check_expect "expect_status 不符回 1，印出預期與實際" 1 200 503 -- expect_status 200
  check_expect "expect_body_eq 不符回 1，印出預期與實際" 1 '"ok"' '"unavailable"' -- \
    expect_body_eq '{"status":"ok","database":"up"}'
  check_expect "expect_body_has 不包含回 1，印出要找的文字與實際內容" 1 '"database":"up"' '"database":"down"' -- \
    expect_body_has '"database":"up"'
}

test_wait_for_status_until_ready() {
  respond "000${TAB}" "503${TAB}{\"status\":\"unavailable\",\"database\":\"down\"}" "200${TAB}{\"status\":\"ok\",\"database\":\"up\"}"
  wait_for_status http://127.0.0.1:8080/api/health 200 10 2>"$tmp/stderr"
  local code=$?
  local calls
  calls=$(wc -l <"$FAKE_CURL_LOG" | tr -d ' ')
  if [ "$code" -eq 0 ] && [ "$calls" -eq 3 ] && [ "$STATUS" = 200 ]; then
    pass "wait_for_status：輪詢到 200 就回 0（呼叫 3 次）"
  else
    fail "wait_for_status 成功：code=${code} 呼叫次數=${calls} STATUS=${STATUS} stderr=$(cat "$tmp/stderr")"
  fi
}

test_wait_for_status_timeout() {
  respond "503${TAB}{\"status\":\"unavailable\",\"database\":\"down\"}"
  local start end
  start=$(date +%s)
  wait_for_status http://127.0.0.1:8080/api/health 200 2 2>"$tmp/stderr"
  local code=$?
  end=$(date +%s)
  local elapsed=$((end - start))
  if [ "$code" -eq 1 ] && [ "$elapsed" -ge 2 ] && [ "$elapsed" -le 4 ] && grep -q '503' "$tmp/stderr"; then
    pass "wait_for_status：逾時回 1（約 ${elapsed} 秒），並印出最後的狀態碼"
  else
    fail "wait_for_status 逾時：code=${code} 經過 ${elapsed} 秒 stderr=$(cat "$tmp/stderr")"
  fi
}

test_smoke_health() {
  respond "200${TAB}{\"status\":\"ok\",\"database\":\"up\"}"
  BACKEND_URL=http://127.0.0.1:18080 bash "$here/smoke/health.sh" >/dev/null 2>"$tmp/stderr"
  local code=$?
  if [ "$code" -eq 0 ] && [ "$(cat "$FAKE_CURL_LOG")" = "GET http://127.0.0.1:18080/api/health " ]; then
    pass "smoke/health.sh：200 且內容正確時回 0，並使用 BACKEND_URL"
  else
    fail "smoke/health.sh 正常：code=${code} 呼叫紀錄=$(cat "$FAKE_CURL_LOG") stderr=$(cat "$tmp/stderr")"
  fi

  respond "503${TAB}{\"status\":\"unavailable\",\"database\":\"down\"}"
  BACKEND_URL=http://127.0.0.1:18080 bash "$here/smoke/health.sh" >/dev/null 2>"$tmp/stderr"
  code=$?
  if [ "$code" -ne 0 ] && grep -q 503 "$tmp/stderr"; then
    pass "smoke/health.sh：資料庫斷線（503）時回非 0，並印出實際狀態碼"
  else
    fail "smoke/health.sh 異常：code=${code} stderr=$(cat "$tmp/stderr")"
  fi

  respond "200${TAB}{\"status\":\"ok\",\"database\":\"down\"}"
  BACKEND_URL=http://127.0.0.1:18080 bash "$here/smoke/health.sh" >/dev/null 2>"$tmp/stderr"
  code=$?
  if [ "$code" -ne 0 ]; then
    pass "smoke/health.sh：狀態碼 200 但內容不對時回非 0"
  else
    fail "smoke/health.sh 內容不符：預期非 0，實際 code=${code}"
  fi
}

test_request_success
test_request_connection_refused
test_request_sends_method_and_body
test_expect_functions
test_wait_for_status_until_ready
test_wait_for_status_timeout
test_smoke_health

if [ "$failures" -gt 0 ]; then
  echo "$failures 個測試失敗"
  exit 1
fi
echo "全部通過"
