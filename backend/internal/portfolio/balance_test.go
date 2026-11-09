package portfolio

import (
	"slices"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/rate"
)

var (
	twd = currency.Currency{Code: "TWD", Name: "新臺幣", MinorUnit: 2}
	usd = currency.Currency{Code: "USD", Name: "美元", MinorUnit: 2}
	// 還沒有這個幣別，用來確認排序與只列有紀錄的幣別
	jpy           = currency.Currency{Code: "JPY", Name: "日圓", MinorUnit: 0}
	allCurrencies = []currency.Currency{jpy, twd, usd}
)

// 用 id 當建立順序，同一個測試裡的 id 不重複
func createdAt(id int64) time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(id) * time.Second)
}

func income(id int64, date, cur, amount, marketRate string) Movement {
	m := Movement{Source: SourceLedger, ID: id, Date: taipeiDate(date), CreatedAt: createdAt(id), Kind: KindIncome, Currency: cur, Amount: dec(amount)}
	if marketRate != "" {
		m.MarketRate = decPtr(marketRate)
	}
	return m
}

func expense(id int64, date, cur, amount string) Movement {
	return Movement{Source: SourceLedger, ID: id, Date: taipeiDate(date), CreatedAt: createdAt(id), Kind: KindExpense, Currency: cur, Amount: dec(amount)}
}

func buyUSD(id int64, date, foreign, twdAmount string) Movement {
	return Movement{Source: SourceFx, ID: id, Date: taipeiDate(date), CreatedAt: createdAt(id), Kind: KindBuy, Currency: "USD", Amount: dec(foreign), TWDAmount: dec(twdAmount)}
}

func sellUSD(id int64, date, foreign, twdAmount string) Movement {
	return Movement{Source: SourceFx, ID: id, Date: taipeiDate(date), CreatedAt: createdAt(id), Kind: KindSell, Currency: "USD", Amount: dec(foreign), TWDAmount: dec(twdAmount)}
}

func twdOpening(date, amount string) Opening {
	return Opening{Currency: "TWD", MinorUnit: 2, OpenedOn: taipeiDate(date), Amount: dec(amount)}
}

func usdOpening(date, amount, twdCost string) Opening {
	return Opening{Currency: "USD", MinorUnit: 2, OpenedOn: taipeiDate(date), Amount: dec(amount), TWDCost: decPtr(twdCost)}
}

func summarize(openings []Opening, movements []Movement, latest *rate.MarketRate) []CashBalance {
	return Summarize(allCurrencies, openings, movements, map[string]*rate.MarketRate{"USD": latest})
}

func rowOf(t *testing.T, rows []CashBalance, code string) CashBalance {
	t.Helper()
	for _, r := range rows {
		if r.Currency.Code == code {
			return r
		}
	}
	t.Fatalf("no %s row in %+v", code, rows)
	return CashBalance{}
}

func fixed(d *decimal.Decimal, places int32) string {
	if d == nil {
		return "null"
	}
	return d.StringFixed(places)
}

func want(t *testing.T, field string, got *decimal.Decimal, places int32, expected string) {
	t.Helper()
	if s := fixed(got, places); s != expected {
		t.Errorf("%s = %s, want %s", field, s, expected)
	}
}

func TestSummarizeTWDBalance(t *testing.T) {
	rows := summarize(
		[]Opening{twdOpening("2026-10-01", "120000")},
		[]Movement{
			income(1, "2026-10-05", "TWD", "50000", ""),
			expense(2, "2026-10-06", "TWD", "12000"),
			buyUSD(3, "2026-10-07", "1000", "32420"),
		},
		marketRate("32.3", "2026-11-02"),
	)

	row := rowOf(t, rows, "TWD")
	want(t, "TWD balance", &row.Balance, 2, "125580.00")
	want(t, "TWD value", row.TWDValue, 2, "125580.00")
	want(t, "TWD cost", row.Cost, 2, "null")
	if row.MarketRate != nil || row.UnavailableReason != "" {
		t.Errorf("TWD market rate, reason = %+v %q; want none", row.MarketRate, row.UnavailableReason)
	}
}

