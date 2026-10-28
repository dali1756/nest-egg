import type { FxTrade } from '../api/fx'

export function makeFxTrade(overrides: Partial<FxTrade> = {}): FxTrade {
  return {
    id: 1,
    traded_on: '2026-10-27',
    side: 'buy',
    currency: 'USD',
    foreign_amount: '1000.00',
    twd_amount: '32420.00',
    trade_rate: '32.42',
    market_rate: '32.3',
    market_rate_date: '2026-10-27',
    spread: '120.00',
    note: '10 月定期換匯',
    ...overrides,
  }
}
