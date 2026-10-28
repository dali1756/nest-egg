import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test } from 'vitest'
import type { FxRates } from '../api/fx'
import { freezeTime } from '../test/clock'
import { fakeBackend } from '../test/fakeBackend'
import { makeFxTrade } from '../test/fxTrades'
import { renderApp } from '../test/renderApp'

freezeTime('2026-10-28T02:00:00Z')

const marketRates: FxRates = { trade_rate: '32.42', market_rate: '32.3', market_rate_date: '2026-10-27', spread: '120.00' }

async function fillAmounts(foreign: string, twd: string) {
  const user = userEvent.setup()
  const foreignField = await screen.findByLabelText('換到的外幣')
  if (foreign) await user.type(foreignField, foreign)
  if (twd) await user.type(screen.getByLabelText('付出的台幣'), twd)
  return user
}

function previewCalls(backend: ReturnType<typeof fakeBackend>) {
  return backend.calls
    .filter((c) => c.path.startsWith('/api/fx-trades/preview'))
    .map((c) => Object.fromEntries(new URL(c.path, 'http://localhost').searchParams))
}

const posted = (backend: ReturnType<typeof fakeBackend>) => backend.calls.find((c) => c.method === 'POST' && c.path === '/api/fx-trades')

describe('recording a trade', () => {
  test('defaults to buying the first foreign currency today', async () => {
    fakeBackend({ user: 'me@example.com' })

    renderApp('/fx/new')

    expect(await screen.findByRole('heading', { level: 1, name: '＋' })).toBeInTheDocument()
    expect(await screen.findByRole('radio', { name: '台幣換外幣' })).toBeChecked()
    expect(screen.getByLabelText('日期')).toHaveValue('2026-10-28')
    const currency = screen.getByLabelText('幣別')
    expect(currency).toHaveValue('USD')
    expect(within(currency).getAllByRole('option').map((o) => o.getAttribute('value'))).toEqual(['USD'])
    expect(screen.getByLabelText('換到的外幣')).toBeInTheDocument()
    expect(screen.getByLabelText('付出的台幣')).toBeInTheDocument()
    expect(screen.queryByLabelText(/匯率/)).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/手續費/)).not.toBeInTheDocument()
  })

  test('relabels the amounts when selling foreign currency', async () => {
    fakeBackend({ user: 'me@example.com' })
    renderApp('/fx/new')

    await userEvent.setup().click(await screen.findByRole('radio', { name: '外幣換台幣' }))

    expect(screen.getByLabelText('付出的外幣')).toBeInTheDocument()
    expect(screen.getByLabelText('換到的台幣')).toBeInTheDocument()
  })

  test('saves the trade and returns to its month', async () => {
    const backend = fakeBackend({ user: 'me@example.com', fx: { rates: marketRates } })
    const { router } = renderApp('/fx/new')

    const user = await fillAmounts('1000', '32420')
    await user.type(screen.getByLabelText('備註'), '10 月定期換匯')
    await user.click(screen.getByRole('button', { name: '儲存' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/fx'))
    expect(router.state.location.search).toBe('?month=2026-10')
    expect(posted(backend)?.body).toEqual({
      traded_on: '2026-10-28',
      side: 'buy',
      currency: 'USD',
      foreign_amount: '1000',
      twd_amount: '32420',
      note: '10 月定期換匯',
    })
    expect(await screen.findByText('10 月定期換匯')).toBeInTheDocument()
  })

  test('returns to the month of an earlier trade', async () => {
    fakeBackend({ user: 'me@example.com' })
    const { router } = renderApp('/fx/new')

    const user = await fillAmounts('500', '16050')
    fireEvent.change(screen.getByLabelText('日期'), { target: { value: '2026-09-30' } })
    await user.click(screen.getByRole('button', { name: '儲存' }))

    await waitFor(() => expect(router.state.location.search).toBe('?month=2026-09'))
  })

  test.each([
    ['', '32420', '換到的外幣', '請輸入金額'],
    ['1000.123', '32420', '換到的外幣', '金額最多 2 位小數'],
    ['1000', '1e3', '付出的台幣', '請輸入數字，例如 120 或 32.5'],
    ['1000', '32420.123', '付出的台幣', '金額最多 2 位小數'],
    ['1000', '0', '付出的台幣', '請輸入大於 0 的金額'],
  ])('rejects foreign %j and TWD %j on %s', async (foreign, twd, label, message) => {
    const backend = fakeBackend({ user: 'me@example.com' })
    renderApp('/fx/new')

    const user = await fillAmounts(foreign, twd)
    await user.click(screen.getByRole('button', { name: '儲存' }))

    await waitFor(() => expect(screen.getByLabelText(label)).toHaveAccessibleDescription(message))
    expect(posted(backend)).toBeUndefined()
  })

  test('rejects a date after today in Taipei', async () => {
    const backend = fakeBackend({ user: 'me@example.com' })
    renderApp('/fx/new')

    const user = await fillAmounts('1000', '32420')
    fireEvent.change(screen.getByLabelText('日期'), { target: { value: '2026-10-29' } })
    await user.click(screen.getByRole('button', { name: '儲存' }))

    await waitFor(() => expect(screen.getByLabelText('日期')).toHaveAccessibleDescription('日期不能晚於今天'))
    expect(posted(backend)).toBeUndefined()
  })

  test('shows the fields the server rejected next to those fields', async () => {
    fakeBackend({ user: 'me@example.com', fx: { saveStatus: 422, saveFields: { twd_amount: 'must be greater than 0' } } })
    renderApp('/fx/new')

    const user = await fillAmounts('1000', '32420')
    await user.click(screen.getByRole('button', { name: '儲存' }))

    await waitFor(() => expect(screen.getByLabelText('付出的台幣')).toHaveAccessibleDescription('金額格式不正確'))
    expect(screen.queryByText('儲存失敗，請稍後再試')).not.toBeInTheDocument()
  })

  test('shows a general error when saving fails for another reason', async () => {
    fakeBackend({ user: 'me@example.com', fx: { saveStatus: 500 } })
    renderApp('/fx/new')

    const user = await fillAmounts('1000', '32420')
    await user.click(screen.getByRole('button', { name: '儲存' }))

    expect(await screen.findByText('儲存失敗，請稍後再試')).toBeInTheDocument()
  })
})

describe('previewing the rates', () => {
  test('shows the trade rate, the market rate and the difference once the amounts are filled in', async () => {
    const backend = fakeBackend({ user: 'me@example.com', fx: { rates: marketRates } })
    renderApp('/fx/new')

    await fillAmounts('1000', '32420')

    const preview = await screen.findByRole('region', { name: '試算' })
    await waitFor(() => expect(preview).toHaveTextContent('成交匯率 32.42'))
    expect(preview).toHaveTextContent('市場參考匯率 32.3（2026-10-27）')
    expect(preview).toHaveTextContent('資料來源：中央銀行')
    expect(preview).toHaveTextContent('比市場多付 120.00')
    expect(previewCalls(backend).at(-1)).toEqual({
      side: 'buy',
      currency: 'USD',
      traded_on: '2026-10-28',
      foreign_amount: '1000',
      twd_amount: '32420',
    })
  })

  test('waits until both amounts are valid', async () => {
    const backend = fakeBackend({ user: 'me@example.com', fx: { rates: marketRates } })
    renderApp('/fx/new')

    await fillAmounts('1000', '')
    await new Promise((resolve) => setTimeout(resolve, 500))

    expect(previewCalls(backend)).toEqual([])
    expect(screen.queryByRole('region', { name: '試算' })).not.toBeInTheDocument()
  })

  test('hides the result once an amount is no longer valid', async () => {
    fakeBackend({ user: 'me@example.com', fx: { rates: marketRates } })
    renderApp('/fx/new')
    const user = await fillAmounts('1000', '32420')
    await screen.findByRole('region', { name: '試算' })

    await user.clear(screen.getByLabelText('付出的台幣'))

    await waitFor(() => expect(screen.queryByRole('region', { name: '試算' })).not.toBeInTheDocument())
  })

  test('says when there is no market rate', async () => {
    fakeBackend({ user: 'me@example.com' })
    renderApp('/fx/new')

    await fillAmounts('1000', '32420')

    const preview = await screen.findByRole('region', { name: '試算' })
    await waitFor(() => expect(preview).toHaveTextContent('查不到市場參考匯率'))
    expect(preview).toHaveTextContent('成交匯率 32.42')
    expect(preview).not.toHaveTextContent('資料來源')
  })

  test('still saves when the preview fails', async () => {
    const backend = fakeBackend({ user: 'me@example.com', fx: { previewStatus: 500 } })
    const { router } = renderApp('/fx/new')

    const user = await fillAmounts('1000', '32420')
    await waitFor(() => expect(previewCalls(backend)).not.toEqual([]))
    expect(screen.queryByRole('region', { name: '試算' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '儲存' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/fx'))
  })
})

describe('changing a trade', () => {
  const stored = makeFxTrade({ id: 12, traded_on: '2026-10-27', note: '10 月定期換匯' })

  test('opens the form filled with the trade', async () => {
    fakeBackend({ user: 'me@example.com', fx: { trades: [stored] } })

    renderApp('/fx/12')

    expect(await screen.findByRole('heading', { level: 1, name: '修改換匯' })).toBeInTheDocument()
    expect(await screen.findByRole('radio', { name: '台幣換外幣' })).toBeChecked()
    expect(screen.getByLabelText('日期')).toHaveValue('2026-10-27')
    expect(screen.getByLabelText('幣別')).toHaveValue('USD')
    expect(screen.getByLabelText('換到的外幣')).toHaveValue('1000.00')
    expect(screen.getByLabelText('付出的台幣')).toHaveValue('32420.00')
    expect(screen.getByLabelText('備註')).toHaveValue('10 月定期換匯')
  })

  test('previews with the trade id so the stored market rate is kept', async () => {
    const backend = fakeBackend({ user: 'me@example.com', fx: { trades: [stored], rates: marketRates } })

    renderApp('/fx/12')

    await screen.findByRole('region', { name: '試算' })
    expect(previewCalls(backend).at(-1)?.id).toBe('12')
  })

  test('saves with PUT and returns to the month', async () => {
    const backend = fakeBackend({ user: 'me@example.com', fx: { trades: [stored] } })
    const { router } = renderApp('/fx/12')
    const user = userEvent.setup()

    const twd = await screen.findByLabelText('付出的台幣')
    await user.clear(twd)
    await user.type(twd, '32000')
    await user.click(screen.getByRole('button', { name: '儲存' }))

    await waitFor(() => expect(router.state.location.search).toBe('?month=2026-10'))
    expect(backend.calls.find((c) => c.method === 'PUT' && c.path === '/api/fx-trades/12')?.body).toMatchObject({ twd_amount: '32000' })
  })

  test('deletes the trade after confirming', async () => {
    const backend = fakeBackend({ user: 'me@example.com', fx: { trades: [stored] } })
    const { router } = renderApp('/fx/12')
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: '刪除' }))
    expect(screen.getByText('確定要刪除這筆換匯嗎？')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '確定刪除' }))

    await waitFor(() => expect(router.state.location.search).toBe('?month=2026-10'))
    expect(backend.called('DELETE', '/api/fx-trades/12')).toBe(true)
  })

  test.each(['/fx/999', '/fx/abc'])('shows that %s does not exist', async (path) => {
    fakeBackend({ user: 'me@example.com' })

    renderApp(path)

    expect(await screen.findByText('找不到這筆換匯')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '回換匯頁' })).toHaveAttribute('href', '/fx')
  })
})
