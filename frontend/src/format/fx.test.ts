import { describe, expect, test } from 'vitest'
import { makeFxTrade } from '../test/fxTrades'
import { directionLabel, receivedAndPaid, spreadText } from './fx'

describe('spreadText', () => {
  test.each([
    ['buy', '120.00', '比市場多付 120.00'],
    ['buy', '-50.00', '比市場少付 50.00'],
    ['sell', '75.00', '比市場少拿 75.00'],
    ['sell', '-1234.50', '比市場多拿 1,234.50'],
    ['buy', '0.00', '和市場相同'],
    ['sell', null, '查不到市場參考匯率'],
  ] as const)('%s with spread %s reads %s', (side, spread, text) => {
    expect(spreadText(side, spread)).toBe(text)
  })
})

describe('directionLabel', () => {
  test('buying shows TWD to the foreign currency', () => {
    expect(directionLabel(makeFxTrade({ side: 'buy', currency: 'USD' }))).toBe('TWD → USD')
  })

  test('selling shows the foreign currency to TWD', () => {
    expect(directionLabel(makeFxTrade({ side: 'sell', currency: 'USD' }))).toBe('USD → TWD')
  })
})

describe('receivedAndPaid', () => {
  test('buying receives the foreign amount and pays TWD', () => {
    const trade = makeFxTrade({ side: 'buy', currency: 'USD', foreign_amount: '1000.00', twd_amount: '32420.00' })
    expect(receivedAndPaid(trade)).toEqual({ received: 'USD 1,000.00', paid: 'TWD 32,420.00' })
  })

  test('selling receives TWD and pays the foreign amount', () => {
    const trade = makeFxTrade({ side: 'sell', currency: 'USD', foreign_amount: '500.00', twd_amount: '16050.00' })
    expect(receivedAndPaid(trade)).toEqual({ received: 'TWD 16,050.00', paid: 'USD 500.00' })
  })
})
