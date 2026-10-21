#!/bin/bash
# GET /api/rates：未帶 session cookie 時必須回 401，抓出「匯率路徑不小心註冊在公開的 mux」。
# 匯率內容由 Go 的 httptest 測試；真實環境抓到的央行資料，由使用者登入後在瀏覽器開這個網址確認。
# smoke test 只呼叫 GET、不登入（登入會新增 session），不寫入資料。
set -u
. "$(dirname "$0")/../lib.sh"
BACKEND_URL=${BACKEND_URL:-http://127.0.0.1:8080}

request GET "${BACKEND_URL}/api/rates?base=USD&quote=TWD"
expect_status 401
