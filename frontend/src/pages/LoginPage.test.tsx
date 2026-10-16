import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { fakeBackend } from '../test/fakeBackend'
import { renderApp } from '../test/renderApp'

async function fillLogin(email: string, password: string) {
  const user = userEvent.setup()
  const emailInput = await screen.findByLabelText('Email')
  if (email) await user.type(emailInput, email)
  if (password) await user.type(screen.getByLabelText('密碼'), password)
  await user.click(screen.getByRole('button', { name: '登入' }))
}

test('logs in and shows the status page', async () => {
  const backend = fakeBackend({ user: null })
  const { router } = renderApp('/login')

  await fillLogin('Me@Example.com', 'Passw0rd!')

  expect(await screen.findByRole('heading', { name: '系統狀態' })).toBeInTheDocument()
  expect(router.state.location.pathname).toBe('/')
  const call = backend.calls.find((c) => c.path === '/api/auth/login')
  expect(call?.body).toEqual({ email: 'Me@Example.com', password: 'Passw0rd!' })
})

test('shows an error and clears the password on bad credentials', async () => {
  fakeBackend({ user: null, loginStatus: 401 })
  const { router } = renderApp('/login')

  await fillLogin('me@example.com', 'Wr0ngpass!')

  expect(await screen.findByRole('alert')).toHaveTextContent('email 或密碼錯誤')
  expect(screen.getByLabelText('密碼')).toHaveValue('')
  expect(router.state.location.pathname).toBe('/login')
})

test('does not submit when fields are empty', async () => {
  const backend = fakeBackend({ user: null })
  renderApp('/login')

  await fillLogin('', '')

  expect(await screen.findByText('請輸入有效的 email')).toBeInTheDocument()
  expect(screen.getByText('請輸入密碼')).toBeInTheDocument()
  expect(screen.getByLabelText('Email')).toHaveAttribute('aria-invalid', 'true')
  expect(screen.getByLabelText('Email')).toHaveAccessibleDescription('請輸入有效的 email')
  expect(backend.called('POST', '/api/auth/login')).toBe(false)
})

test('disables the button while logging in', async () => {
  fakeBackend({ user: null, loginPending: true })
  renderApp('/login')

  await fillLogin('me@example.com', 'Passw0rd!')

  expect(await screen.findByRole('button', { name: '登入中…' })).toBeDisabled()
})

test('redirects to / when already logged in', async () => {
  fakeBackend({ user: 'me@example.com' })
  const { router } = renderApp('/login')

  expect(await screen.findByRole('heading', { name: '系統狀態' })).toBeInTheDocument()
  expect(router.state.location.pathname).toBe('/')
})

test('links to the register page', async () => {
  fakeBackend({ user: null })
  renderApp('/login')

  expect(await screen.findByRole('link', { name: '註冊新帳號' })).toHaveAttribute('href', '/register')
})

test('tells the user when the account is locked', async () => {
  fakeBackend({ user: null, loginStatus: 423 })
  renderApp('/login')

  await fillLogin('me@example.com', 'Passw0rd!')

  expect(await screen.findByRole('alert')).toHaveTextContent('帳號已鎖定，請聯絡管理員')
})

test('links to the forgot password page', async () => {
  fakeBackend({ user: null })
  renderApp('/login')

  expect(await screen.findByRole('link', { name: '忘記密碼？' })).toHaveAttribute('href', '/forgot-password')
})

test('asks the user to wait when there are too many attempts', async () => {
  fakeBackend({ user: null, loginStatus: 429 })
  renderApp('/login')

  await fillLogin('me@example.com', 'Passw0rd!')

  expect(await screen.findByRole('alert')).toHaveTextContent('嘗試次數過多，請稍後再試')
})
