import { z } from 'zod'
import { decimalPlaces } from '../format/money'

const amountPattern = /^[0-9]{1,16}(\.[0-9]+)?$/

export function amountField() {
  return z
    .string()
    .min(1, '請輸入金額')
    .regex(amountPattern, '請輸入數字，例如 120 或 32.5')
    .refine((amount) => /[1-9]/.test(amount), '請輸入大於 0 的金額')
}

export function optionalAmountField() {
  return z.string().superRefine((amount, ctx) => {
    if (amount === '') return
    const problem = amountField().safeParse(amount).error?.issues[0].message
    if (problem) ctx.addIssue({ code: 'custom', message: problem })
  })
}

export function decimalPlacesProblem(amount: string, minorUnit: number | undefined): string | undefined {
  if (minorUnit === undefined || decimalPlaces(amount) <= minorUnit) return undefined
  return `金額最多 ${minorUnit} 位小數`
}
