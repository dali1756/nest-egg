import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import type { Entry } from '../api/ledger'
import { fakeBackend } from '../test/fakeBackend'
import { renderApp } from '../test/renderApp'

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-23T04:00:00Z'))
})

afterEach(() => {
  vi.useRealTimers()
})

function categoryNames() {
  return within(screen.getByRole('group', { name: '分類' }))
    .getAllByRole('radio')
    .map((radio) => radio.closest('label')?.textContent)
}

test('a new entry defaults to an expense today in TWD with no category chosen', async () => {
  fakeBackend({ user: 'me@example.com' })

  renderApp('/entries/new')

  expect(await screen.findByRole('heading', { level: 1, name: '記一筆' })).toBeInTheDocument()
  expect(await screen.findByRole('radio', { name: '支出' })).toBeChecked()
  expect(screen.getByLabelText('日期')).toHaveValue('2026-10-23')
  await waitFor(() => expect(screen.getByLabelText('幣別')).toHaveValue('TWD'))
  await waitFor(() => expect(categoryNames()).toEqual(['飲食', '生活用品', '美容美髮', '其他']))
  for (const radio of within(screen.getByRole('group', { name: '分類' })).getAllByRole('radio')) expect(radio).not.toBeChecked()
})

test('records an expense and returns to its month', async () => {
  const backend = fakeBackend({ user: 'me@example.com' })
  const { router } = renderApp('/entries/new')
  const user = userEvent.setup()

  await user.click(await screen.findByRole('radio', { name: '飲食' }))
  await user.type(screen.getByLabelText('金額'), '120')
  await user.type(screen.getByLabelText('備註'), '午餐')
  await user.click(screen.getByRole('button', { name: '儲存' }))

  await waitFor(() => expect(router.state.location.pathname).toBe('/'))
  expect(router.state.location.search).toBe('?month=2026-10')
  expect(backend.calls.find((c) => c.method === 'POST' && c.path === '/api/ledger-entries')?.body).toEqual({
    occurred_on: '2026-10-23',
    category_id: 1,
    currency: 'TWD',
    amount: '120',
    note: '午餐',
  })
  expect(await screen.findByText('午餐')).toBeInTheDocument()
})

test('records income on an earlier date and returns to that month', async () => {
  fakeBackend({ user: 'me@example.com' })
  const { router } = renderApp('/entries/new')
  const user = userEvent.setup()

  await user.click(await screen.findByRole('radio', { name: '收入' }))
  await user.click(await screen.findByRole('radio', { name: '薪水' }))
  fireEvent.change(screen.getByLabelText('日期'), { target: { value: '2026-09-05' } })
  await user.type(screen.getByLabelText('金額'), '50000')
  await user.click(screen.getByRole('button', { name: '儲存' }))

  await waitFor(() => expect(router.state.location.search).toBe('?month=2026-09'))
  expect(await screen.findByText('薪水')).toBeInTheDocument()
  expect(screen.getByText('+50,000.00')).toBeInTheDocument()
})

test('switching between expense and income shows that kind of categories and clears the choice', async () => {
  fakeBackend({ user: 'me@example.com' })
  renderApp('/entries/new')
  const user = userEvent.setup()

  await user.click(await screen.findByRole('radio', { name: '飲食' }))
  await user.click(screen.getByRole('radio', { name: '收入' }))

  expect(categoryNames()).toEqual(['薪水', '獎金', '股息', '其他'])
  await user.click(screen.getByRole('radio', { name: '支出' }))
  expect(screen.getByRole('radio', { name: '飲食' })).not.toBeChecked()
})

test('sends a single request while saving', async () => {
  const backend = fakeBackend({ user: 'me@example.com', ledger: { savePending: true } })
  renderApp('/entries/new')
  const user = userEvent.setup()

  await user.click(await screen.findByRole('radio', { name: '飲食' }))
  await user.type(screen.getByLabelText('金額'), '120')
  await user.click(screen.getByRole('button', { name: '儲存' }))
  const saving = await screen.findByRole('button', { name: '儲存中…' })
  await user.click(saving)

  expect(saving).toBeDisabled()
  expect(backend.calls.filter((c) => c.method === 'POST' && c.path === '/api/ledger-entries')).toHaveLength(1)
})

test('is only available when logged in', async () => {
  fakeBackend({ user: null })
  const { router } = renderApp('/entries/new')

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
})

