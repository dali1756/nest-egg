import { expect, test } from 'vitest'
import { formatAmount } from './money'

test.each([
  ['50000.00', '50,000.00'],
  ['120.00', '120.00'],
  ['0.00', '0.00'],
  ['-80.00', '-80.00'],
  ['32.508', '32.508'],
  ['1234567890123456.78', '1,234,567,890,123,456.78'],
] as const)('formats %s as %s without changing its value', (value, want) => {
  expect(formatAmount(value)).toBe(want)
})