func TestSummarizeUSDBalance(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{
			buyUSD(1, "2026-10-07", "1000", "32420"),
			sellUSD(2, "2026-10-08", "500", "16050"),
			expense(3, "2026-10-09", "USD", "200"),
		},
		marketRate("32.3", "2026-11-02"),
	)

	row := rowOf(t, rows, "USD")
	want(t, "USD balance", &row.Balance, 2, "10300.00")
	if row.Currency != usd {
		t.Errorf("currency = %+v, want %+v", row.Currency, usd)
	}
}

func TestSummarizeTWDGoesNegativeWithOnlyAnFxBuy(t *testing.T) {
	rows := summarize(nil, []Movement{buyUSD(1, "2026-10-07", "1000", "32420")}, marketRate("32.3", "2026-11-02"))

	row := rowOf(t, rows, "TWD")
	want(t, "TWD balance", &row.Balance, 2, "-32420.00")
	want(t, "TWD value", row.TWDValue, 2, "-32420.00")
}

func TestSummarizeIgnoresRecordsBeforeTheOpeningDate(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{expense(1, "2026-09-15", "USD", "300")},
		marketRate("32.3", "2026-11-02"),
	)

	wantBalance(t, rows, "USD", "10000.00")
}

func TestSummarizeStillCountsTheTWDSideOfAnFxTradeBeforeTheUSDOpening(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{buyUSD(1, "2026-09-15", "500", "16000")},
		marketRate("32.3", "2026-11-02"),
	)

	wantBalance(t, rows, "USD", "10000.00")
	wantBalance(t, rows, "TWD", "-16000.00")
}

func TestSummarizeIgnoresTheOpeningDayAndCountsTheNextDay(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{
			expense(1, "2026-10-01", "USD", "300"),
			expense(2, "2026-10-02", "USD", "200"),
		},
		marketRate("32.3", "2026-11-02"),
	)

	wantBalance(t, rows, "USD", "9800.00")
}

func wantBalance(t *testing.T, rows []CashBalance, code, expected string) {
	t.Helper()
	row := rowOf(t, rows, code)
	want(t, code+" balance", &row.Balance, int32(row.Currency.MinorUnit), expected)
}

func wantCost(t *testing.T, row CashBalance, cost, average string) {
	t.Helper()
	want(t, row.Currency.Code+" cost", row.Cost, 2, cost)
	want(t, row.Currency.Code+" average cost", row.AverageCost, 4, average)
}

var latestUSD = marketRate("32.3", "2026-11-02")

func TestSummarizeOpeningCostDefaultsToItsMarketRate(t *testing.T) {
	opening := Opening{Currency: "USD", MinorUnit: 2, OpenedOn: taipeiDate("2026-10-01"), Amount: dec("10000"), MarketRate: marketRate("32", "2026-10-01")}
	rows := summarize([]Opening{opening}, []Movement{buyUSD(1, "2026-10-07", "1000", "32420")}, latestUSD)

	wantCost(t, rowOf(t, rows, "USD"), "352420.00", "32.0382")
}

func TestSummarizeOpeningCostPrefersTheTWDCostTheUserEntered(t *testing.T) {
	opening := usdOpening("2026-10-01", "10000", "315000")
	opening.MarketRate = marketRate("32", "2026-10-01")
	rows := summarize([]Opening{opening}, nil, latestUSD)

	wantCost(t, rowOf(t, rows, "USD"), "315000.00", "31.5000")
}

func TestSummarizeSellingDeductsCostAtTheAverage(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{buyUSD(1, "2026-10-07", "1000", "32420"), sellUSD(2, "2026-10-08", "500", "16050")},
		latestUSD,
	)

	wantBalance(t, rows, "USD", "10500.00")
	wantCost(t, rowOf(t, rows, "USD"), "336400.91", "32.0382")
}

