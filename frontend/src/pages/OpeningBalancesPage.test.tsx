import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import type { OpeningBalance } from '../api/portfolio'
import { fakeBackend } from '../test/fakeBackend'
import { makeUsdOpening } from '../test/portfolio'
import { renderApp } from '../test/renderApp'

function showOpenings(openings: OpeningBalance[]) {
  fakeBackend({ user: 'me@example.com', portfolio: { openings } })
  return renderApp('/assets/opening-balances')
}

async function rowOf(label: string) {
  return (await screen.findByText(label)).closest('a')!
}

const twdOpening: OpeningBalance = {
  currency: 'TWD',
  opened_on: '2026-10-01',
  amount: '120000.00',
  twd_cost: null,
  market_rate: null,
  market_rate_date: null,
  cost: null,
  average_cost: null,
}

test('explains what an opening balance is', async () => {
  showOpenings([])

  expect(await screen.findByRole('heading', { level: 1, name: '期初餘額' })).toBeInTheDocument()
  expect(screen.getByText('開始用 nest-egg 前手上已經有的錢，每個幣別一筆。')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '‹ 資產' })).toHaveAttribute('href', '/assets')
})

test('lists every currency, set or not', async () => {
  showOpenings([makeUsdOpening()])

  const twd = await rowOf('TWD 新臺幣')
  expect(within(twd).getByText('未設定')).toBeInTheDocument()
  expect(within(twd).getByText('設定 ›')).toBeInTheDocument()
  const usd = await rowOf('USD 美元')
  expect(within(usd).getByText('2026-10-01')).toBeInTheDocument()
  expect(within(usd).getByText('10,000.00')).toBeInTheDocument()
  expect(within(usd).getByText('成本 320,000.00（均價 32.0000）')).toBeInTheDocument()
})

test('shows a TWD opening balance without a cost', async () => {
  showOpenings([twdOpening])

  const twd = await rowOf('TWD 新臺幣')
  expect(within(twd).getByText('120,000.00')).toBeInTheDocument()
  expect(within(twd).queryByText(/成本/)).not.toBeInTheDocument()
})

test.each([
  ['a set', [makeUsdOpening()]],
  ['an unset', []],
] as const)('opens the form from %s currency', async (_, openings) => {
  const { router } = showOpenings([...openings])

  await userEvent.setup().click(await rowOf('USD 美元'))

  await waitFor(() => expect(router.state.location.pathname).toBe('/assets/opening-balances/USD'))
})

test('says it is loading', async () => {
  fakeBackend({ user: 'me@example.com', currenciesPending: true })
  renderApp('/assets/opening-balances')

  await screen.findByRole('heading', { level: 1, name: '期初餘額' })
  expect(screen.getByText('載入中...')).toBeInTheDocument()
})

test('offers to reload when the opening balances fail', async () => {
  fakeBackend({ user: 'me@example.com', portfolio: { openingsStatus: 500 } })
  renderApp('/assets/opening-balances')

  expect(await screen.findByRole('alert')).toHaveTextContent('無法載入期初餘額，請稍後再試。')
  expect(screen.getByRole('button', { name: '重新載入' })).toBeInTheDocument()
})
