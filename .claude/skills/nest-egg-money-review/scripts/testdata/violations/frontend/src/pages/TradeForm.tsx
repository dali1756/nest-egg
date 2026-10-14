import { z } from 'zod'

const schema = z.object({
  buy_amount: z.coerce.number(),
  fee: z.number(),
})

export function total(trade: { buy_amount: string; fee: string }) {
  return parseFloat(trade.buy_amount) + Number(trade.fee)
}

export function show(rate: number) {
  return rate.toFixed(2)
}

export function isNumber(x: unknown) {
  return typeof x === 'number'
}