func TestSummarizeForeignIncomeCostsItsMarketRate(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{income(1, "2026-10-10", "USD", "100", "32.2")},
		latestUSD,
	)

	wantCost(t, rowOf(t, rows, "USD"), "323220.00", "32.0020")
}

func TestSummarizeForeignExpenseDeductsCostAtTheAverage(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{expense(1, "2026-10-10", "USD", "1000")},
		latestUSD,
	)

	wantCost(t, rowOf(t, rows, "USD"), "288000.00", "32.0000")
}

func TestSummarizeSameDayFollowsCreationOrder(t *testing.T) {
	opening := []Opening{usdOpening("2026-10-01", "1000", "32000")}
	for name, tc := range map[string]struct {
		sellID, buyID int64
		average       string
	}{
		"sell created first": {sellID: 1, buyID: 2, average: "33.0000"},
		"buy created first":  {sellID: 2, buyID: 1, average: "32.5000"},
	} {
		t.Run(name, func(t *testing.T) {
			// 刻意讓傳入的順序和建立順序相反
			movements := []Movement{buyUSD(tc.buyID, "2026-10-28", "1000", "33000"), sellUSD(tc.sellID, "2026-10-28", "1000", "32300")}
			if tc.buyID < tc.sellID {
				movements[0], movements[1] = movements[1], movements[0]
			}

			want(t, "USD average cost", rowOf(t, summarize(opening, movements, latestUSD), "USD").AverageCost, 4, tc.average)
		})
	}
}

func TestSummarizeOrdersByDateBeforeCreationOrder(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "1000", "32000")},
		[]Movement{buyUSD(1, "2026-10-28", "1000", "33000"), sellUSD(2, "2026-10-05", "1000", "32300")},
		latestUSD,
	)

	want(t, "USD average cost", rowOf(t, rows, "USD").AverageCost, 4, "33.0000")
}

func TestSummarizeBuyingAfterOverselling(t *testing.T) {
	rows := summarize(nil, []Movement{sellUSD(1, "2026-10-05", "500", "16050"), buyUSD(2, "2026-10-06", "1000", "32000")}, latestUSD)

	wantBalance(t, rows, "USD", "500.00")
	wantCost(t, rowOf(t, rows, "USD"), "16000.00", "32.0000")
}

func TestSummarizeSameDayAcrossLedgerAndFxFollowsCreationTime(t *testing.T) {
	// 收支與換匯的 id 各自編號，同一天只能靠建立時間排先後
	spend := expense(9, "2026-10-28", "USD", "1000")
	spend.CreatedAt = createdAt(1)
	buy := buyUSD(1, "2026-10-28", "1000", "33000")
	buy.CreatedAt = createdAt(2)

	rows := summarize([]Opening{usdOpening("2026-10-01", "1000", "32000")}, []Movement{buy, spend}, latestUSD)

	want(t, "USD average cost", rowOf(t, rows, "USD").AverageCost, 4, "33.0000")
}

func wantValueAndPnL(t *testing.T, row CashBalance, value, pnl, percent string) {
	t.Helper()
	want(t, row.Currency.Code+" TWD value", row.TWDValue, 2, value)
	want(t, row.Currency.Code+" unrealized P&L", row.UnrealizedPnL, 2, pnl)
	want(t, row.Currency.Code+" unrealized P&L %", row.UnrealizedPnLPercent, 2, percent)
}

func holding11000USD() ([]Opening, []Movement) {
	return []Opening{usdOpening("2026-10-01", "10000", "320000")}, []Movement{buyUSD(1, "2026-10-07", "1000", "32420")}
}

func TestSummarizeUnrealizedGain(t *testing.T) {
	openings, movements := holding11000USD()
	latest := marketRate("32.3", "2026-11-02")

	row := rowOf(t, summarize(openings, movements, latest), "USD")

	wantValueAndPnL(t, row, "355300.00", "2880.00", "0.82")
	if !sameMarketRate(row.MarketRate, latest) {
		t.Errorf("market rate = %+v, want %+v", row.MarketRate, latest)
	}
}

