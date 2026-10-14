#!/bin/bash
# check-rules.sh 的測試：對 testdata 執行，比對輸出與 exit code。
# 用法：.claude/skills/nest-egg-money-review/scripts/check-rules.test.sh
set -u

here=$(cd "$(dirname "$0")" && pwd)
script="$here/check-rules.sh"
data="$here/testdata"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
failures=0

pass() { echo "ok   $1"; }
fail() { echo "FAIL $1"; failures=$((failures + 1)); }

# run <args...>：執行 check-rules.sh，結果放在 out、err、code
run() {
  out=$("$script" "$@" 2>"$tmp/stderr")
  code=$?
  err=$(cat "$tmp/stderr")
}

test_clean_has_no_findings() {
  run --root "$data/clean"
  if [ "$code" -eq 0 ] && [ -z "$out" ]; then
    pass "clean：回傳 0 且沒有違規"
  else
    fail "clean：預期回傳 0 且沒有輸出，實際 code=${code}，輸出："
    echo "$out"
  fi
}

# node_modules、dist 會被 .gitignore 忽略，無法放進 testdata，所以執行時才建立
test_skips_build_and_dependency_dirs() {
  local root="$tmp/excluded"
  local d
  mkdir -p "$root/backend/internal"
  echo 'var x float64' >"$root/backend/internal/ok.go"
  for d in node_modules dist tmp bin; do
    mkdir -p "$root/backend/$d"
    echo 'var x float64' >"$root/backend/$d/x.go"
  done
  run --root "$root"
  if [ "$(echo "$out" | cut -d' ' -f1-2)" = "backend/internal/ok.go:1: [float-go]" ]; then
    pass "略過 node_modules、dist、tmp、bin"
  else
    fail "排除目錄：預期只有 backend/internal/ok.go:1，實際："
    echo "$out"
  fi
}

test_unknown_arg_exits_2() {
  run --bogus
  if [ "$code" -eq 2 ] && echo "$err" | grep -q '用法'; then
    pass "未知參數：回傳 2 並印出用法"
  else
    fail "未知參數：預期回傳 2 且 stderr 有「用法」，實際 code=${code}，stderr：${err}"
  fi
}

test_missing_root_exits_2() {
  run --root "$data/does-not-exist"
  if [ "$code" -eq 2 ] && echo "$err" | grep -q '用法'; then
    pass "--root 不存在：回傳 2 並印出用法"
  else
    fail "--root 不存在：預期回傳 2 且 stderr 有「用法」，實際 code=${code}，stderr：${err}"
  fi
}

# violations/expected.txt 是人工核對過的預期結果（path:line: [rule-id]，依 path、行號排序）
test_violations_match_expected() {
  run --root "$data/violations"
  # 去掉訊息，只比對 path:line: [rule-id]
  echo "$out" | sed -E 's/^([^ ]+ \[[a-z0-9-]+\]).*/\1/' >"$tmp/got"
  local same=0
  diff -u "$data/violations/expected.txt" "$tmp/got" >"$tmp/diff" && same=1
  if [ "$code" -eq 1 ] && [ "$same" -eq 1 ]; then
    pass "violations：回傳 1，違規清單與 expected.txt 一致"
  else
    fail "violations：預期回傳 1 且與 expected.txt 一致，實際 code=${code}，差異（- 預期 / + 實際）："
    cat "$tmp/diff"
  fi
}

test_findings_have_message() {
  run --root "$data/violations"
  local bare
  bare=$(echo "$out" | grep -vE '^[^ ]+ \[[a-z0-9-]+\] [^ ]' || true)
  if [ -n "$out" ] && [ -z "$bare" ]; then
    pass "每個違規都附上說明訊息"
  else
    fail "違規缺少說明訊息：${bare:-（沒有任何輸出）}"
  fi
}

test_clean_has_no_findings
test_violations_match_expected
test_findings_have_message
test_skips_build_and_dependency_dirs
test_unknown_arg_exits_2
test_missing_root_exits_2

if [ "$failures" -gt 0 ]; then
  echo "$failures 個測試失敗"
  exit 1
fi
echo "全部通過"
