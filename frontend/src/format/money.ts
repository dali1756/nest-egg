import type { DecimalString } from '../api/http'

export function decimalPlaces(value: string): number {
  const dot = value.indexOf('.')
  return dot < 0 ? 0 : value.length - dot - 1
}

export function formatAmount(value: DecimalString): string {
  const decimals = decimalPlaces(value)
  return new Intl.NumberFormat('zh-TW', { minimumFractionDigits: decimals, maximumFractionDigits: decimals }).format(value)
}