func TestSummarizeUnrealizedLoss(t *testing.T) {
	openings, movements := holding11000USD()

	row := rowOf(t, summarize(openings, movements, marketRate("31.9", "2026-11-02")), "USD")

	wantValueAndPnL(t, row, "350900.00", "-1520.00", "-0.43")
}

func TestSummarizePnLIsTheRoundedValueMinusTheRoundedCost(t *testing.T) {
	// 市值 0.48 × 32.3 = 15.504、成本 15.60 × 0.48 ÷ 0.5 = 14.976；先相減再捨入會得到 0.53，和畫面上的 15.50、14.98 對不起來
	rows := summarize(nil, []Movement{buyUSD(1, "2026-10-07", "0.5", "15.60"), sellUSD(2, "2026-10-08", "0.02", "0.65")}, marketRate("32.3", "2026-11-02"))

	row := rowOf(t, rows, "USD")
	want(t, "USD cost", row.Cost, 2, "14.98")
	wantValueAndPnL(t, row, "15.50", "0.52", "3.47")
}

func TestSummarizeLeavesThePercentEmptyWhenTheRoundedCostIsZero(t *testing.T) {
	// 剩下的成本 0.01 × 0.4 = 0.004，捨入後是 0，百分比除不下去
	rows := summarize(nil, []Movement{buyUSD(1, "2026-10-07", "1", "0.01"), sellUSD(2, "2026-10-08", "0.6", "19")}, marketRate("32.3", "2026-11-02"))

	wantValueAndPnL(t, rowOf(t, rows, "USD"), "12.92", "12.92", "null")
}

func wantUnavailable(t *testing.T, row CashBalance, reason UnavailableReason) {
	t.Helper()
	if row.UnavailableReason != reason {
		t.Errorf("%s unavailable reason = %q, want %q", row.Currency.Code, row.UnavailableReason, reason)
	}
}

func wantMissingRateEntry(t *testing.T, row CashBalance, id int64, date string) {
	t.Helper()
	got := row.MissingRateEntry
	if got == nil || got.ID != id || !got.OccurredOn.Equal(taipeiDate(date)) {
		t.Errorf("missing rate entry = %+v, want id %d on %s", got, id, date)
	}
}

func TestSummarizeNegativeBalanceHasNoCostValueOrPnL(t *testing.T) {
	row := rowOf(t, summarize(nil, []Movement{sellUSD(1, "2026-10-05", "500", "16050")}, latestUSD), "USD")

	wantBalance(t, []CashBalance{row}, "USD", "-500.00")
	wantUnavailable(t, row, NegativeBalance)
	wantCost(t, row, "null", "null")
	wantValueAndPnL(t, row, "null", "null", "null")
}

func incomeWithoutRate(id int64, date, amount string) Movement {
	return income(id, date, "USD", amount, "")
}

func TestSummarizeIncomeWithoutMarketRateLeavesCostUnknown(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{incomeWithoutRate(42, "2026-10-10", "100")},
		latestUSD,
	)

	row := rowOf(t, rows, "USD")
	wantBalance(t, rows, "USD", "10100.00")
	wantUnavailable(t, row, MissingIncomeRate)
	wantMissingRateEntry(t, row, 42, "2026-10-10")
	wantCost(t, row, "null", "null")
	wantValueAndPnL(t, row, "326230.00", "null", "null")
}

func TestSummarizePointsAtTheEarliestIncomeWithoutMarketRate(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{incomeWithoutRate(43, "2026-10-12", "50"), incomeWithoutRate(42, "2026-10-10", "100")},
		latestUSD,
	)

	wantMissingRateEntry(t, rowOf(t, rows, "USD"), 42, "2026-10-10")
}

