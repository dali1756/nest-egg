package portfolio

import (
	"cmp"
	"maps"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/currency"
	"nest-egg/backend/internal/rate"
)

type Source string

const (
	SourceLedger Source = "ledger"
	SourceFx     Source = "fx"
)

type Kind string

const (
	KindIncome  Kind = "income"
	KindExpense Kind = "expense"
	KindBuy     Kind = "buy"
	KindSell    Kind = "sell"
)

// Movement 是一筆收支或換匯，換匯的 Currency 是外幣、Amount 是外幣金額，TWDAmount 只有換匯有，MarketRate 只有外幣收支有。
type Movement struct {
	Source     Source
	ID         int64
	Date       time.Time
	CreatedAt  time.Time
	Kind       Kind
	Currency   string
	Amount     decimal.Decimal
	TWDAmount  decimal.Decimal
	MarketRate *decimal.Decimal
}

// UnavailableReason 說明均價、成本、市值、損益為什麼算不出來；都算得出來時是空字串。
type UnavailableReason string

const (
	NegativeBalance   UnavailableReason = "negative_balance"
	MissingIncomeRate UnavailableReason = "missing_income_rate"
	MissingLatestRate UnavailableReason = "missing_latest_rate"
)

// MissingRateEntry 是沒有市場參考匯率、成本不明的那筆外幣收入。
type MissingRateEntry struct {
	ID         int64
	OccurredOn time.Time
}

// CashBalance 是一個幣別的現金。金額都已捨入到要顯示的位數；算不出來的值是 nil。
type CashBalance struct {
	Currency             currency.Currency
	Balance              decimal.Decimal
	TWDValue             *decimal.Decimal
	AverageCost          *decimal.Decimal
	Cost                 *decimal.Decimal
	MarketRate           *rate.MarketRate
	UnrealizedPnL        *decimal.Decimal
	UnrealizedPnLPercent *decimal.Decimal
	UnavailableReason    UnavailableReason
	MissingRateEntry     *MissingRateEntry
	TWDMinorUnit         int
}

const (
	// 均價固定算到小數點後四位，如日幣這類匯率小於 1 的幣別也看得出差異
	averageCostPlaces = 4
	percentPlaces     = 2
)

var hundred = decimal.NewFromInt(100)

// flow 是某個幣別的一筆進出，quantity 一律是正數；cost 是流入的台幣成本，missing 有值表示這筆成本不明。
type flow struct {
	currency  string
	date      time.Time
	createdAt time.Time
	source    Source
	id        int64
	inflow    bool
	quantity  decimal.Decimal
	cost      decimal.Decimal
	missing   *MissingRateEntry
}

// Summarize 依期初餘額與收支、換匯算出各幣別的現金；latestRates 是各外幣最新的市場參考匯率，查不到時是 nil。
func Summarize(currencies []currency.Currency, openings []Opening, movements []Movement, latestRates map[string]*rate.MarketRate) []CashBalance {
	flows := flowsByCurrency(openings, movements)
	twdPlaces := minorUnitOf(currencies, currency.TWD)
	rows := []CashBalance{}
	for _, cur := range displayOrder(currencies) {
		if f, ok := flows[cur.Code]; ok {
			rows = append(rows, summarizeCurrency(cur, f, latestRates[cur.Code], twdPlaces))
		}
	}
	return rows
}

// currenciesIn 是期初餘額與紀錄碰到的幣別，依代碼排序。換匯的台幣邊不用查匯率，所以只看 Currency。
func currenciesIn(openings []Opening, movements []Movement) []string {
	codes := map[string]bool{}
	for _, o := range openings {
		codes[o.Currency] = true
	}
	for _, m := range movements {
		codes[m.Currency] = true
	}
	return slices.Sorted(maps.Keys(codes))
}

// displayOrder 是畫面上的順序：台幣第一，其他依幣別代碼。
func displayOrder(currencies []currency.Currency) []currency.Currency {
	ordered := slices.Clone(currencies)
	slices.SortFunc(ordered, func(a, b currency.Currency) int {
		return cmp.Or(cmp.Compare(displayRank(a), displayRank(b)), cmp.Compare(a.Code, b.Code))
	})
	return ordered
}

func displayRank(c currency.Currency) int {
	if c.Code == currency.TWD {
		return 0
	}
	return 1
}

func minorUnitOf(currencies []currency.Currency, code string) int32 {
	for _, c := range currencies {
		if c.Code == code {
			return int32(c.MinorUnit)
		}
	}
	return 0
}

// flowsByCurrency 依幣別整理出要計算的進出，期初餘額排第一，其餘依日期與建立順序（設計規則 4）。
func flowsByCurrency(openings []Opening, movements []Movement) map[string][]flow {
	openedOn := map[string]time.Time{}
	for _, o := range openings {
		openedOn[o.Currency] = o.OpenedOn
	}
	byCurrency := map[string][]flow{}
	for _, m := range movements {
		for _, f := range m.flows() {
			// 期初金額是那天結束時的餘額，已經包含那天與以前的紀錄
			if opened, ok := openedOn[f.currency]; ok && !f.date.After(opened) {
				continue
			}
			byCurrency[f.currency] = append(byCurrency[f.currency], f)
		}
	}
	for cur := range byCurrency {
		slices.SortFunc(byCurrency[cur], compareFlows)
	}
	// 留下來的紀錄都晚於期初日期，所以期初直接放在最前面
	for _, o := range openings {
		byCurrency[o.Currency] = append([]flow{o.flow()}, byCurrency[o.Currency]...)
	}
	return byCurrency
}

