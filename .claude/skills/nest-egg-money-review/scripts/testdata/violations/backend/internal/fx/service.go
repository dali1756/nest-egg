package fx

import (
	"strconv"

	"github.com/shopspring/decimal"
)

var total float64

func parse(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func rate() decimal.Decimal {
	return decimal.NewFromFloat(32.42)
}

func toFloat(d decimal.Decimal) float32 {
	return float32(d.InexactFloat64())
}

func init() {
	decimal.MarshalJSONWithoutQuotes = true
}

func exact(d decimal.Decimal) bool {
	_, ok := d.Float64()
	return ok
}

// 例外註解寫的是別條規則，float-go 仍要抓到
var wrongAllow float64 // money-rules:allow json-number
