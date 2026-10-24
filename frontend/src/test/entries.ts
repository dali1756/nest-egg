import type { Entry } from '../api/ledger'

export function makeEntry(overrides: Partial<Entry> = {}): Entry {
  return {
    id: 1,
    occurred_on: '2026-10-20',
    kind: 'expense',
    category: { id: 1, name: '飲食' },
    currency: 'TWD',
    amount: '120.00',
    market_rate: null,
    market_rate_date: null,
    note: '午餐',
    ...overrides,
  }
}
