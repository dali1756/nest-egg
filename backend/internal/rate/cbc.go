package rate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"nest-egg/backend/internal/platform/taipei"
)

// 政府資料開放授權：畫面顯示匯率時要註明「資料來源：中央銀行」
const CBCURL = "https://cpx.cbc.gov.tw/api/OpenData/FTDOpenData_Day"

const maxCBCBody = 5 << 20

type CBCProvider struct {
	url    string
	client *http.Client
}

func NewCBCProvider(url string) *CBCProvider {
	return &CBCProvider{url: url, client: &http.Client{Timeout: 30 * time.Second}}
}

func (p *CBCProvider) Name() string { return "cbc" }

type cbcRow struct {
	Date string `json:"日期"`
	Rate string `json:"NTD_USD"`
}

func (p *CBCProvider) Fetch(ctx context.Context) ([]Rate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return nil, fmt.Errorf("build cbc request: %w", err)
	}
	req.Header.Set("User-Agent", "nest-egg")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get cbc rates: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get cbc rates: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCBCBody+1))
	if err != nil {
		return nil, fmt.Errorf("read cbc rates: %w", err)
	}
	if len(body) > maxCBCBody {
		return nil, errors.New("read cbc rates: body larger than 5 MB")
	}
	var rows []cbcRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("decode cbc rates: %w", err)
	}

	rates := make([]Rate, 0, len(rows))
	for _, row := range rows {
		date, err := time.ParseInLocation("20060102", row.Date, taipei.Location)
		if err != nil {
			return nil, fmt.Errorf("parse cbc date %q: %w", row.Date, err)
		}
		value, err := decimal.NewFromString(row.Rate)
		if err != nil {
			return nil, fmt.Errorf("parse cbc rate %q on %s: %w", row.Rate, row.Date, err)
		}
		rates = append(rates, Rate{Base: "USD", Quote: "TWD", Date: date, Value: value})
	}
	return rates, nil
}
