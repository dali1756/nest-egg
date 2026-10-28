import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { freezeTime } from '../test/clock'
import { fakeBackend, type FxOptions } from '../test/fakeBackend'
import { makeFxTrade } from '../test/fxTrades'
import { renderApp } from '../test/renderApp'

freezeTime('2026-10-28T02:00:00Z')

const buy = makeFxTrade({ id: 12, traded_on: '2026-10-27', note: '10 月定期換匯' })
const sell = makeFxTrade({
  id: 13,
  traded_on: '2026-10-15',
  side: 'sell',
  foreign_amount: '500.00',
  twd_amount: '16050.00',
  trade_rate: '32.1',
  spread: '75.00',
  note: '',
})
const september = makeFxTrade({ id: 9, traded_on: '2026-09-30', note: '九月換的美金' })

const octoberAndSeptember: FxOptions = { trades: [buy, sell, september] }

function rowFor(id: number) {
  return screen.getAllByRole('link').find((link) => link.getAttribute('href') === `/fx/${id}`)!
}

test('shows the title and a link to record a trade', async () => {
  fakeBackend({ user: 'me@example.com', fx: octoberAndSeptember })

  renderApp('/fx')

  expect(await screen.findByRole('heading', { level: 1, name: '換匯' })).toBeInTheDocument()
  expect(screen.getAllByRole('link', { name: '＋' })[0]).toHaveAttribute('href', '/fx/new')
})

test('lists the current Taipei month grouped by date, newest first', async () => {
  const backend = fakeBackend({ user: 'me@example.com', fx: octoberAndSeptember })

  renderApp('/fx')

  expect(await screen.findByRole('heading', { level: 2, name: '2026 年 10 月' })).toBeInTheDocument()
  await waitFor(() =>
    expect(screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual(['10/27（二）', '10/15（四）']),
  )
  expect(screen.queryByText('九月換的美金')).not.toBeInTheDocument()
  expect(backend.called('GET', '/api/fx-trades?month=2026-10')).toBe(true)
})

test('shows what each trade received and paid, its rate and how it compares with the market', async () => {
  fakeBackend({ user: 'me@example.com', fx: octoberAndSeptember })

  renderApp('/fx')

  await screen.findByRole('heading', { level: 3, name: '10/27（二）' })
  const buyRow = rowFor(12)
  expect(buyRow).toHaveTextContent('TWD → USD')
  expect(buyRow).toHaveTextContent('USD 1,000.00')
  expect(buyRow).toHaveTextContent('付出 TWD 32,420.00 · 成交匯率 32.42')
  expect(buyRow).toHaveTextContent('比市場多付 120.00')
  expect(buyRow).toHaveTextContent('10 月定期換匯')

  const sellRow = rowFor(13)
  expect(sellRow).toHaveTextContent('USD → TWD')
  expect(sellRow).toHaveTextContent('TWD 16,050.00')
  expect(sellRow).toHaveTextContent('付出 USD 500.00 · 成交匯率 32.1')
  expect(sellRow).toHaveTextContent('比市場少拿 75.00')
})

test('credits the source of the market rates', async () => {
  fakeBackend({ user: 'me@example.com', fx: octoberAndSeptember })

  renderApp('/fx')

  expect(await screen.findByText('市場參考匯率資料來源：中央銀行')).toBeInTheDocument()
})

test('says when a trade has no market rate', async () => {
  fakeBackend({ user: 'me@example.com', fx: { trades: [makeFxTrade({ id: 12, market_rate: null, market_rate_date: null, spread: null })] } })

  renderApp('/fx')

  await waitFor(() => expect(rowFor(12)).toHaveTextContent('查不到市場參考匯率'))
})

test('switches to the previous month and keeps it in the URL', async () => {
  const backend = fakeBackend({ user: 'me@example.com', fx: octoberAndSeptember })
  const { router } = renderApp('/fx')
  const user = userEvent.setup()
  const nav = await screen.findByRole('navigation', { name: '切換月份' })
  expect(within(nav).getByRole('button', { name: '下個月' })).toBeDisabled()

  await user.click(within(nav).getByRole('button', { name: '上個月' }))

  expect(await screen.findByRole('heading', { level: 2, name: '2026 年 9 月' })).toBeInTheDocument()
  expect(router.state.location.search).toBe('?month=2026-09')
  expect(await screen.findByText('九月換的美金')).toBeInTheDocument()
  expect(backend.called('GET', '/api/fx-trades?month=2026-09')).toBe(true)
})

test('opens the month in the URL', async () => {
  fakeBackend({ user: 'me@example.com', fx: octoberAndSeptember })

  renderApp('/fx?month=2026-09')

  expect(await screen.findByText('九月換的美金')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '下個月' })).toBeEnabled()
})

test('suggests recording a trade in an empty month', async () => {
  fakeBackend({ user: 'me@example.com', fx: {} })

  renderApp('/fx')

  expect(await screen.findByText('這個月還沒有換匯紀錄')).toBeInTheDocument()
  expect(screen.getAllByRole('link', { name: '＋' })).toHaveLength(2)
})

test('offers to reload when the list fails', async () => {
  const backend = fakeBackend({ user: 'me@example.com', fx: { listStatus: 500 } })
  renderApp('/fx')

  expect(await screen.findByText('無法載入換匯紀錄，請稍後再試。')).toBeInTheDocument()
  await userEvent.setup().click(screen.getByRole('button', { name: '重新載入' }))

  await waitFor(() => expect(backend.calls.filter((c) => c.path === '/api/fx-trades?month=2026-10')).toHaveLength(2))
})
