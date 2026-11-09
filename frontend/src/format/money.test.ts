import { expect, test } from 'vitest'
import { formatAmount, formatAmountWithPlaces, formatSignedAmount, formatSignedPercent, pnlDirection } from './money'

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

test.each([
  ['2880.00', 'gain', '+2,880.00', '+2,880.00%'],
  ['-1520.00', 'loss', '−1,520.00', '−1,520.00%'],
  ['0.00', 'flat', '0.00', '0.00%'],
  ['0.82', 'gain', '+0.82', '+0.82%'],
  ['-0.43', 'loss', '−0.43', '−0.43%'],
  ['0.01', 'gain', '+0.01', '+0.01%'],
] as const)('%s is a %s, shown as %s and %s', (value, direction, amount, percent) => {
  expect(pnlDirection(value)).toBe(direction)
  expect(formatSignedAmount(value)).toBe(amount)
  expect(formatSignedPercent(value)).toBe(percent)
})

test.each([
  ['10000', 2, '10,000.00'],
  ['309.5', 2, '309.50'],
  ['3000', 0, '3,000'],
] as const)('pads %s to %i decimals as %s', (value, places, want) => {
  expect(formatAmountWithPlaces(value, places)).toBe(want)
})