async function fillAndSave({ category = '飲食', amount = '120', date }: { category?: string | null; amount?: string; date?: string }) {
  const user = userEvent.setup()
  if (category) await user.click(await screen.findByRole('radio', { name: category }))
  else await screen.findByRole('radio', { name: '飲食' })
  if (amount) await user.type(screen.getByLabelText('金額'), amount)
  if (date !== undefined) fireEvent.change(screen.getByLabelText('日期'), { target: { value: date } })
  await user.click(screen.getByRole('button', { name: '儲存' }))
  return user
}

const posted = (backend: ReturnType<typeof fakeBackend>) => backend.called('POST', '/api/ledger-entries')

test('requires a category', async () => {
  const backend = fakeBackend({ user: 'me@example.com' })
  renderApp('/entries/new')

  await fillAndSave({ category: null })

  expect(await within(screen.getByRole('group', { name: '分類' })).findByText('請選擇分類')).toBeInTheDocument()
  expect(posted(backend)).toBe(false)
})

test.each([
  ['', '請輸入金額'],
  ['abc', '請輸入數字，例如 120 或 32.5'],
  ['1,000', '請輸入數字，例如 120 或 32.5'],
  ['0', '請輸入大於 0 的金額'],
  ['0.00', '請輸入大於 0 的金額'],
  ['10.005', '金額最多 2 位小數'],
])('rejects the amount %j', async (amount, message) => {
  const backend = fakeBackend({ user: 'me@example.com' })
  renderApp('/entries/new')

  await fillAndSave({ amount })

  await waitFor(() => expect(screen.getByLabelText('金額')).toHaveAccessibleDescription(message))
  expect(posted(backend)).toBe(false)
})

test('rejects a date after today in Taipei', async () => {
  const backend = fakeBackend({ user: 'me@example.com' })
  renderApp('/entries/new')

  await fillAndSave({ date: '2026-10-24' })

  await waitFor(() => expect(screen.getByLabelText('日期')).toHaveAccessibleDescription('日期不能晚於今天'))
  expect(posted(backend)).toBe(false)
})

test('requires a date', async () => {
  const backend = fakeBackend({ user: 'me@example.com' })
  renderApp('/entries/new')

  await fillAndSave({ date: '' })

  await waitFor(() => expect(screen.getByLabelText('日期')).toHaveAccessibleDescription('請選擇日期'))
  expect(posted(backend)).toBe(false)
})

test('limits the note to 200 characters', async () => {
  const backend = fakeBackend({ user: 'me@example.com' })
  renderApp('/entries/new')
  const user = userEvent.setup()

  await user.click(await screen.findByLabelText('備註'))
  await user.paste('午'.repeat(201))
  await fillAndSave({})

  await waitFor(() => expect(screen.getByLabelText('備註')).toHaveAccessibleDescription('備註最多 200 字'))
  expect(posted(backend)).toBe(false)
})

test('shows the fields the server rejected next to those fields', async () => {
  fakeBackend({
    user: 'me@example.com',
    ledger: { saveStatus: 422, saveFields: { category_id: 'unknown category', amount: 'must have at most 2 decimal places' } },
  })
  const { router } = renderApp('/entries/new')

  await fillAndSave({})

  expect(await within(screen.getByRole('group', { name: '分類' })).findByText('找不到這個分類，請重新選擇')).toBeInTheDocument()
  expect(screen.getByLabelText('金額')).toHaveAccessibleDescription('金額格式不正確')
  expect(router.state.location.pathname).toBe('/entries/new')
})

test('shows a general error when saving fails for another reason', async () => {
  fakeBackend({ user: 'me@example.com', ledger: { saveStatus: 500 } })
  renderApp('/entries/new')

  await fillAndSave({})

  expect(await screen.findByRole('alert')).toHaveTextContent('儲存失敗，請稍後再試')
})

test('sends the user to the login page when the session has expired', async () => {
  const backend = fakeBackend({ user: 'me@example.com', ledger: { saveStatus: 401 } })
  const { router } = renderApp('/entries/new')

  await screen.findByRole('radio', { name: '飲食' })
  backend.state.user = null
  await fillAndSave({})

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
})

const lunch: Entry = {
  id: 1,
  occurred_on: '2026-10-20',
  kind: 'expense',
  category: { id: 1, name: '飲食' },
  currency: 'TWD',
  amount: '120.00',
  market_rate: null,
  market_rate_date: null,
  note: '午餐',
}

const dividend: Entry = {
  id: 7,
  occurred_on: '2026-10-19',
  kind: 'income',
  category: { id: 7, name: '股息' },
  currency: 'USD',
  amount: '12.34',
  market_rate: '32.508',
  market_rate_date: '2026-10-17',
  note: '',
}

