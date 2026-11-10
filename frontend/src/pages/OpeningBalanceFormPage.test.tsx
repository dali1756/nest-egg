import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test } from 'vitest'
import type { OpeningCost } from '../api/portfolio'
import { freezeTime } from '../test/clock'
import { fakeBackend, type PortfolioOptions } from '../test/fakeBackend'
import { makeUsdOpening } from '../test/portfolio'
import { renderApp } from '../test/renderApp'

freezeTime('2026-11-03T02:00:00Z')

const atMarketRate: OpeningCost = { market_rate: '32', market_rate_date: '2026-10-01', cost: '320000.00', average_cost: '32.0000' }

function openForm(currency: string, portfolio: PortfolioOptions = {}) {
  const backend = fakeBackend({ user: 'me@example.com', portfolio })
  const app = renderApp(`/assets/opening-balances/${currency}`)
  return { backend, ...app }
}

async function fill({ date, amount, twdCost }: { date?: string; amount?: string; twdCost?: string }) {
  const user = userEvent.setup()
  const amountField = await screen.findByLabelText(/^金額/)
  if (date !== undefined) fireEvent.change(screen.getByLabelText('日期'), { target: { value: date } })
  if (amount) await user.type(amountField, amount)
  if (twdCost) await user.type(screen.getByLabelText('台幣成本（選填）'), twdCost)
  return user
}

function previewQueries(backend: ReturnType<typeof fakeBackend>) {
  return backend.calls
    .filter((c) => c.path.startsWith('/api/opening-balances/preview'))
    .map((c) => Object.fromEntries(new URL(c.path, 'http://localhost').searchParams))
}

test('says it is loading', async () => {
  fakeBackend({ user: 'me@example.com', currenciesPending: true })
  renderApp('/assets/opening-balances/USD')

  await screen.findByRole('link', { name: '‹ 期初餘額' })
  expect(screen.getByText('載入中...')).toBeInTheDocument()
})

describe('fields', () => {
  test('a foreign currency has a date, an amount and an optional TWD cost', async () => {
    openForm('USD')

    expect(await screen.findByRole('heading', { level: 1, name: 'USD 美元 期初餘額' })).toBeInTheDocument()
    expect(await screen.findByLabelText('日期')).toHaveValue('2026-11-03')
    expect(screen.getByText('填這天結束時的餘額；這天（含）以前的收支與換匯不算進餘額')).toBeInTheDocument()
    expect(screen.getByLabelText('金額（USD）')).toHaveValue('')
    expect(screen.getByLabelText('台幣成本（選填）')).toHaveValue('')
  })

  test('TWD has only a date and an amount', async () => {
    const { backend } = openForm('TWD')

    await fill({ date: '2026-10-01', amount: '120000' })

    expect(screen.getByRole('heading', { level: 1, name: 'TWD 新臺幣 期初餘額' })).toBeInTheDocument()
    expect(screen.queryByLabelText('台幣成本（選填）')).not.toBeInTheDocument()
    await new Promise((resolve) => setTimeout(resolve, 400))
    expect(previewQueries(backend)).toEqual([])
    expect(screen.queryByText(/市場參考匯率/)).not.toBeInTheDocument()
  })

  test('an unknown currency is not found', async () => {
    openForm('XYZ')

    expect(await screen.findByText('找不到這個幣別')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '回期初餘額' })).toHaveAttribute('href', '/assets/opening-balances')
  })
})

describe('preview', () => {
  test('shows how a blank TWD cost is worked out from the market rate', async () => {
    const { backend } = openForm('USD', { preview: atMarketRate })

    await fill({ date: '2026-10-01', amount: '10000' })

    expect(await screen.findByText('留空時用 10/01 的市場參考匯率換算：10,000.00 × 32 = 320,000.00')).toBeInTheDocument()
    expect(screen.getByText('資料來源：中央銀行')).toHaveClass('text-destructive-text')
    expect(screen.queryByText(/匯率日期/)).not.toBeInTheDocument()
    expect(previewQueries(backend).at(-1)).toEqual({ currency: 'USD', opened_on: '2026-10-01', amount: '10000' })
  })

  test('shows the average cost of the TWD cost the user entered', async () => {
    const { backend } = openForm('USD', { preview: { ...atMarketRate, cost: '315000.00', average_cost: '31.5000' } })

    await fill({ date: '2026-10-01', amount: '10000', twdCost: '315000' })

    expect(await screen.findByText('均價 31.5000')).toBeInTheDocument()
    expect(screen.queryByText(/留空時/)).not.toBeInTheDocument()
    expect(previewQueries(backend).at(-1)).toMatchObject({ twd_cost: '315000' })
  })

  test('asks for the TWD cost when there is no market rate', async () => {
    openForm('USD', { previewFields: { twd_cost: 'required when there is no market rate for this date' } })

    await fill({ date: '2007-06-01', amount: '10000' })

    expect(await screen.findByText('查不到這天的市場參考匯率，請填台幣成本')).toBeInTheDocument()
  })
})

