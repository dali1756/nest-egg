import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { ApiError } from '../api/http'
import {
  getOpeningBalance,
  getOpeningBalances,
  getPortfolioSummary,
  previewOpeningBalance,
  type OpeningBalanceInput,
  type OpeningCost,
} from '../api/portfolio'
import { useDebouncedValue } from './useDebouncedValue'

export const portfolioKey = ['portfolio'] as const

const openingBalancesKey = [...portfolioKey, 'opening-balances'] as const

export function usePortfolioSummary() {
  return useQuery({ queryKey: [...portfolioKey, 'summary'], queryFn: getPortfolioSummary })
}

export function useOpeningBalances() {
  return useQuery({ queryKey: openingBalancesKey, queryFn: getOpeningBalances })
}

export function useOpeningBalance(currency: string) {
  const query = useQuery({ queryKey: [...openingBalancesKey, currency], queryFn: () => getOpeningBalance(currency) })
  const isNotSet = query.error instanceof ApiError && query.error.status === 404
  return { ...query, isNotSet }
}

const previewDelayMs = 300

export type OpeningPreview = { cost?: OpeningCost; needsTWDCost: boolean }

export function useOpeningBalancePreview(currency: string, input: OpeningBalanceInput | undefined): OpeningPreview {
  const key = input ? JSON.stringify(input) : undefined
  const settledKey = useDebouncedValue(key, previewDelayMs)
  const query = useQuery({
    queryKey: [...portfolioKey, 'opening-preview', currency, settledKey],
    queryFn: () => previewOpeningBalance(currency, JSON.parse(settledKey!) as OpeningBalanceInput),
    enabled: settledKey !== undefined,
    placeholderData: keepPreviousData,
  })
  if (key === undefined) return { needsTWDCost: false }
  const needsTWDCost = query.error instanceof ApiError && Boolean(query.error.fields?.twd_cost)
  return { cost: needsTWDCost ? undefined : query.data, needsTWDCost }
}
