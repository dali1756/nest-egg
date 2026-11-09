import { describe, expect, test } from 'vitest'
import { makeUsdOpening } from '../test/portfolio'
import { openingCutoffs, openingCutoffText } from './opening'

const usdOn = (opened_on: string) => makeUsdOpening({ opened_on })

describe('openingCutoffs', () => {
  test.each(['2026-11-09', '2026-11-01'])('includes a USD record on %s, on or before the opening date', (date) => {
    expect(openingCutoffs([usdOn('2026-11-09')], date, ['USD'])).toEqual([usdOn('2026-11-09')])
  })

  test('leaves out a record after the opening date', () => {
    expect(openingCutoffs([usdOn('2026-11-09')], '2026-11-10', ['USD'])).toEqual([])
  })

  test('only looks at the currencies of the record', () => {
    expect(openingCutoffs([usdOn('2026-11-09')], '2026-11-09', ['TWD'])).toEqual([])
  })

  test.each(['', '2026-11'])('waits for a whole date, not %j', (date) => {
    expect(openingCutoffs([usdOn('2026-11-09')], date, ['USD'])).toEqual([])
  })
})

test('openingCutoffText names the currency and its opening date', () => {
  expect(openingCutoffText(usdOn('2026-11-09'))).toBe('USD 期初餘額設在 2026-11-09，這天（含）以前的紀錄不會算進 USD 餘額')
})
