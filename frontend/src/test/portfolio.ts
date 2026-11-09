import type { CashBalance, OpeningBalance } from '../api/portfolio'
import type { fakeBackend } from './fakeBackend'

export const twdCash: CashBalance = {
  currency: 'TWD',
  name: '新臺幣',
  balance: '125580.00',
  twd_value: '125580.00',
  average_cost: null,
  cost: null,
  market_rate: null,
  market_rate_date: null,
  unrealized_pnl: null,
  unrealized_pnl_percent: null,
  unavailable_reason: null,
  missing_rate_entry: null,
}

export function makeUsdCash(overrides: Partial<CashBalance> = {}): CashBalance {
  return {
    currency: 'USD',
    name: '美元',
    balance: '11000.00',
    twd_value: '355300.00',
    average_cost: '32.0382',
    cost: '352420.00',
    market_rate: '32.3',
    market_rate_date: '2026-11-02',
    unrealized_pnl: '2880.00',
    unrealized_pnl_percent: '0.82',
    unavailable_reason: null,
    missing_rate_entry: null,
    ...overrides,
  }
}

export function makeUsdOpening(overrides: Partial<OpeningBalance> = {}): OpeningBalance {
  return {
    currency: 'USD',
    opened_on: '2026-10-01',
    amount: '10000.00',
    twd_cost: null,
    market_rate: '32',
    market_rate_date: '2026-10-01',
    cost: '320000.00',
    average_cost: '32.0000',
    ...overrides,
  }
}

export function summaryRequests(backend: ReturnType<typeof fakeBackend>): number {
  return backend.calls.filter((c) => c.method === 'GET' && c.path === '/api/portfolio/summary').length
}
