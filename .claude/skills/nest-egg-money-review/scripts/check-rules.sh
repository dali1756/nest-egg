#!/bin/bash
# 檢查程式碼是否違反 CLAUDE.md 的設計規則（金額、時區、幣別、log、外部 API）。
#
# 用法：check-rules.sh [--root <dir>]
#   預設掃描 git 專案根目錄下的 backend/（*.go、*.sql）與 frontend/src/（*.ts、*.tsx）。
#   每個違規輸出一行 path:line: [rule-id] 訊息。
#   回傳：0 沒有違規、1 有違規、2 參數錯誤。
#
# 例外：確定不是問題的行，在同一行加註解 money-rules:allow <rule-id>: 原因
# 新增規則：在下方加一行 check，並在 testdata/ 補 fixture 與 expected.txt（check-rules.test.sh）
#
# 只用 bash 3.2 與 BSD grep 支援的語法（macOS 內建），兩台開發電腦都不用另外安裝工具。
set -u

usage() {
  echo "用法：$(basename "$0") [--root <dir>]" >&2
}

root=""
while [ $# -gt 0 ]; do
  case "$1" in
    --root)
      [ $# -ge 2 ] || { usage; exit 2; }
      root=$2
      shift 2
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

if [ -z "$root" ]; then
  root=$(git rev-parse --show-toplevel 2>/dev/null) || {
    echo "找不到 git 專案根目錄，請用 --root 指定" >&2
    usage
    exit 2
  }
fi
if [ ! -d "$root" ]; then
  echo "目錄不存在：${root}" >&2
  usage
  exit 2
fi
cd "$root" || exit 2

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
: >"$tmp/findings"

# find_files <dir> <name-glob>...：列出要掃描的檔案（相對於 root），一行一個
find_files() {
  local dir=$1
  shift
  [ -d "$dir" ] || return 0
  local names=(-name "$1")
  shift
  local g
  for g in "$@"; do
    names=("${names[@]}" -o -name "$g")
  done
  # 略過依賴與建置產物
  find "$dir" -type d \( -name node_modules -o -name dist -o -name tmp -o -name bin \) -prune \
    -o -type f \( "${names[@]}" \) -print
}

find_files backend '*.go' >"$tmp/go"
find_files backend '*.sql' >"$tmp/sql"
find_files frontend/src '*.ts' '*.tsx' >"$tmp/ts"
# 測試檔可以寫死幣別當測試資料
{
  grep -v '_test\.go$' "$tmp/go"
  grep -vE '\.(test|spec)\.tsx?$' "$tmp/ts"
} >"$tmp/code-nontest"

# check <rule-id> <檔案清單> <regex> <訊息> [<unless-regex>]
#   檔案清單：go、sql、ts、code-nontest（go 與 ts，不含測試檔）
#   unless-regex：同一行也符合時不算違規
#   sql 不分大小寫，並略過 -- 開頭的註解行
check() {
  local id=$1 kind=$2 regex=$3 msg=$4 unless=${5:-'^$'}
  local icase=() comment='^$'
  [ -s "$tmp/$kind" ] || return 0
  if [ "$kind" = sql ]; then
    icase=(-i)
    comment='^[^:]*:[0-9]+:[[:space:]]*--'
  fi
  tr '\n' '\0' <"$tmp/$kind" |
    xargs -0 grep -nHE ${icase[@]+"${icase[@]}"} -e "$regex" -- |
    grep -vE "$comment" |
    grep -vE ${icase[@]+"${icase[@]}"} -e "$unless" |
    grep -vE "money-rules:allow ${id}([^a-z0-9-]|\$)" |
    awk -F: -v id="$id" -v msg="$msg" '{ print $1 ":" $2 ": [" id "] " msg }' >>"$tmp/findings"
}

# 1. 金額不用 float
check float-go go '\bfloat(32|64)\b|strconv\.ParseFloat|decimal\.NewFromFloat|\.InexactFloat64\(|\.Float64\(\)' \
  '金額與匯率不得用 float，改用 decimal.Decimal'
check json-number go 'MarshalJSONWithoutQuotes' \
  'decimal 在 JSON 中必須是字串，不可關閉引號'
check float-sql sql '\b(real|double precision|float|float4|float8)\b' \
  '金額與匯率欄位用 NUMERIC，不用浮點型別'
check number-frontend ts '(^|[^A-Za-z0-9_$.])Number\(|parseFloat\(|\.toFixed\(|z\.(coerce\.)?number\(' \
  '前端金額保持字串，不轉成 number（顯示用 Intl.NumberFormat 格式化字串）'

# 2. 日期以 Asia/Taipei 計算
# time.Parse 的 layout 沒有時區時會解析成 UTC；layout 有 Z07、-0700、MST 的不算
check utc-day-go go 'Truncate\(\s*24\s*\*\s*time\.Hour\s*\)|time\.Now\(\)\.(Format|Date|Year|Month|Day|YearDay|Weekday)\(|time\.Parse\(\s*("2006-01|time\.(DateOnly|DateTime))' \
  '日期要用 Asia/Taipei：time.Now().In(loc)、time.ParseInLocation（容器時區是 UTC）' \
  'Z07|-07:?00|MST'
for main in backend/cmd/*/main.go; do
  [ -f "$main" ] || continue
  grep -q '_ "time/tzdata"' "$main" ||
    echo "${main}:1: [tzdata-import] main 必須 import _ \"time/tzdata\"，否則在沒有系統時區資料的環境（alpine、scratch）載入 Asia/Taipei 會失敗" >>"$tmp/findings"
done
check utc-day-sql sql 'date_trunc\(|::date\b|\bcurrent_date\b' \
  "日期要先轉台北時間：AT TIME ZONE 'Asia/Taipei'" \
  'at time zone'
check utc-day-frontend ts 'toISOString\(\)\.(slice|substring|substr|split)\(' \
  'toISOString() 是 UTC 日期；台北日期用 Intl.DateTimeFormat 搭配 timeZone: Asia/Taipei'

# 3. 平均成本 = SUM(付出 + 手續費) ÷ SUM(取得)
check avg-rate-sql sql '\bavg\s*\([^)]*rate' \
  '不可把匯率直接平均；平均成本 = SUM(pay_amount + fee) / SUM(buy_amount)'

# 4. 幣別是資料
check currency-list code-nontest "[\"'\`][A-Z]{3}[\"'\`]\s*,\s*[\"'\`][A-Z]{3}[\"'\`]" \
  '幣別是資料：從 currencies 表查詢，不要寫死清單'
check currency-list sql "\bin\s*\(\s*'[A-Z]{3}'" \
  '幣別是資料：用外鍵指向 currencies 表，不要用 IN 寫死清單'

# 5. log 不記機密
check secret-log go 'slog\.[A-Za-z]+\(.*(DatabaseURL|DATABASE_URL|[Pp]assword|PASSWORD|[Tt]oken|TOKEN|[Aa]pi_?[Kk]ey|API_?KEY|[Ss]ecret|SECRET)' \
  'log 不可記密碼、token、API key、完整的 DATABASE_URL'

# 6. 只有後端呼叫外部 API
check frontend-external-api ts "(fetch|axios(\.[a-z]+)?)\(\s*[\"'\`]https?://|er-api\.com|twse\.com\.tw|rate\.bot\.com\.tw|alphavantage|finnhub|VITE_[A-Z0-9_]*(KEY|TOKEN|SECRET)" \
  '前端只呼叫自己的 /api；外部資料來源與 API key 只放在後端（RateProvider）'

if [ -s "$tmp/findings" ]; then
  LC_ALL=C sort -t: -k1,1 -k2,2n -k3,3 "$tmp/findings"
  echo "money-rules: $(wc -l <"$tmp/findings" | tr -d ' ') 個違規" >&2
  exit 1
fi
echo "money-rules: 沒有違規" >&2
exit 0