func TestSummarizeRecoversOnceTheUnknownCostIsSoldOff(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{
			incomeWithoutRate(42, "2026-10-10", "100"),
			sellUSD(1, "2026-10-20", "10100", "326000"),
			buyUSD(2, "2026-10-21", "1000", "32420"),
		},
		latestUSD,
	)

	row := rowOf(t, rows, "USD")
	wantUnavailable(t, row, "")
	if row.MissingRateEntry != nil {
		t.Errorf("missing rate entry = %+v, want none", row.MissingRateEntry)
	}
	want(t, "USD average cost", row.AverageCost, 4, "32.4200")
}

func TestSummarizeIncomeWithoutMarketRateAfterOversellingLeavesCostUnknown(t *testing.T) {
	rows := summarize(nil, []Movement{sellUSD(1, "2026-10-05", "500", "16050"), incomeWithoutRate(42, "2026-10-10", "1000")}, latestUSD)

	row := rowOf(t, rows, "USD")
	wantBalance(t, rows, "USD", "500.00")
	wantUnavailable(t, row, MissingIncomeRate)
	wantMissingRateEntry(t, row, 42, "2026-10-10")
}

func TestSummarizeWithoutALatestRateKeepsTheCost(t *testing.T) {
	openings, movements := holding11000USD()

	row := rowOf(t, summarize(openings, movements, nil), "USD")

	wantUnavailable(t, row, MissingLatestRate)
	wantCost(t, row, "352420.00", "32.0382")
	wantValueAndPnL(t, row, "null", "null", "null")
	if row.MarketRate != nil {
		t.Errorf("market rate = %+v, want none", row.MarketRate)
	}
}

func TestSummarizeReasonsFollowTheirPriority(t *testing.T) {
	negative := rowOf(t, summarize(nil, []Movement{sellUSD(1, "2026-10-05", "500", "16050")}, nil), "USD")
	wantUnavailable(t, negative, NegativeBalance)

	unknownCost := rowOf(t, summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{incomeWithoutRate(42, "2026-10-10", "100")},
		nil,
	), "USD")
	wantUnavailable(t, unknownCost, MissingIncomeRate)
	wantValueAndPnL(t, unknownCost, "null", "null", "null")
}

func TestSummarizeZeroBalanceIsWorthZero(t *testing.T) {
	movements := []Movement{buyUSD(1, "2026-10-07", "1000", "32420"), sellUSD(2, "2026-10-08", "1000", "32300")}
	for name, latest := range map[string]*rate.MarketRate{"with a latest rate": latestUSD, "without a latest rate": nil} {
		t.Run(name, func(t *testing.T) {
			row := rowOf(t, summarize(nil, movements, latest), "USD")

			wantBalance(t, []CashBalance{row}, "USD", "0.00")
			wantUnavailable(t, row, "")
			wantCost(t, row, "null", "null")
			wantValueAndPnL(t, row, "0.00", "null", "null")
		})
	}
}

func currencyCodes(rows []CashBalance) []string {
	codes := []string{}
	for _, r := range rows {
		codes = append(codes, r.Currency.Code)
	}
	return codes
}

func TestSummarizeListsOnlyCurrenciesWithRecords(t *testing.T) {
	rows := summarize(nil, []Movement{expense(1, "2026-10-05", "TWD", "120")}, latestUSD)

	if got := currencyCodes(rows); !slices.Equal(got, []string{"TWD"}) {
		t.Errorf("currencies = %v, want [TWD]", got)
	}
}

func TestSummarizeListsTWDFirstThenByCode(t *testing.T) {
	rows := summarize(
		[]Opening{usdOpening("2026-10-01", "10000", "320000")},
		[]Movement{expense(1, "2026-10-05", "JPY", "3000"), expense(2, "2026-10-06", "TWD", "120")},
		latestUSD,
	)

	if got := currencyCodes(rows); !slices.Equal(got, []string{"TWD", "JPY", "USD"}) {
		t.Errorf("currencies = %v, want [TWD JPY USD]", got)
	}
}

func TestSummarizeWithoutRecordsIsAnEmptyList(t *testing.T) {
	rows := summarize(nil, nil, latestUSD)

	if rows == nil || len(rows) != 0 {
		t.Errorf("rows = %#v, want an empty, non-nil list", rows)
	}
}
