import { TWD } from '../api/currencies'
import type { FxTrade, Side } from '../api/fx'
import type { DecimalString } from '../api/http'
import { formatAmount } from './money'

const spreadWording: Record<Side, { worse: string; better: string }> = {
  buy: { worse: '比市場多付', better: '比市場少付' },
  sell: { worse: '比市場少拿', better: '比市場多拿' },
}

export function spreadText(side: Side, spread: DecimalString | null): string {
  if (spread === null) return '查不到市場參考匯率'
  if (!/[1-9]/.test(spread)) return '和市場相同'
  const better = spread.startsWith('-')
  const amount = formatAmount((better ? spread.slice(1) : spread) as DecimalString)
  return `${better ? spreadWording[side].better : spreadWording[side].worse} ${amount}`
}

export function directionLabel({ side, currency }: Pick<FxTrade, 'side' | 'currency'>): string {
  return side === 'buy' ? `${TWD} → ${currency}` : `${currency} → ${TWD}`
}

export function receivedAndPaid(trade: FxTrade): { received: string; paid: string } {
  const foreign = `${trade.currency} ${formatAmount(trade.foreign_amount)}`
  const twd = `${TWD} ${formatAmount(trade.twd_amount)}`
  return trade.side === 'buy' ? { received: foreign, paid: twd } : { received: twd, paid: foreign }
}
