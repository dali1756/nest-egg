import { describe, expect, test } from 'vitest'
import { amountField, decimalPlacesProblem } from './amount'

describe('amountField', () => {
  test.each(['120', '32.5', '0.01'])('accepts %s', (amount) => {
    expect(amountField().safeParse(amount).success).toBe(true)
  })

  test.each(['', 'abc', '1e3', '-1', '+5', '0', '0.00', '12345678901234567'])('rejects %j', (amount) => {
    expect(amountField().safeParse(amount).success).toBe(false)
  })

  test('asks for an amount when it is empty', () => {
    expect(amountField().safeParse('').error?.issues[0].message).toBe('請輸入金額')
  })
})

describe('decimalPlacesProblem', () => {
  test('allows up to the currency minor unit', () => {
    expect(decimalPlacesProblem('1.23', 2)).toBeUndefined()
    expect(decimalPlacesProblem('100', 0)).toBeUndefined()
  })

  test('reports more decimals than the currency has', () => {
    expect(decimalPlacesProblem('1.234', 2)).toBe('金額最多 2 位小數')
    expect(decimalPlacesProblem('100.5', 0)).toBe('金額最多 0 位小數')
  })

  test('skips the check until the currency is known', () => {
    expect(decimalPlacesProblem('1.234', undefined)).toBeUndefined()
  })
})
