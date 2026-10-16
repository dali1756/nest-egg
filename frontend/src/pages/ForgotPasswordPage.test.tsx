import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { fakeBackend } from '../test/fakeBackend'
import { renderApp } from '../test/renderApp'

async function submitEmail(email: string) {
  const user = userEvent.setup()
  const input = await screen.findByLabelText('Email')
  if (email) await user.type(input, email)
  await user.click(screen.getByRole('button', { name: '寄出重設連結' }))
}

test('says the link was sent without revealing whether the email exists', async () => {
  const backend = fakeBackend({ user: null })
  renderApp('/forgot-password')

  await submitEmail('me@example.com')

  expect(await screen.findByRole('status')).toHaveTextContent('如果這個 email 有註冊，重設密碼的連結已經寄出')
  expect(backend.calls.find((c) => c.path === '/api/auth/password/forgot')?.body).toEqual({ email: 'me@example.com' })
})

test.each([
  [503, '忘記密碼功能尚未開放，請聯絡管理員'],
  [500, '送出失敗，請稍後再試'],
])('shows an error when the server responds %i', async (status, message) => {
  fakeBackend({ user: null, forgotStatus: status })
  renderApp('/forgot-password')

  await submitEmail('me@example.com')

  expect(await screen.findByRole('alert')).toHaveTextContent(message)
})

test('does not submit an invalid email', async () => {
  const backend = fakeBackend({ user: null })
  renderApp('/forgot-password')

  await submitEmail('not-an-email')

  expect(await screen.findByText('請輸入有效的 email')).toBeInTheDocument()
  expect(backend.called('POST', '/api/auth/password/forgot')).toBe(false)
})

test('links back to the login page', async () => {
  fakeBackend({ user: null })
  renderApp('/forgot-password')

  expect(await screen.findByRole('link', { name: '回到登入' })).toHaveAttribute('href', '/login')
})
