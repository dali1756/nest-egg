import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import type { CashBalance } from '../api/portfolio'
import { fakeBackend } from '../test/fakeBackend'
import { makeUsdCash, twdCash } from '../test/portfolio'
import { renderApp } from '../test/renderApp'

function showAssets(cash: CashBalance[]) {
  fakeBackend({ user: 'me@example.com', portfolio: { summary: { cash } } })
  renderApp('/assets')
}

async function rowOf(label: string) {
  return (await screen.findByText(label)).closest('li')!
}

test('lists TWD with only its balance', async () => {
  showAssets([twdCash, makeUsdCash()])

  expect(await screen.findByRole('heading', { level: 1, name: '資產' })).toBeInTheDocument()
  const twd = await rowOf('TWD 新臺幣')
  expect(screen.getByRole('heading', { level: 2, name: '現金' })).toBeInTheDocument()
  expect(within(twd).getByText('125,580.00')).toBeInTheDocument()
  expect(within(twd).queryByText(/均價|匯率|%/)).not.toBeInTheDocument()
})

test('lists USD with its balance, cost, rate, value and P&L', async () => {
  showAssets([twdCash, makeUsdCash()])

  const usd = await rowOf('USD 美元')
  expect(within(usd).getByText('11,000.00')).toBeInTheDocument()
  expect(within(usd).getByText('均價 32.0382')).toBeInTheDocument()
  expect(within(usd).getByText('匯率 32.3（11/02）')).toBeInTheDocument()
  expect(within(usd).getByText('355,300.00')).toBeInTheDocument()
  expect(within(usd).getByText('+2,880.00（+0.82%）')).toBeInTheDocument()
})

test('lists currencies in the order the API gives', async () => {
  showAssets([twdCash, makeUsdCash()])

  await rowOf('USD 美元')
  const labels = screen.getAllByRole('listitem').map((li) => li.textContent ?? '')
  expect(labels[0]).toContain('TWD 新臺幣')
  expect(labels[1]).toContain('USD 美元')
})

test.each([
  ['a gain in red', '2880.00', '0.82', '+2,880.00（+0.82%）', 'text-gain'],
  ['a loss in green', '-1520.00', '-0.43', '−1,520.00（−0.43%）', 'text-loss'],
] as const)('shows %s', async (_, pnl, percent, text, colour) => {
  showAssets([makeUsdCash({ unrealized_pnl: pnl, unrealized_pnl_percent: percent })])

  expect(await screen.findByText(text)).toHaveClass(colour)
})

test('shows no P&L colour when it is flat', async () => {
  showAssets([makeUsdCash({ unrealized_pnl: '0.00', unrealized_pnl_percent: '0.00' })])

  const flat = await screen.findByText('0.00（0.00%）')
  expect(flat).not.toHaveClass('text-gain')
  expect(flat).not.toHaveClass('text-loss')
})

test('points at the income without a market rate', async () => {
  showAssets([
    makeUsdCash({
      balance: '10100.00',
      twd_value: '326230.00',
      average_cost: null,
      cost: null,
      unrealized_pnl: null,
      unrealized_pnl_percent: null,
      unavailable_reason: 'missing_income_rate',
      missing_rate_entry: { id: 42, occurred_on: '2026-10-10' },
    }),
  ])

  const usd = await rowOf('USD 美元')
  expect(within(usd).getByText('326,230.00')).toBeInTheDocument()
  expect(within(usd).getByText('2026-10-10 的收入查不到市場參考匯率，均價與損益算不出來')).toBeInTheDocument()
  expect(within(usd).getByRole('link', { name: '查看這筆' })).toHaveAttribute('href', '/entries/42')
})

test.each([
  [
    'a negative balance',
    { balance: '-500.00', unavailable_reason: 'negative_balance' },
    '餘額是負數，均價與損益算不出來',
  ],
  ['no latest market rate', { market_rate: null, market_rate_date: null, unavailable_reason: 'missing_latest_rate' }, '查不到最近的市場參考匯率，市值與損益算不出來'],
] as const)('explains %s and shows a dash for the value', async (_, overrides, message) => {
  showAssets([
    makeUsdCash({ twd_value: null, average_cost: null, cost: null, unrealized_pnl: null, unrealized_pnl_percent: null, ...overrides }),
  ])

  const usd = await rowOf('USD 美元')
  expect(within(usd).getByText('—')).toBeInTheDocument()
  expect(within(usd).getByText(message)).toBeInTheDocument()
})

test('shows the negative balance as it is', async () => {
  showAssets([makeUsdCash({ balance: '-500.00', twd_value: null, average_cost: null, cost: null, unrealized_pnl: null, unrealized_pnl_percent: null, unavailable_reason: 'negative_balance' })])

  expect(within(await rowOf('USD 美元')).getByText('-500.00')).toBeInTheDocument()
})

test('credits the source of the foreign values in red', async () => {
  showAssets([twdCash, makeUsdCash()])

  expect(await screen.findByText('資料來源：中央銀行')).toHaveClass('text-destructive-text')
  expect(screen.queryByText(/外幣以市場參考匯率折算/)).not.toBeInTheDocument()
})

test('has no rate footnote with only TWD', async () => {
  showAssets([twdCash])

  await rowOf('TWD 新臺幣')
  expect(screen.queryByText(/資料來源/)).not.toBeInTheDocument()
})

test('links to the opening balances', async () => {
  showAssets([twdCash])

  expect(await screen.findByRole('link', { name: '期初餘額 ›' })).toHaveAttribute('href', '/assets/opening-balances')
})

test('invites a new user to set opening balances', async () => {
  showAssets([])

  expect(await screen.findByText('還沒有任何紀錄')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '設定期初餘額' })).toHaveAttribute('href', '/assets/opening-balances')
  expect(screen.queryByRole('heading', { name: '現金' })).not.toBeInTheDocument()
})

test('says it is loading', async () => {
  fakeBackend({ user: 'me@example.com', portfolio: { summaryPending: true } })
  renderApp('/assets')

  await screen.findByRole('heading', { level: 1, name: '資產' })
  expect(screen.getByText('載入中…')).toBeInTheDocument()
})

test('offers to reload when the summary fails', async () => {
  const backend = fakeBackend({ user: 'me@example.com', portfolio: { summaryStatus: 500 } })
  renderApp('/assets')

  expect(await screen.findByRole('alert')).toHaveTextContent('無法載入資產，請稍後再試。')
  await userEvent.setup().click(screen.getByRole('button', { name: '重新載入' }))

  await waitFor(() => expect(backend.calls.filter((c) => c.path === '/api/portfolio/summary')).toHaveLength(2))
})
