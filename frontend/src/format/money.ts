import type { DecimalString } from '../api/ledger'

export function formatAmount(value: DecimalString): string {
  const dot = value.indexOf('.')
  const decimals = dot < 0 ? 0 : value.length - dot - 1
  return new Intl.NumberFormat('zh-TW', { minimumFractionDigits: decimals, maximumFractionDigits: decimals }).format(value)
}
