import { focusManager } from '@tanstack/react-query'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test } from 'vitest'
import { freezeTime } from '../test/clock'
import { makeEntry } from '../test/entries'
import { fakeBackend, type LedgerOptions } from '../test/fakeBackend'
import { cachedQueryKeys, renderApp } from '../test/renderApp'

freezeTime('2026-10-23T04:00:00Z')

const previousNote = 'A 的私人午餐'

function previousUserLedger(): LedgerOptions {
  return { entries: [makeEntry({ occurred_on: '2026-10-23', note: previousNote })] }
}

afterEach(() => focusManager.setFocused(undefined))

async function showPreviousUserLedger(ledger: LedgerOptions, changeStatus?: number) {
  const backend = fakeBackend({ user: 'a@example.com', ledger, changeStatus })
  const app = renderApp('/')
  await screen.findByText(previousNote)
  return { backend, ...app }
}

test('logging out keeps nothing of the previous user in the cache', async () => {
  const { queryClient } = await showPreviousUserLedger(previousUserLedger())
  const user = userEvent.setup()

  await user.click(screen.getByRole('button', { name: '帳號選單' }))
  await user.click(screen.getByRole('button', { name: '登出' }))

  await screen.findByRole('heading', { name: '登入' })
  expect(cachedQueryKeys(queryClient)).toEqual([['me']])
})

test('an expired session keeps nothing of the previous user in the cache', async () => {
  const { router, queryClient } = await showPreviousUserLedger(previousUserLedger(), 401)
  await act(() => router.navigate('/change-password'))
  const user = userEvent.setup()

  await user.type(await screen.findByLabelText('目前的密碼'), 'Passw0rd!')
  await user.type(screen.getByLabelText('新密碼'), 'N3w-password')
  await user.type(screen.getByLabelText('確認新密碼'), 'N3w-password')
  await user.click(screen.getByRole('button', { name: '變更密碼' }))

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  expect(cachedQueryKeys(queryClient)).toEqual([['me']])
})

test('resetting the password keeps nothing of the previous user in the cache', async () => {
  const { router, queryClient } = await showPreviousUserLedger(previousUserLedger())
  await act(() => router.navigate('/reset-password#token=ABC234'))
  const user = userEvent.setup()

  await user.type(await screen.findByLabelText('新密碼'), 'N3w-password')
  await user.type(screen.getByLabelText('確認新密碼'), 'N3w-password')
  await user.click(screen.getByRole('button', { name: '重設密碼' }))

  await screen.findByRole('status')
  expect(cachedQueryKeys(queryClient)).toEqual([['me']])
})

async function expireSessionAndOpen(path: string) {
  const ledger = previousUserLedger()
  const { backend, router } = await showPreviousUserLedger(ledger)
  backend.state.user = null
  ledger.listPending = true
  // 切回這個分頁時會重新確認登入狀態，發現 session 已經失效
  act(() => {
    focusManager.setFocused(false)
    focusManager.setFocused(true)
  })
  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  await act(() => router.navigate(path))
  return userEvent.setup()
}

test('logging in after an expired session never shows the previous user data', async () => {
  const user = await expireSessionAndOpen('/login')

  await user.type(await screen.findByLabelText('Email'), 'b@example.com')
  await user.type(screen.getByLabelText('密碼'), 'Passw0rd!')
  await user.click(screen.getByRole('button', { name: '登入' }))

  await screen.findByRole('heading', { name: '記帳' })
  expect(screen.queryByText(previousNote)).not.toBeInTheDocument()
  expect(screen.getByText('載入中...')).toBeInTheDocument()
})

test('registering after an expired session never shows the previous user data', async () => {
  const user = await expireSessionAndOpen('/register')

  await user.type(await screen.findByLabelText('Email'), 'b@example.com')
  await user.type(screen.getByLabelText('密碼'), 'Passw0rd!')
  await user.type(screen.getByLabelText('確認密碼'), 'Passw0rd!')
  await user.click(screen.getByRole('button', { name: '註冊' }))

  await screen.findByRole('heading', { name: '記帳' })
  expect(screen.queryByText(previousNote)).not.toBeInTheDocument()
  expect(screen.getByText('載入中...')).toBeInTheDocument()
})
