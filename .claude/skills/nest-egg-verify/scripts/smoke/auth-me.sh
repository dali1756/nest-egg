#!/bin/bash
# GET /api/auth/me：未帶 session cookie 時必須回 401（會員 API 有掛上、且有保護）
set -u
. "$(dirname "$0")/../lib.sh"
BACKEND_URL=${BACKEND_URL:-http://127.0.0.1:8080}

request GET "${BACKEND_URL}/api/auth/me"
expect_status 401