func compareFlows(a, b flow) int {
	return cmp.Or(
		a.date.Compare(b.date),
		a.createdAt.Compare(b.createdAt),
		cmp.Compare(a.source, b.source),
		cmp.Compare(a.id, b.id),
	)
}

func (o Opening) flow() flow {
	f := flow{currency: o.Currency, date: o.OpenedOn, inflow: true, quantity: o.Amount}
	if cost := o.Cost(); cost != nil {
		f.cost = *cost
	}
	return f
}

// flows 把一筆紀錄拆成各幣別的進出：換匯同時影響外幣與台幣。
func (m Movement) flows() []flow {
	switch m.Kind {
	case KindIncome:
		return []flow{m.incomeFlow()}
	case KindExpense:
		return []flow{m.flow(m.Currency, false, m.Amount)}
	case KindBuy:
		in := m.flow(m.Currency, true, m.Amount)
		in.cost = m.TWDAmount
		return []flow{in, m.flow(currency.TWD, false, m.TWDAmount)}
	case KindSell:
		return []flow{m.flow(m.Currency, false, m.Amount), m.flow(currency.TWD, true, m.TWDAmount)}
	}
	return nil
}

// incomeFlow：外幣收入的成本用當天的市場參考匯率換算；查不到匯率時成本不明，記下是哪一筆。台幣不算成本。
func (m Movement) incomeFlow() flow {
	in := m.flow(m.Currency, true, m.Amount)
	if m.Currency == currency.TWD {
		return in
	}
	if m.MarketRate == nil {
		in.missing = &MissingRateEntry{ID: m.ID, OccurredOn: m.Date}
		return in
	}
	in.cost = m.Amount.Mul(*m.MarketRate)
	return in
}

func (m Movement) flow(cur string, inflow bool, quantity decimal.Decimal) flow {
	return flow{currency: cur, date: m.Date, createdAt: m.CreatedAt, source: m.Source, id: m.ID, inflow: inflow, quantity: quantity}
}

// position 是逐筆計算到目前為止的數量與移動平均成本（設計規則 7）；missing 是目前持有的部分裡最早一筆成本不明的收入。
type position struct {
	quantity decimal.Decimal
	cost     decimal.Decimal
	missing  *MissingRateEntry
}

func (p *position) apply(f flow) {
	if f.inflow {
		p.receive(f)
		return
	}
	p.pay(f.quantity)
}

func (p *position) receive(f flow) {
	if p.quantity.IsPositive() {
		p.quantity = p.quantity.Add(f.quantity)
		p.cost = p.cost.Add(f.cost)
		if p.missing == nil {
			p.missing = f.missing
		}
		return
	}
	// 先前賣超或剛好歸零：這筆先補足缺口，剩下的數量依比例帶入這筆的成本，成本不明也從這筆重新算
	remaining := p.quantity.Add(f.quantity)
	*p = position{quantity: remaining}
	if remaining.IsPositive() {
		p.cost = f.cost.Mul(remaining).Div(f.quantity)
		p.missing = f.missing
	}
}

func (p *position) pay(quantity decimal.Decimal) {
	remaining := p.quantity.Sub(quantity)
	if !p.quantity.IsPositive() || !remaining.IsPositive() {
		*p = position{quantity: remaining}
		return
	}
	// 依當下均價扣成本，均價不變；乘完再除，不用先算出有循環小數的均價
	p.cost = p.cost.Mul(remaining).Div(p.quantity)
	p.quantity = remaining
}

func summarizeCurrency(cur currency.Currency, flows []flow, latest *rate.MarketRate, twdPlaces int32) CashBalance {
	var p position
	for _, f := range flows {
		p.apply(f)
	}
	row := CashBalance{Currency: cur, Balance: p.quantity.Round(int32(cur.MinorUnit)), TWDMinorUnit: int(twdPlaces)}
	if cur.Code == currency.TWD {
		row.TWDValue = &row.Balance
		return row
	}
	row.MarketRate = latest
	fillForeignValues(&row, p, latest, twdPlaces)
	return row
}

// fillForeignValues 填外幣的市值、成本與損益；算不出來的留 nil，並依優先順序寫下原因（設計規則 3、7）。
func fillForeignValues(row *CashBalance, p position, latest *rate.MarketRate, twdPlaces int32) {
	switch {
	case p.quantity.IsNegative():
		row.UnavailableReason = NegativeBalance
		return
	case p.quantity.IsZero():
		zero := decimal.Zero
		row.TWDValue = &zero
		return
	}
	if latest != nil {
		value := p.quantity.Mul(latest.Value).Round(twdPlaces)
		row.TWDValue = &value
	}
	if p.missing != nil {
		row.UnavailableReason = MissingIncomeRate
		row.MissingRateEntry = p.missing
		return
	}
	cost := p.cost.Round(twdPlaces)
	average := p.cost.Div(p.quantity).Round(averageCostPlaces)
	row.Cost, row.AverageCost = &cost, &average
	if latest == nil {
		row.UnavailableReason = MissingLatestRate
		return
	}
	row.UnrealizedPnL, row.UnrealizedPnLPercent = unrealizedPnL(*row.TWDValue, cost)
}

// unrealizedPnL 用捨入後的市值與成本相減，畫面上的三個數字才對得起來。
func unrealizedPnL(value, cost decimal.Decimal) (*decimal.Decimal, *decimal.Decimal) {
	pnl := value.Sub(cost)
	if cost.IsZero() {
		return &pnl, nil
	}
	percent := pnl.Mul(hundred).Div(cost).Round(percentPlaces)
	return &pnl, &percent
}
