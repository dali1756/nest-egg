---
name: nest-egg-money-review
description: Use when nest-egg code that touches money amounts, exchange rates, dates or time zones, currencies, or logging has been written or changed, before committing or running /opsx:verify, or when asked to review code against the project's design rules (檢查設計規則、金額、匯率、時區、幣別、review).
---

# nest-egg money review

CLAUDE.md 的設計規則有兩種檢查：`check-rules.sh` 抓單行就看得出來的違規，`references/judgement.md` 列出要讀程式碼才能判斷的項目。兩個都要做，script 通過不代表沒問題。

## 流程

1. 在專案根目錄執行：
   ```bash
   bash .claude/skills/nest-egg-money-review/scripts/check-rules.sh
   ```
2. 逐一處理每個違規：
   - 真的問題 --> 依下方「修正方向」修改程式碼。
   - 誤報（例如路由 id 的 `Number(params.id)`）--> 在同一行加 `money-rules:allow <rule-id>: 原因`。原因必寫，一個註解只豁免一條規則。
   - 無法判斷 --> 問使用者，不要自己加例外。
3. 讀 `references/judgement.md`，對本次變更的檔案（`git status`、`git diff`）逐項檢查。
4. 重跑 script，確認回傳 0。
5. 依下方格式回報。

## 修正方向

| 規則 | 修法 |
|---|---|
| `float-go` / `json-number` | 用 `decimal.Decimal`；從字串建立用 `decimal.NewFromString` |
| `float-sql` | `NUMERIC(20,4)`（金額）、`NUMERIC(20,8)`（匯率） |
| `number-frontend` | 前端只顯示字串；合計、平均、損益改由後端算好回傳字串。不在前端手寫小數運算 |
| `utc-day-*` / `tzdata-import` | `time.Now().In(loc)`、`time.ParseInLocation`；SQL 加 `AT TIME ZONE 'Asia/Taipei'`；前端用 `Intl.DateTimeFormat` 搭配 `timeZone` |
| `avg-rate-sql` | `SUM(pay_amount + fee) / SUM(buy_amount)` |
| `currency-list` | 查 `currencies` 表，或用外鍵 |
| `secret-log` | 只記 ID 與非機密欄位 |
| `frontend-external-api` | 改成呼叫自己的 `/api`，由後端的 `RateProvider` 取資料 |

## 回報格式

```
### money review
- script：N 個違規 --> 已修正 X、已加例外 Y（修完重跑：0）
- 新增例外：path:line [rule-id] 原因（沒有就寫「無」）
- 人工檢查：path:line｜違反的規則｜建議修法（沒有就寫「無」）
- 結論：可以 commit／需先修正上列項目
```

## Gotchas

- `testdata/` 是故意違規的測試資料，不是範例。
- 修改 `check-rules.sh` 時，先在 `testdata/` 加 fixture 和 `expected.txt`，跑 `bash .claude/skills/nest-egg-money-review/scripts/check-rules.test.sh` 看到紅燈再改 script。
- script 只能用 bash 3.2（macOS 內建）與 BSD grep 的語法。變數後面接中文時寫成 `${var}`，否則 bash 3.2 會把中文字的 byte 當成變數名稱。
