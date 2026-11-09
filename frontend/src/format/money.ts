import type { DecimalString } from '../api/http'

export function decimalPlaces(value: string): number {
  const dot = value.indexOf('.')
  return dot < 0 ? 0 : value.length - dot - 1
}

export function formatAmount(value: DecimalString): string {
  return formatAmountWithPlaces(value, decimalPlaces(value))
}

export function formatAmountWithPlaces(value: string, places: number): string {
  return new Intl.NumberFormat('zh-TW', { minimumFractionDigits: places, maximumFractionDigits: places }).format(value as DecimalString)
}

export type PnlDirection = 'gain' | 'loss' | 'flat'

export function pnlDirection(value: DecimalString): PnlDirection {
  if (value.startsWith('-')) return 'loss'
  return /[1-9]/.test(value) ? 'gain' : 'flat'
}

const signs: Record<PnlDirection, string> = { gain: '+', loss: '−', flat: '' }

export function formatSignedAmount(value: DecimalString): string {
  const unsigned = (value.startsWith('-') ? value.slice(1) : value) as DecimalString
  return signs[pnlDirection(value)] + formatAmount(unsigned)
}

export function formatSignedPercent(value: DecimalString): string {
  return `${formatSignedAmount(value)}%`
}
