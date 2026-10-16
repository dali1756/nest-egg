import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { fakeBackend } from '../test/fakeBackend'
import { renderApp } from '../test/renderApp'

async function fillRegister(email: string, password: string, confirm: string) {
  const user = userEvent.setup()
  const emailInput = await screen.findByLabelText('Email')
  if (email) await user.type(emailInput, email)
  if (password) await user.type(screen.getByLabelText('密碼'), password)
  if (confirm) await user.type(screen.getByLabelText('確認密碼'), confirm)
  await user.click(screen.getByRole('button', { name: '註冊' }))
}

test('registers, logs in and shows the status page', async () => {
  const backend = fakeBackend({ user: null })
  const { router } = renderApp('/register')

  await fillRegister('Me@Example.com', 'Passw0rd!', 'Passw0rd!')

  expect(await screen.findByRole('heading', { name: '系統狀態' })).toBeInTheDocument()
  expect(router.state.location.pathname).toBe('/')
  const call = backend.calls.find((c) => c.path === '/api/auth/register')
  expect(call?.body).toEqual({ email: 'Me@Example.com', password: 'Passw0rd!', confirm_password: 'Passw0rd!' })
})

test('explains the password rules when the password is too weak', async () => {
  const backend = fakeBackend({ user: null })
  renderApp('/register')

  await fillRegister('me@example.com', 'password', 'password')

  expect(await screen.findByLabelText('密碼')).toHaveAttribute('aria-invalid', 'true')
  expect(screen.getByLabelText('密碼')).toHaveAccessibleDescription(/缺少大寫英文字母/)
  expect(backend.called('POST', '/api/auth/register')).toBe(false)
})

test.each([
  ['Passw0rd', '缺少符號'],
  ['passw0rd!', '缺少大寫英文字母'],
  ['PASSW0RD!', '缺少小寫英文字母'],
  ['Password!', '缺少數字'],
  ['Pa0rd!', '至少需要 8 個字元'],
])('rejects %s (%s)', async (password, missing) => {
  const backend = fakeBackend({ user: null })
  renderApp('/register')

  await fillRegister('me@example.com', password, password)

  expect(await screen.findByLabelText('密碼')).toHaveAccessibleDescription(new RegExp(missing))
  expect(backend.called('POST', '/api/auth/register')).toBe(false)
})

test('rejects a different confirmation password', async () => {
  const backend = fakeBackend({ user: null })
  renderApp('/register')

  await fillRegister('me@example.com', 'Passw0rd!', 'Passw0rd?')

  expect(await screen.findByLabelText('確認密碼')).toHaveAccessibleDescription('兩次輸入的密碼不同')
  expect(backend.called('POST', '/api/auth/register')).toBe(false)
})

test.each([
  [409, '這個 email 已經註冊過'],
  [422, '資料格式不正確'],
  [500, '註冊失敗，請稍後再試'],
])('shows a form error when the server responds %i', async (status, message) => {
  fakeBackend({ user: null, registerStatus: status })
  const { router } = renderApp('/register')

  await fillRegister('me@example.com', 'Passw0rd!', 'Passw0rd!')

  expect(await screen.findByRole('alert')).toHaveTextContent(message)
  expect(router.state.location.pathname).toBe('/register')
})

test('links to the login page', async () => {
  fakeBackend({ user: null })
  renderApp('/register')

  expect(await screen.findByRole('link', { name: '登入' })).toHaveAttribute('href', '/login')
})
