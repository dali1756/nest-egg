import { request, type DecimalString } from './http'

export type Side = 'buy' | 'sell'

export type FxRates = {
  trade_rate: DecimalString
  market_rate: DecimalString | null
  market_rate_date: string | null
  spread: DecimalString | null
}

export type FxTrade = FxRates & {
  id: number
  traded_on: string
  side: Side
  currency: string
  foreign_amount: DecimalString
  twd_amount: DecimalString
  note: string
}

export type FxMonth = { month: string; trades: FxTrade[] }

export type FxTradeInput = {
  traded_on: string
  side: Side
  currency: string
  foreign_amount: string
  twd_amount: string
  note: string
}

export function getFxMonth(month: string): Promise<FxMonth> {
  return request('GET', `/api/fx-trades?month=${encodeURIComponent(month)}`)
}

export function getFxTrade(id: number): Promise<FxTrade> {
  return request('GET', `/api/fx-trades/${id}`)
}

export function createFxTrade(input: FxTradeInput): Promise<FxTrade> {
  return request('POST', '/api/fx-trades', input)
}

export function updateFxTrade(id: number, input: FxTradeInput): Promise<FxTrade> {
  return request('PUT', `/api/fx-trades/${id}`, input)
}

export function deleteFxTrade(id: number): Promise<void> {
  return request('DELETE', `/api/fx-trades/${id}`)
}

export function previewFxTrade(input: Omit<FxTradeInput, 'note'>, id?: number): Promise<FxRates> {
  const params = new URLSearchParams({
    side: input.side,
    currency: input.currency,
    traded_on: input.traded_on,
    foreign_amount: input.foreign_amount,
    twd_amount: input.twd_amount,
  })
  if (id !== undefined) params.set('id', String(id))
  return request('GET', `/api/fx-trades/preview?${params}`)
}
