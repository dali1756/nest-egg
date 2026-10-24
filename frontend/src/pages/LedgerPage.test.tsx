import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { freezeTime } from '../test/clock'
import { makeEntry } from '../test/entries'
import { fakeBackend, type LedgerOptions } from '../test/fakeBackend'
import { renderApp } from '../test/renderApp'

freezeTime('2026-10-23T04:00:00Z')

const food = { id: 1, name: '飲食' }
const supplies = { id: 2, name: '生活用品' }
const salary = { id: 5, name: '薪水' }

const octoberAndSeptember: LedgerOptions = {
  entries: [
    makeEntry({ id: 1, occurred_on: '2026-10-23', category: food, amount: '120.00', note: '午餐' }),
    makeEntry({ id: 2, occurred_on: '2026-10-23', category: supplies, amount: '35.50', note: '牙膏' }),
    makeEntry({ id: 3, occurred_on: '2026-10-05', category: salary, kind: 'income', amount: '50000.00', note: '' }),
    makeEntry({ id: 4, occurred_on: '2026-09-30', category: food, amount: '80.00', note: '九月的晚餐' }),
  ],
  totals: {
    '2026-10': [{ currency: 'TWD', income: '50000.00', expense: '155.50', net: '49844.50' }],
    '2026-09': [{ currency: 'TWD', income: '0.00', expense: '80.00', net: '-80.00' }],
  },
}

function entryTexts() {
  return screen.getAllByRole('listitem').map((li) => li.textContent)
}

test('shows the current Taipei month grouped by date, newest first', async () => {
  const backend = fakeBackend({ user: 'me@example.com', ledger: octoberAndSeptember })

  renderApp('/')

  expect(await screen.findByRole('heading', { level: 2, name: '2026 年 10 月' })).toBeInTheDocument()
  await waitFor(() => expect(screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual(['10/23（五）', '10/5（一）']))
  const texts = entryTexts()
  expect(texts).toHaveLength(3)
  expect(texts[0]).toContain('生活用品')
  expect(texts[0]).toContain('牙膏')
  expect(texts[0]).toContain('−35.50')
  expect(texts[1]).toContain('飲食')
  expect(texts[2]).toContain('薪水')
  expect(texts[2]).toContain('+50,000.00')
  expect(screen.queryByText('九月的晚餐')).not.toBeInTheDocument()
  expect(backend.called('GET', '/api/ledger-entries?month=2026-10')).toBe(true)
})

test('shows the totals exactly as the backend sent them, with thousands separators', async () => {
  fakeBackend({ user: 'me@example.com', ledger: octoberAndSeptember })

  renderApp('/')

  const totals = await screen.findByRole('region', { name: '本月合計' })
  await waitFor(() => expect(within(totals).getByText('50,000.00')).toBeInTheDocument())
  expect(within(totals).getByText('TWD')).toBeInTheDocument()
  expect(within(totals).getByText('155.50')).toBeInTheDocument()
  expect(within(totals).getByText('49,844.50')).toBeInTheDocument()
})

test('links each entry to its edit page and shows the currency', async () => {
  fakeBackend({
    user: 'me@example.com',
    ledger: { entries: [makeEntry({ id: 7, category: { id: 7, name: '股息' }, kind: 'income', amount: '12.34', note: '', currency: 'USD' })] },
  })

  renderApp('/')

  const link = await screen.findByRole('link', { name: /股息/ })
  expect(link).toHaveAttribute('href', '/entries/7')
  expect(link).toHaveTextContent('USD')
  expect(link).toHaveTextContent('+12.34')
})

test('switches to the previous month and keeps it in the URL', async () => {
  const backend = fakeBackend({ user: 'me@example.com', ledger: octoberAndSeptember })
  const { router } = renderApp('/')
  const user = userEvent.setup()

  await user.click(await screen.findByRole('button', { name: '上個月' }))

  expect(await screen.findByRole('heading', { level: 2, name: '2026 年 9 月' })).toBeInTheDocument()
  expect(router.state.location.search).toBe('?month=2026-09')
  expect(await screen.findByText('九月的晚餐')).toBeInTheDocument()
  expect(within(screen.getByRole('region', { name: '本月合計' })).getByText('-80.00')).toBeInTheDocument()
  expect(backend.called('GET', '/api/ledger-entries?month=2026-09')).toBe(true)
})

test('opens the month in the URL and can go forward to the current month only', async () => {
  fakeBackend({ user: 'me@example.com', ledger: octoberAndSeptember })
  const { router } = renderApp('/?month=2026-09')
  const user = userEvent.setup()

  expect(await screen.findByRole('heading', { level: 2, name: '2026 年 9 月' })).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '下個月' }))

  expect(await screen.findByRole('heading', { level: 2, name: '2026 年 10 月' })).toBeInTheDocument()
  expect(router.state.location.search).toBe('?month=2026-10')
  expect(screen.getByRole('button', { name: '下個月' })).toBeDisabled()
})

test('falls back to the current month when the URL month is malformed', async () => {
  fakeBackend({ user: 'me@example.com', ledger: octoberAndSeptember })

  renderApp('/?month=2026-13')

  expect(await screen.findByRole('heading', { level: 2, name: '2026 年 10 月' })).toBeInTheDocument()
})

test('shows an empty state for a month without entries', async () => {
  fakeBackend({ user: 'me@example.com', ledger: octoberAndSeptember })

  renderApp('/?month=2026-08')

  expect(await screen.findByText('這個月還沒有紀錄')).toBeInTheDocument()
  expect(screen.queryAllByRole('listitem')).toHaveLength(0)
  for (const link of screen.getAllByRole('link', { name: '記一筆' })) expect(link).toHaveAttribute('href', '/entries/new')
})

test('shows an error and a way to retry when loading fails', async () => {
  const backend = fakeBackend({ user: 'me@example.com', ledger: { listStatus: 500 } })
  renderApp('/')
  const user = userEvent.setup()

  expect(await screen.findByRole('alert')).toHaveTextContent('無法載入紀錄')
  expect(screen.queryByText('這個月還沒有紀錄')).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '重新載入' }))

  await waitFor(() => expect(backend.calls.filter((c) => c.path === '/api/ledger-entries?month=2026-10')).toHaveLength(2))
})
