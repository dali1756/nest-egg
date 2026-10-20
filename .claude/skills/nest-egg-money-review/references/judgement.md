# 人工檢查清單

`check-rules.sh` 只看單行文字，下列問題要讀程式碼判斷。只檢查本次變更的檔案；每項都對照「怎麼看」逐一確認。

## 1. 算不出來的值留空，不用別的值頂替（規則 3）

- `market_rate` 查不到交易日（台北日期）的匯率時，是 `nil` / SQL `NULL` / JSON `null`。
- 不可用 `Latest()`、前一天的匯率、`0`、`decimal.Zero` 頂替。
- 同樣適用於其他算不出來的值：沒有交易的月份平均成本、沒有匯率時的市值與損益。回 `null` 或不回這一筆，不回 `"0"`。
- 怎麼看：找所有「查不到」「長度為 0」「除數為 0」的分支，看它回傳什麼。

## 2. 平均成本用總額相除（規則 7）

- 正確：`SUM(pay_amount + fee) / SUM(buy_amount)`，Go 端也一樣（先各自加總再相除）。
- 錯誤：把每筆 `trade_rate` 加總再除以筆數、漏掉 `fee`。
- `decimal.Div` 除以 0 會 panic，除數為 0 時依第 1 項處理。

## 3. 報價方向：1 外幣 = X 台幣（規則 6）

- 買外幣（`pay_currency = TWD`）：`trade_rate ≈ pay_amount / buy_amount`。
- 賣外幣（`buy_currency = TWD`）：`trade_rate ≈ buy_amount / pay_amount`。
- 匯差方向買賣相反：買外幣時成交匯率越高越吃虧，賣外幣時越低越吃虧。
- 怎麼看：賣外幣的路徑有沒有直接沿用買外幣的公式。

## 4. 小數位數

- 金額欄位 `NUMERIC(20,4)`、匯率 `NUMERIC(20,8)`。PostgreSQL 寫入超過位數的值會無聲四捨五入，不會報錯。
- API 收到超過位數的金額要回傳 `apperr.ValidationError`（422，標出是哪個欄位），不可默默截斷。
- 計算過程不先四捨五入，只在輸出顯示用的字串時才處理位數。

## 5. 日期以台北時間計算（規則 4）

- 從時間戳取日期、月份前先 `.In(loc)`。
- 查詢區間用 `time.ParseInLocation` 或 `time.Date(..., loc)` 組出台北的邊界：`[月初 00:00, 下月初 00:00)`。
- SQL 的分組與 `WHERE` 篩選兩邊都要 `AT TIME ZONE 'Asia/Taipei'`。

## 6. log 與錯誤回應

- log 只記 ID，不記金額、匯率、request body（例如 `"pay_amount", t.PayAmount` 就算是財務資料外洩）。
- 跨多行的 `slog` 呼叫 script 抓不到，要逐一看參數。
- 錯誤回應只給通用訊息與 `request_id`，內部錯誤只留在後端 log。

## 7. 前端不做金額運算

- 金額、匯率在前端一律是字串：直接顯示，或用 `Intl.NumberFormat` 格式化字串。
- 合計、平均、損益由後端用 `decimal` 算好，以字串回傳。
- 不在前端自己寫小數運算（包括用 `BigInt` 手刻），也不為此引入小數函式庫。

## 8. 外部 API 只在 RateProvider 裡呼叫（規則 8）

- 只有 `internal/rate`（之後的 `internal/stock`）的 provider 實作可以呼叫外部網址。
- 其他 package 透過 interface 取資料；測試用假的 provider，不打真的 API。

## 9. 幣別是資料（規則 5）

- 檢查幣別是否存在要查 `currencies` 表（外鍵錯誤轉成 `apperr.ValidationError`，回 422），不在程式碼裡比對清單。
- 唯一可以寫死的是本位幣 `TWD`（v1 每筆換匯有一邊必須是 TWD）。

## 10. 使用者只看得到自己的資料（規則 9）

- 使用者只從 `auth.CurrentUser(ctx)` 取得；handler、service 的參數裡的 `userID` 都要能追回這裡，不可來自路徑、query、body、header。
- 存取使用者資料表的每一條 sqlc 查詢都有 `user_id` 條件，包括列表、加總、每月統計這類彙總查詢；查單筆、修改、刪除用 `WHERE id = $1 AND user_id = $2`。
- 修改、刪除後檢查影響的列數：0 列要回 `apperr.ErrNotFound`（404），不可回成功。
- 別人的資料與不存在的資料回應完全相同（404），不回 403。
- 回應的 struct 沒有 `user_id`，也不會帶出其他使用者的資料（例如 JOIN 之後忘了篩選）。
- 新的使用者資料表有 `user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE`。
- 有 A/B 隔離測試：A 建立的資料，B 的列表看不到；B 用 A 的 id 查、改、刪都得到 404，而且 A 的資料不變。
- 怎麼看：從 handler 一路追到 SQL，確認 `user.ID` 有傳到每一條查詢；搜尋這次新增的 `.sql` 裡每個 `FROM`、`UPDATE`、`DELETE` 是否都有 `user_id`。
