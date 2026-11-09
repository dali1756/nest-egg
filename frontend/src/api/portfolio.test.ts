import { describe, expect, test } from 'vitest'
import { stubFetch } from '../test/stubFetch'
import { ApiError } from './http'
import {
  deleteOpeningBalance,
  getOpeningBalance,
  getOpeningBalances,
  getPortfolioSummary,
  previewOpeningBalance,
  putOpeningBalance,
  type OpeningBalance,
  type PortfolioSummary,
} from './portfolio'

const jsonHeaders = { 'Content-Type': 'application/json' }

const usdOpening: OpeningBalance = {
  currency: 'USD',
  opened_on: '2026-10-01',
  amount: '10000.00',
  twd_cost: null,
  market_rate: '32',
  market_rate_date: '2026-10-01',
  cost: '320000.00',
  average_cost: '32.0000',
}

const input = { opened_on: '2026-10-01', amount: '10000', twd_cost: '' }

describe('reading', () => {
  test('getPortfolioSummary asks for the summary', async () => {
    const summary: PortfolioSummary = { cash: [] }
    const fetchMock = stubFetch(200, summary)
    await expect(getPortfolioSummary()).resolves.toEqual(summary)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/portfolio/summary')
  })

  test('getOpeningBalances unwraps the list', async () => {
    const fetchMock = stubFetch(200, { opening_balances: [usdOpening] })
    await expect(getOpeningBalances()).resolves.toEqual([usdOpening])
    expect(fetchMock.mock.calls[0][0]).toBe('/api/opening-balances')
  })

  test('getOpeningBalance asks for one currency', async () => {
    const fetchMock = stubFetch(200, usdOpening)
    await expect(getOpeningBalance('USD')).resolves.toEqual(usdOpening)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/opening-balances/USD')
  })

  test('an opening balance that is not set rejects with 404', async () => {
    stubFetch(404, { error: 'not found' })
    await expect(getOpeningBalance('USD')).rejects.toMatchObject({ status: 404 })
  })
})

describe('writing', () => {
  test('putOpeningBalance puts the whole opening balance', async () => {
    const fetchMock = stubFetch(201, usdOpening)
    await expect(putOpeningBalance('USD', input)).resolves.toEqual(usdOpening)
    expect(fetchMock).toHaveBeenCalledWith('/api/opening-balances/USD', { method: 'PUT', headers: jsonHeaders, body: JSON.stringify(input) })
  })

  test('putOpeningBalance carries the invalid fields', async () => {
    stubFetch(422, { error: 'validation failed', fields: { twd_cost: 'required when there is no market rate for this date' } })
    const err = await putOpeningBalance('USD', input).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).fields).toEqual({ twd_cost: 'required when there is no market rate for this date' })
  })

  test('deleteOpeningBalance deletes one currency', async () => {
    const fetchMock = stubFetch(204)
    await expect(deleteOpeningBalance('USD')).resolves.toBeUndefined()
    expect(fetchMock.mock.calls[0]).toEqual(['/api/opening-balances/USD', { method: 'DELETE', headers: jsonHeaders, body: undefined }])
  })
})

describe('previewOpeningBalance', () => {
  const cost = { market_rate: '32', market_rate_date: '2026-10-01', cost: '320000.00', average_cost: '32.0000' }

  function previewQuery(fetchMock: ReturnType<typeof stubFetch>) {
    const url = new URL(String(fetchMock.mock.calls[0][0]), 'http://localhost')
    expect(url.pathname).toBe('/api/opening-balances/preview')
    return Object.fromEntries(url.searchParams)
  }

  test('sends the fields as query parameters', async () => {
    const fetchMock = stubFetch(200, cost)
    await expect(previewOpeningBalance('USD', { ...input, twd_cost: '315000' })).resolves.toEqual(cost)
    expect(previewQuery(fetchMock)).toEqual({ currency: 'USD', opened_on: '2026-10-01', amount: '10000', twd_cost: '315000' })
  })

  test('leaves out an empty TWD cost', async () => {
    const fetchMock = stubFetch(200, cost)
    await previewOpeningBalance('USD', input)
    expect(previewQuery(fetchMock)).toEqual({ currency: 'USD', opened_on: '2026-10-01', amount: '10000' })
  })
})
