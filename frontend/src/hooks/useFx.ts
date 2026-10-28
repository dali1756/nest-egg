import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getFxMonth, getFxTrade, previewFxTrade, type FxRates, type FxTradeInput } from '../api/fx'
import { useDebouncedValue } from './useDebouncedValue'

export const fxTradesKey = ['fx-trades'] as const

export function useFxMonth(month: string) {
  return useQuery({ queryKey: [...fxTradesKey, month], queryFn: () => getFxMonth(month) })
}

export function useFxTrade(id: number | undefined) {
  return useQuery({
    queryKey: [...fxTradesKey, 'trade', id],
    queryFn: () => getFxTrade(id!),
    enabled: id !== undefined,
  })
}

export type FxPreviewInput = Omit<FxTradeInput, 'note'>

const previewDelayMs = 300

export function useFxPreview(input: FxPreviewInput | undefined, id?: number): FxRates | undefined {
  const key = input ? JSON.stringify(input) : undefined
  const settledKey = useDebouncedValue(key, previewDelayMs)
  const query = useQuery({
    queryKey: ['fx-preview', settledKey, id],
    queryFn: () => previewFxTrade(JSON.parse(settledKey!) as FxPreviewInput, id),
    enabled: settledKey !== undefined,
    placeholderData: keepPreviousData,
  })
  return key === undefined ? undefined : query.data
}
