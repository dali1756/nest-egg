import { describe, expect, test } from 'vitest'
import { dayLabel, isMonth, monthLabel, shiftMonth, taipeiMonth, taipeiToday } from './date'

describe('taipeiToday', () => {
  test('uses the Taipei date, not the UTC date', () => {
    expect(taipeiToday(new Date('2026-10-23T16:30:00Z'))).toBe('2026-10-24')
    expect(taipeiToday(new Date('2026-10-23T15:59:59Z'))).toBe('2026-10-23')
  })
})

describe('taipeiMonth', () => {
  test('uses the Taipei month', () => {
    expect(taipeiMonth(new Date('2026-10-31T16:30:00Z'))).toBe('2026-11')
  })
})

describe('shiftMonth', () => {
  test.each([
    ['2026-10', -1, '2026-09'],
    ['2026-01', -1, '2025-12'],
    ['2026-12', 1, '2027-01'],
    ['2026-10', 1, '2026-11'],
  ])('%s %i → %s', (month, delta, want) => {
    expect(shiftMonth(month, delta)).toBe(want)
  })
})

describe('isMonth', () => {
  test.each(['2026-10', '2026-01', '2026-12'])('accepts %s', (m) => expect(isMonth(m)).toBe(true))
  test.each(['2026-13', '2026-00', '202610', '2026-1', 'abc', ''])('rejects %s', (m) => expect(isMonth(m)).toBe(false))
})

describe('labels', () => {
  test('monthLabel', () => {
    expect(monthLabel('2026-10')).toBe('2026 年 10 月')
    expect(monthLabel('2026-09')).toBe('2026 年 9 月')
  })

  test('dayLabel shows month/day and the weekday', () => {
    expect(dayLabel('2026-10-23')).toBe('10/23（五）')
    expect(dayLabel('2026-11-01')).toBe('11/1（日）')
    expect(dayLabel('2024-02-29')).toBe('2/29（四）')
  })
})
