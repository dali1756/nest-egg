#!/bin/bash
# GET /api/auth/me：未帶 session cookie 時必須回 401（API 有登入保護）。
# /api/ 底下的非公開路徑都會先經過 RequireUser，所以就算會員路徑沒掛上也是 401；
# 「會員路徑有掛上」由 backend/cmd/server/main_test.go 的登入測試確認。smoke test 只呼叫 GET，不寫入資料。
set -u
. "$(dirname "$0")/../lib.sh"
BACKEND_URL=${BACKEND_URL:-http://127.0.0.1:8080}

request GET "${BACKEND_URL}/api/auth/me"
expect_status 401
