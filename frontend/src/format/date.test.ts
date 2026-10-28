import { afterAll, beforeAll, describe, expect, test, vi } from 'vitest'
import {
  dayLabel,
  fromCalendarDay,
  groupByDay,
  isDate,
  isMonth,
  monthLabel,
  monthOrCurrent,
  pickableDate,
  shiftMonth,
  taipeiMonth,
  taipeiToday,
  toCalendarDay,
} from './date'

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

describe('monthOrCurrent', () => {
  const now = new Date('2026-10-28T02:00:00Z')
  test('keeps a valid month', () => expect(monthOrCurrent('2026-09', now)).toBe('2026-09'))
  test.each(['2026-13', '', null])('falls back to the Taipei month for %j', (m) => expect(monthOrCurrent(m, now)).toBe('2026-10'))
})

describe('groupByDay', () => {
  test('groups items by day in the order they come', () => {
    const items = [
      { day: '2026-10-27', id: 1 },
      { day: '2026-10-27', id: 2 },
      { day: '2026-10-15', id: 3 },
    ]
    const groups = groupByDay(items, (item) => item.day).map(([day, group]) => [day, group.map((item) => item.id)])
    expect(groups).toEqual([
      ['2026-10-27', [1, 2]],
      ['2026-10-15', [3]],
    ])
  })
})

describe('isDate', () => {
  test.each(['2026-10-26', '2024-02-29'])('accepts %s', (d) => expect(isDate(d)).toBe(true))
  test.each(['2026-02-30', '2026-13-01', '2026-10-1', '20261026', 'abc', ''])('rejects %s', (d) => expect(isDate(d)).toBe(false))
})

describe('pickableDate', () => {
  test.each(['2026-10-26', '2026-10-27'])('keeps %s on or before today', (d) => expect(pickableDate(d, '2026-10-27')).toBe(d))
  test.each(['2026-10-28', '2026-02-30', null])('drops %s', (d) => expect(pickableDate(d, '2026-10-27')).toBeUndefined())
})

function useBrowserTimeZone(timeZone: string) {
  beforeAll(() => {
    vi.stubEnv('TZ', timeZone)
  })
  afterAll(() => {
    vi.unstubAllEnvs()
  })
}

describe.each(['Asia/Taipei', 'America/Los_Angeles', 'Pacific/Kiritimati'])('calendar days in %s', (timeZone) => {
  useBrowserTimeZone(timeZone)

  test('toCalendarDay is that day in the browser time zone', () => {
    const day = toCalendarDay('2026-10-26')
    expect([day.getFullYear(), day.getMonth() + 1, day.getDate(), day.getDay()]).toEqual([2026, 10, 26, 1])
  })

  test('fromCalendarDay reads the day in the browser time zone', () => {
    expect(fromCalendarDay(new Date(2026, 9, 26))).toBe('2026-10-26')
  })

  test.each(['2024-02-29', '2026-01-01', '2026-12-31'])('%s survives the round trip', (date) =>
    expect(fromCalendarDay(toCalendarDay(date))).toBe(date),
  )
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
