import { request, type DecimalString } from './http'

export type UnavailableReason = 'negative_balance' | 'missing_income_rate' | 'missing_latest_rate'

export type CashBalance = {
  currency: string
  name: string
  balance: DecimalString
  twd_value: DecimalString | null
  average_cost: DecimalString | null
  cost: DecimalString | null
  market_rate: DecimalString | null
  market_rate_date: string | null
  unrealized_pnl: DecimalString | null
  unrealized_pnl_percent: DecimalString | null
  unavailable_reason: UnavailableReason | null
  missing_rate_entry: { id: number; occurred_on: string } | null
}

export type PortfolioSummary = { cash: CashBalance[] }

export type OpeningCost = {
  market_rate: DecimalString | null
  market_rate_date: string | null
  cost: DecimalString | null
  average_cost: DecimalString | null
}

export type OpeningBalance = OpeningCost & {
  currency: string
  opened_on: string
  amount: DecimalString
  twd_cost: DecimalString | null
}

/** twd_cost 空字串表示沒填，由後端用市場參考匯率換算 */
export type OpeningBalanceInput = {
  opened_on: string
  amount: string
  twd_cost: string
}

function openingBalancePath(currency: string): string {
  return `/api/opening-balances/${encodeURIComponent(currency)}`
}

export function getPortfolioSummary(): Promise<PortfolioSummary> {
  return request('GET', '/api/portfolio/summary')
}

export async function getOpeningBalances(): Promise<OpeningBalance[]> {
  const res = await request<{ opening_balances: OpeningBalance[] }>('GET', '/api/opening-balances')
  return res.opening_balances
}

export function getOpeningBalance(currency: string): Promise<OpeningBalance> {
  return request('GET', openingBalancePath(currency))
}

export function putOpeningBalance(currency: string, input: OpeningBalanceInput): Promise<OpeningBalance> {
  return request('PUT', openingBalancePath(currency), input)
}

export function deleteOpeningBalance(currency: string): Promise<void> {
  return request('DELETE', openingBalancePath(currency))
}

export function previewOpeningBalance(currency: string, input: OpeningBalanceInput): Promise<OpeningCost> {
  const params = new URLSearchParams({ currency, opened_on: input.opened_on, amount: input.amount })
  if (input.twd_cost !== '') params.set('twd_cost', input.twd_cost)
  return request('GET', `/api/opening-balances/preview?${params}`)
}
