import type { OpeningBalance } from '../api/portfolio'
import { isDate } from './date'

export function openingCutoffs(openings: OpeningBalance[], date: string, currencies: string[]): OpeningBalance[] {
  if (!isDate(date)) return []
  return openings.filter((o) => currencies.includes(o.currency) && date <= o.opened_on)
}

export function openingCutoffText(opening: OpeningBalance): string {
  return `${opening.currency} 期初餘額設在 ${opening.opened_on}，這天（含）以前的紀錄不會算進 ${opening.currency} 餘額`
}
