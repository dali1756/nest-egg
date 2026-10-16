import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { fakeBackend } from '../test/fakeBackend'
import { renderApp } from '../test/renderApp'

async function fillReset(password: string, confirm: string) {
  const user = userEvent.setup()
  const input = await screen.findByLabelText('新密碼')
  if (password) await user.type(input, password)
  if (confirm) await user.type(screen.getByLabelText('確認新密碼'), confirm)
  await user.click(screen.getByRole('button', { name: '重設密碼' }))
}

test('sends the token from the link and asks the user to log in again', async () => {
  const backend = fakeBackend({ user: null })
  renderApp('/reset-password#token=ABC234')

  await fillReset('N3w-password', 'N3w-password')

  expect(await screen.findByRole('status')).toHaveTextContent('密碼已重設，所有裝置都已登出')
  expect(screen.getByRole('link', { name: '前往登入' })).toHaveAttribute('href', '/login')
  expect(backend.calls.find((c) => c.path === '/api/auth/password/reset')?.body).toEqual({
    token: 'ABC234',
    password: 'N3w-password',
    confirm_password: 'N3w-password',
  })
})

test('applies the same password rules as registration', async () => {
  const backend = fakeBackend({ user: null })
  renderApp('/reset-password#token=ABC234')

  await fillReset('Pass w0rd!', 'Pass w0rd!')

  expect(await screen.findByLabelText('新密碼')).toHaveAccessibleDescription(/不可有空白/)
  expect(backend.called('POST', '/api/auth/password/reset')).toBe(false)
})

test('rejects a different confirmation password', async () => {
  const backend = fakeBackend({ user: null })
  renderApp('/reset-password#token=ABC234')

  await fillReset('N3w-password', 'N3w-passworD')

  expect(await screen.findByLabelText('確認新密碼')).toHaveAccessibleDescription('兩次輸入的密碼不同')
  expect(backend.called('POST', '/api/auth/password/reset')).toBe(false)
})

test.each([
  [400, '連結無效或已過期，請重新申請'],
  [422, '資料格式不正確'],
  [500, '重設失敗，請稍後再試'],
])('shows an error when the server responds %i', async (status, message) => {
  fakeBackend({ user: null, resetStatus: status })
  renderApp('/reset-password#token=ABC234')

  await fillReset('N3w-password', 'N3w-password')

  expect(await screen.findByRole('alert')).toHaveTextContent(message)
})

test('asks for a new link when the token is missing', async () => {
  fakeBackend({ user: null })
  renderApp('/reset-password')

  expect(await screen.findByRole('alert')).toHaveTextContent('連結不完整')
  expect(screen.getByRole('link', { name: '重新申請' })).toHaveAttribute('href', '/forgot-password')
  expect(screen.queryByLabelText('新密碼')).not.toBeInTheDocument()
})
