import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { fakeBackend } from '../test/fakeBackend'
import { renderApp } from '../test/renderApp'

async function fillChange(current: string, password: string, confirm: string) {
  const user = userEvent.setup()
  const input = await screen.findByLabelText('目前的密碼')
  if (current) await user.type(input, current)
  if (password) await user.type(screen.getByLabelText('新密碼'), password)
  if (confirm) await user.type(screen.getByLabelText('確認新密碼'), confirm)
  await user.click(screen.getByRole('button', { name: '變更密碼' }))
}

test('changes the password and clears the form', async () => {
  const backend = fakeBackend({ user: 'me@example.com' })
  renderApp('/change-password')

  await fillChange('Passw0rd!', 'N3w-password', 'N3w-password')

  expect(await screen.findByRole('status')).toHaveTextContent('密碼已變更，其他裝置都已登出')
  expect(screen.getByLabelText('目前的密碼')).toHaveValue('')
  expect(screen.getByLabelText('新密碼')).toHaveValue('')
  expect(backend.calls.find((c) => c.path === '/api/auth/password/change')?.body).toEqual({
    current_password: 'Passw0rd!',
    password: 'N3w-password',
    confirm_password: 'N3w-password',
  })
})

test.each([
  [403, '目前的密碼不正確'],
  [423, '帳號已鎖定，請聯絡管理員'],
  [422, '資料格式不正確'],
  [429, '嘗試次數過多，請稍後再試'],
  [500, '變更失敗，請稍後再試'],
])('shows an error and clears the current password when the server responds %i', async (status, message) => {
  fakeBackend({ user: 'me@example.com', changeStatus: status })
  renderApp('/change-password')

  await fillChange('Wr0ngpass!', 'N3w-password', 'N3w-password')

  expect(await screen.findByRole('alert')).toHaveTextContent(message)
  expect(screen.getByLabelText('目前的密碼')).toHaveValue('')
})

test('applies the shared password rules to the new password', async () => {
  const backend = fakeBackend({ user: 'me@example.com' })
  renderApp('/change-password')

  await fillChange('Passw0rd!', 'password', 'password')

  expect(await screen.findByLabelText('新密碼')).toHaveAccessibleDescription(/缺少大寫英文字母/)
  expect(backend.called('POST', '/api/auth/password/change')).toBe(false)
})

test('requires the current password', async () => {
  const backend = fakeBackend({ user: 'me@example.com' })
  renderApp('/change-password')

  await fillChange('', 'N3w-password', 'N3w-password')

  expect(await screen.findByText('請輸入目前的密碼')).toBeInTheDocument()
  expect(backend.called('POST', '/api/auth/password/change')).toBe(false)
})

test('sends the user to the login page when the session has expired', async () => {
  fakeBackend({ user: 'me@example.com', changeStatus: 401 })
  const { router } = renderApp('/change-password')

  await fillChange('Passw0rd!', 'N3w-password', 'N3w-password')

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
})

test('is only available when logged in', async () => {
  fakeBackend({ user: null })
  const { router } = renderApp('/change-password')

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
})