describe('saving', () => {
  const put = (backend: ReturnType<typeof fakeBackend>) => backend.calls.find((c) => c.method === 'PUT')

  test('puts the opening balance and returns to the list', async () => {
    const { backend, router } = openForm('USD', { preview: atMarketRate })

    const user = await fill({ date: '2026-10-01', amount: '10000' })
    await user.click(screen.getByRole('button', { name: '儲存' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/assets/opening-balances'))
    expect(put(backend)).toMatchObject({
      path: '/api/opening-balances/USD',
      body: { opened_on: '2026-10-01', amount: '10000', twd_cost: '' },
    })
    expect(within(await screen.findByRole('link', { name: /USD 美元/ })).getByText('10,000.00')).toBeInTheDocument()
  })

  test('disables the button while saving', async () => {
    openForm('TWD', { savePending: true })

    const user = await fill({ amount: '1000' })
    await user.click(screen.getByRole('button', { name: '儲存' }))

    expect(await screen.findByRole('button', { name: '儲存中...' })).toBeDisabled()
  })

  test('shows the field the server rejected', async () => {
    const { router } = openForm('USD', { saveStatus: 422, saveFields: { twd_cost: 'required when there is no market rate for this date' } })

    const user = await fill({ date: '2007-06-01', amount: '10000' })
    await user.click(screen.getByRole('button', { name: '儲存' }))

    expect(await screen.findByLabelText('台幣成本（選填）')).toHaveAccessibleDescription('查不到這天的市場參考匯率，請填台幣成本')
    expect(router.state.location.pathname).toBe('/assets/opening-balances/USD')
  })

  test('shows a general error when saving fails for another reason', async () => {
    openForm('TWD', { saveStatus: 500 })

    const user = await fill({ amount: '1000' })
    await user.click(screen.getByRole('button', { name: '儲存' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('儲存失敗，請稍後再試')
  })

  test('sends the user to the login page when the session has expired', async () => {
    const { backend, router } = openForm('TWD', { saveStatus: 401 })

    const user = await fill({ amount: '1000' })
    backend.state.user = null
    await user.click(screen.getByRole('button', { name: '儲存' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  })

  test('checks the TWD cost before sending', async () => {
    const { backend } = openForm('USD')

    const user = await fill({ date: '2026-10-01', amount: '10000', twdCost: '0' })
    await user.click(screen.getByRole('button', { name: '儲存' }))

    expect(await screen.findByLabelText('台幣成本（選填）')).toHaveAccessibleDescription('請輸入大於 0 的金額')
    expect(put(backend)).toBeUndefined()
  })
})

describe('changing an opening balance', () => {
  const stored = makeUsdOpening({ twd_cost: '315000.00', cost: '315000.00', average_cost: '31.5000' })

  test('fills in the stored values and offers to delete', async () => {
    openForm('USD', { openings: [stored] })

    expect(await screen.findByLabelText('日期')).toHaveValue('2026-10-01')
    expect(screen.getByLabelText('金額（USD）')).toHaveValue('10000.00')
    expect(screen.getByLabelText('台幣成本（選填）')).toHaveValue('315000.00')
    expect(screen.getByRole('button', { name: '刪除' })).toBeInTheDocument()
  })

  test('a new opening balance cannot be deleted', async () => {
    openForm('USD')

    await screen.findByLabelText('日期')
    expect(screen.queryByRole('button', { name: '刪除' })).not.toBeInTheDocument()
  })

  test('deletes it after confirming and returns to the list', async () => {
    const { backend, router } = openForm('USD', { openings: [stored] })
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: '刪除' }))
    await user.click(screen.getByRole('button', { name: '確定刪除' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/assets/opening-balances'))
    expect(backend.called('DELETE', '/api/opening-balances/USD')).toBe(true)
    expect(within(await screen.findByRole('link', { name: /USD 美元/ })).getByText('未設定')).toBeInTheDocument()
  })
})