test('opening an entry from the ledger shows the form filled with it', async () => {
  fakeBackend({ user: 'me@example.com', ledger: { entries: [lunch] } })
  const { router } = renderApp('/')
  const user = userEvent.setup()

  await user.click(await screen.findByRole('link', { name: /午餐/ }))

  await waitFor(() => expect(router.state.location.pathname).toBe('/entries/1'))
  expect(await screen.findByRole('heading', { level: 1, name: '修改紀錄' })).toBeInTheDocument()
  expect(await screen.findByRole('radio', { name: '支出' })).toBeChecked()
  expect(screen.getByRole('radio', { name: '飲食' })).toBeChecked()
  expect(screen.getByLabelText('日期')).toHaveValue('2026-10-20')
  expect(screen.getByLabelText('幣別')).toHaveValue('TWD')
  expect(screen.getByLabelText('金額')).toHaveValue('120.00')
  expect(screen.getByLabelText('備註')).toHaveValue('午餐')
})

test('saves changes with PUT and returns to the entry month', async () => {
  const backend = fakeBackend({ user: 'me@example.com', ledger: { entries: [lunch] } })
  const { router } = renderApp('/entries/1')
  const user = userEvent.setup()

  const amount = await screen.findByLabelText('金額')
  await user.clear(amount)
  await user.type(amount, '150')
  await user.click(screen.getByRole('button', { name: '儲存' }))

  await waitFor(() => expect(router.state.location.search).toBe('?month=2026-10'))
  expect(backend.calls.find((c) => c.method === 'PUT')).toMatchObject({
    path: '/api/ledger-entries/1',
    body: { occurred_on: '2026-10-20', category_id: 1, currency: 'TWD', amount: '150', note: '午餐' },
  })
  expect(await screen.findByText('−150.00')).toBeInTheDocument()
})

test('deletes the entry after confirming', async () => {
  const backend = fakeBackend({ user: 'me@example.com', ledger: { entries: [lunch] } })
  const { router } = renderApp('/entries/1')
  const user = userEvent.setup()

  await user.click(await screen.findByRole('button', { name: '刪除' }))
  expect(backend.called('DELETE', '/api/ledger-entries/1')).toBe(false)
  await user.click(screen.getByRole('button', { name: '確定刪除' }))

  await waitFor(() => expect(router.state.location.search).toBe('?month=2026-10'))
  expect(backend.called('DELETE', '/api/ledger-entries/1')).toBe(true)
  expect(await screen.findByText('這個月還沒有紀錄')).toBeInTheDocument()
})

test('cancelling the delete keeps the entry', async () => {
  const backend = fakeBackend({ user: 'me@example.com', ledger: { entries: [lunch] } })
  renderApp('/entries/1')
  const user = userEvent.setup()

  await user.click(await screen.findByRole('button', { name: '刪除' }))
  await user.click(screen.getByRole('button', { name: '取消' }))

  expect(screen.getByRole('button', { name: '刪除' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '確定刪除' })).not.toBeInTheDocument()
  expect(backend.called('DELETE', '/api/ledger-entries/1')).toBe(false)
})

test.each(['/entries/999999', '/entries/abc'])('shows that %s does not exist', async (path) => {
  fakeBackend({ user: 'me@example.com', ledger: { entries: [lunch] } })

  renderApp(path)

  expect(await screen.findByText('找不到這筆紀錄')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '回記帳頁' })).toHaveAttribute('href', '/')
})

test('shows the market rate and its source for a foreign entry', async () => {
  fakeBackend({ user: 'me@example.com', ledger: { entries: [dividend] } })

  renderApp('/entries/7')

  expect(await screen.findByText('市場參考匯率 32.508（2026-10-17）')).toBeInTheDocument()
  expect(screen.getByText('資料來源：中央銀行')).toBeInTheDocument()
})

test('says when a foreign entry has no market rate', async () => {
  fakeBackend({ user: 'me@example.com', ledger: { entries: [{ ...dividend, market_rate: null, market_rate_date: null }] } })

  renderApp('/entries/7')

  expect(await screen.findByText('查不到這天的市場參考匯率')).toBeInTheDocument()
  expect(screen.queryByText(/資料來源/)).not.toBeInTheDocument()
})

test('a TWD entry shows no market rate', async () => {
  fakeBackend({ user: 'me@example.com', ledger: { entries: [lunch] } })

  renderApp('/entries/1')

  await screen.findByRole('heading', { level: 1, name: '修改紀錄' })
  await screen.findByLabelText('金額')
  expect(screen.queryByText(/市場參考匯率/)).not.toBeInTheDocument()
})
