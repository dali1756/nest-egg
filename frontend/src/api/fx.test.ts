import { describe, expect, test } from 'vitest'
import { stubFetch } from '../test/stubFetch'
import { createFxTrade, deleteFxTrade, getFxMonth, getFxTrade, previewFxTrade, updateFxTrade, type FxTrade } from './fx'

const jsonHeaders = { 'Content-Type': 'application/json' }

const buy: FxTrade = {
  id: 12,
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
}

const input = {
  traded_on: '2026-10-27',
  side: 'buy',
  currency: 'USD',
  foreign_amount: '1000',
  twd_amount: '32420',
  note: '10 月定期換匯',
} as const

describe('reading', () => {
  test('getFxMonth asks for the given month', async () => {
    const month = { month: '2026-10', trades: [buy] }
    const fetchMock = stubFetch(200, month)
    await expect(getFxMonth('2026-10')).resolves.toEqual(month)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/fx-trades?month=2026-10')
  })

  test('getFxTrade asks for one trade', async () => {
    const fetchMock = stubFetch(200, buy)
    await expect(getFxTrade(12)).resolves.toEqual(buy)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/fx-trades/12')
  })
})

describe('writing', () => {
  test('createFxTrade posts the trade', async () => {
    const fetchMock = stubFetch(201, buy)
    await expect(createFxTrade(input)).resolves.toEqual(buy)
    expect(fetchMock).toHaveBeenCalledWith('/api/fx-trades', { method: 'POST', headers: jsonHeaders, body: JSON.stringify(input) })
  })

  test('updateFxTrade puts the whole trade', async () => {
    const fetchMock = stubFetch(200, buy)
    await updateFxTrade(12, input)
    expect(fetchMock).toHaveBeenCalledWith('/api/fx-trades/12', { method: 'PUT', headers: jsonHeaders, body: JSON.stringify(input) })
  })

  test('deleteFxTrade deletes the trade', async () => {
    const fetchMock = stubFetch(204)
    await expect(deleteFxTrade(12)).resolves.toBeUndefined()
    expect(fetchMock.mock.calls[0]).toEqual(['/api/fx-trades/12', { method: 'DELETE', headers: jsonHeaders, body: undefined }])
  })
})

describe('previewFxTrade', () => {
  const rates = { trade_rate: '32.42', market_rate: '32.3', market_rate_date: '2026-10-27', spread: '120.00' }

  test('sends the fields as query parameters', async () => {
    const fetchMock = stubFetch(200, rates)
    await expect(previewFxTrade(input)).resolves.toEqual(rates)
    const url = new URL(String(fetchMock.mock.calls[0][0]), 'http://localhost')
    expect(url.pathname).toBe('/api/fx-trades/preview')
    expect(Object.fromEntries(url.searchParams)).toEqual({
      side: 'buy',
      currency: 'USD',
      traded_on: '2026-10-27',
      foreign_amount: '1000',
      twd_amount: '32420',
    })
  })

  test('adds the id when previewing a change', async () => {
    const fetchMock = stubFetch(200, rates)
    await previewFxTrade(input, 12)
    const url = new URL(String(fetchMock.mock.calls[0][0]), 'http://localhost')
    expect(url.searchParams.get('id')).toBe('12')
  })
})
