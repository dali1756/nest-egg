#!/bin/bash
# GET /api/health：後端與資料庫都正常時回 200 與 {"status":"ok","database":"up"}
set -u
. "$(dirname "$0")/../lib.sh"
BACKEND_URL=${BACKEND_URL:-http://127.0.0.1:8080}

request GET "${BACKEND_URL}/api/health"
expect_status 200 && expect_body_eq '{"status":"ok","database":"up"}'
