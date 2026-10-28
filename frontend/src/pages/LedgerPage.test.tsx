import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test } from 'vitest'
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

describe('calendar', () => {
  freezeTime('2026-10-27T04:00:00Z')

  async function openCalendar(path = '/') {
    fakeBackend({ user: 'me@example.com', ledger: octoberAndSeptember })
    const { router } = renderApp(path)
    const calendar = await screen.findByRole('grid', { name: '月曆' })
    return { router, calendar }
  }

  function dayButtons(calendar: HTMLElement) {
    return within(calendar).getAllByRole('button')
  }

  function pickedDays(calendar = screen.getByRole('grid', { name: '月曆' })) {
    return within(calendar)
      .queryAllByRole('gridcell', { selected: true })
      .map((cell) => within(cell).getByRole('button').getAttribute('aria-label'))
  }

  test('lays out the month from Sunday with one button per day', async () => {
    const { calendar } = await openCalendar()

    const weekdays = within(calendar).getAllByRole('columnheader', { hidden: true })
    expect(weekdays.map((h) => h.textContent)).toEqual([...'日一二三四五六'])
    expect(dayButtons(calendar)).toHaveLength(31)
    const firstWeek = within(calendar).getAllByRole('row')[0]
    expect(within(firstWeek).getAllByRole('gridcell').map((c) => c.textContent)).toEqual(['', '', '', '', '1', '2', '3'])
  })

  test('marks the days that have entries', async () => {
    const { calendar } = await openCalendar()

    await waitFor(() =>
      expect(within(calendar).getAllByRole('button', { name: /有紀錄/ }).map((b) => b.getAttribute('aria-label'))).toEqual([
        '10/5（一），有紀錄',
        '10/23（五），有紀錄',
      ]),
    )
    expect(within(calendar).getByRole('button', { name: '10/26（一）' })).toBeEnabled()
  })

  test('marks today and disables the days after it', async () => {
    const { calendar } = await openCalendar()

    const today = within(calendar).getByRole('button', { name: '10/27（二）' })
    expect(today).toHaveAttribute('aria-current', 'date')
    expect(today).toBeEnabled()
    expect(dayButtons(calendar).filter((b) => b.hasAttribute('aria-current'))).toHaveLength(1)
    expect(dayButtons(calendar).filter((b) => b.hasAttribute('disabled')).map((b) => b.textContent)).toEqual(['28', '29', '30', '31'])
  })

  test('shows only the entries of the day picked on the calendar', async () => {
    const { router, calendar } = await openCalendar()
    const user = userEvent.setup()

    await user.click(await within(calendar).findByRole('button', { name: '10/23（五），有紀錄' }))

    expect(router.state.location.search).toBe('?date=2026-10-23')
    await waitFor(() => expect(screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual(['10/23（五）']))
    expect(entryTexts()).toHaveLength(2)
    expect(screen.queryByText('薪水')).not.toBeInTheDocument()
    expect(pickedDays(calendar)).toEqual(['10/23（五），有紀錄'])
    expect(within(screen.getByRole('region', { name: '本月合計' })).getByText('50,000.00')).toBeInTheDocument()
  })

  test('says so when the picked day has no entries', async () => {
    const { calendar } = await openCalendar()
    const user = userEvent.setup()

    await user.click(within(calendar).getByRole('button', { name: '10/26（一）' }))

    expect(await screen.findByText('這天還沒有紀錄')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 3, name: '10/26（一）' })).toBeInTheDocument()
    expect(screen.queryAllByRole('listitem')).toHaveLength(0)
    expect(screen.queryByText('這個月還沒有紀錄')).not.toBeInTheDocument()
  })

  test('opens the day in the URL within its month', async () => {
    const backend = fakeBackend({ user: 'me@example.com', ledger: octoberAndSeptember })

    renderApp('/?date=2026-09-30')

    expect(await screen.findByRole('heading', { level: 2, name: '2026 年 9 月' })).toBeInTheDocument()
    expect(await screen.findByText('九月的晚餐')).toBeInTheDocument()
    expect(entryTexts()).toHaveLength(1)
    expect(pickedDays()).toEqual(['9/30（三），有紀錄'])
    expect(backend.called('GET', '/api/ledger-entries?month=2026-09')).toBe(true)
  })

  test.each(['2026-02-30', '2026-10-28', 'abc'])('ignores the unusable date %s in the URL', async (date) => {
    fakeBackend({ user: 'me@example.com', ledger: octoberAndSeptember })

    renderApp(`/?date=${date}`)

    expect(await screen.findByRole('heading', { level: 2, name: '2026 年 10 月' })).toBeInTheDocument()
    await waitFor(() => expect(entryTexts()).toHaveLength(3))
    expect(pickedDays()).toEqual([])
  })

  test('goes back to the whole month from a picked day', async () => {
    const { router } = await openCalendar('/?date=2026-10-23')
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: '顯示整個月' }))

    expect(router.state.location.search).toBe('?month=2026-10')
    await waitFor(() => expect(entryTexts()).toHaveLength(3))
    expect(screen.queryByRole('button', { name: '顯示整個月' })).not.toBeInTheDocument()
  })

  test('clicking the picked day again shows the whole month', async () => {
    const { router, calendar } = await openCalendar('/?date=2026-10-23')
    const user = userEvent.setup()

    await user.click(within(calendar).getByRole('button', { name: /^10\/23（五）/ }))

    expect(router.state.location.search).toBe('?month=2026-10')
    expect(pickedDays(calendar)).toEqual([])
  })

  test('switching months clears the picked day', async () => {
    const { router, calendar } = await openCalendar('/?date=2026-10-23')
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: '上個月' }))

    expect(router.state.location.search).toBe('?month=2026-09')
    expect(pickedDays(calendar)).toEqual([])
  })

  test('records a new entry on the picked day', async () => {
    await openCalendar('/?date=2026-10-26')

    expect(await screen.findByText('這天還沒有紀錄')).toBeInTheDocument()
    const links = screen.getAllByRole('link', { name: '記一筆' })
    expect(links).toHaveLength(2)
    for (const link of links) expect(link).toHaveAttribute('href', '/entries/new?date=2026-10-26')
  })

  test('back-fills yesterday from the calendar and comes back to that day', async () => {
    const { router, calendar } = await openCalendar()
    const user = userEvent.setup()

    await user.click(within(calendar).getByRole('button', { name: '10/26（一）' }))
    await user.click(screen.getAllByRole('link', { name: '記一筆' })[0])
    expect(await screen.findByLabelText('日期')).toHaveValue('2026-10-26')
    await user.click(await screen.findByRole('radio', { name: '飲食' }))
    await user.type(screen.getByLabelText('金額'), '85')
    await user.click(screen.getByRole('button', { name: '儲存' }))

    await waitFor(() => expect(router.state.location.search).toBe('?date=2026-10-26'))
    expect(await screen.findByRole('link', { name: /飲食/ })).toHaveTextContent('−85.00')
    expect(pickedDays()).toEqual(['10/26（一），有紀錄'])
  })

  test('follows the month when going back', async () => {
    const { calendar } = await openCalendar()
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: '上個月' }))

    expect(await screen.findByRole('heading', { level: 2, name: '2026 年 9 月' })).toBeInTheDocument()
    await waitFor(() => expect(dayButtons(calendar)).toHaveLength(30))
    expect(within(calendar).getByRole('button', { name: '9/30（三），有紀錄' })).toBeEnabled()
    expect(dayButtons(calendar).filter((b) => b.hasAttribute('aria-current'))).toHaveLength(0)
  })

  describe('keyboard', () => {
    async function tabIntoCalendar() {
      const user = userEvent.setup()
      act(() => screen.getByRole('button', { name: '上個月' }).focus())
      await user.tab()
      return user
    }

    test('tabs into the calendar once, on today', async () => {
      const { calendar } = await openCalendar()

      const user = await tabIntoCalendar()

      expect(within(calendar).getByRole('button', { name: '10/27（二）' })).toHaveFocus()
      await user.tab()
      expect(calendar).not.toContainElement(document.activeElement as HTMLElement)
    })

    test('tabs into the calendar on the picked day', async () => {
      const { calendar } = await openCalendar('/?date=2026-10-23')

      await tabIntoCalendar()

      expect(within(calendar).getByRole('button', { name: '10/23（五），有紀錄' })).toHaveFocus()
    })

    test('moves between days with the arrow keys', async () => {
      const { calendar } = await openCalendar()
      const user = userEvent.setup()
      act(() => within(calendar).getByRole('button', { name: '10/26（一）' }).focus())

      await user.keyboard('{ArrowRight}')
      expect(within(calendar).getByRole('button', { name: '10/27（二）' })).toHaveFocus()
      await user.keyboard('{ArrowUp}')
      expect(within(calendar).getByRole('button', { name: '10/20（二）' })).toHaveFocus()
    })

    test('follows the month when the arrow keys leave it', async () => {
      const { router, calendar } = await openCalendar()
      const user = userEvent.setup()
      act(() => within(calendar).getByRole('button', { name: '10/1（四）' }).focus())

      await user.keyboard('{ArrowLeft}')

      expect(await screen.findByRole('heading', { level: 2, name: '2026 年 9 月' })).toBeInTheDocument()
      expect(router.state.location.search).toBe('?month=2026-09')
      await waitFor(() => expect(within(calendar).getByRole('button', { name: '9/30（三），有紀錄' })).toHaveFocus())
    })
  })
})
