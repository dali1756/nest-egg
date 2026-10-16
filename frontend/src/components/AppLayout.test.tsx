import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { fakeBackend } from '../test/fakeBackend'
import { renderApp } from '../test/renderApp'

test('shows the user email and a logout button when logged in', async () => {
  fakeBackend({ user: 'me@example.com' })
  renderApp('/')

  const banner = await screen.findByRole('banner')
  expect(await screen.findByText('me@example.com')).toBeInTheDocument()
  expect(banner).toContainElement(screen.getByRole('button', { name: '登出' }))
})

test('does not show the logout button when logged out', async () => {
  fakeBackend({ user: null })
  renderApp('/login')

  await screen.findByRole('heading', { name: '登入' })
  expect(screen.queryByRole('button', { name: '登出' })).not.toBeInTheDocument()
})

test('logs out and goes back to the login page', async () => {
  const backend = fakeBackend({ user: 'me@example.com' })
  const { router } = renderApp('/')
  const user = userEvent.setup()

  await user.click(await screen.findByRole('button', { name: '登出' }))

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  expect(backend.called('POST', '/api/auth/logout')).toBe(true)
  expect(await screen.findByRole('heading', { name: '登入' })).toBeInTheDocument()
  expect(screen.queryByText('me@example.com')).not.toBeInTheDocument()

  // 登出後再進首頁，仍會被導回登入頁
  await router.navigate('/')
  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
})

test('clicking the brand mark when logged out goes to the login page', async () => {
  fakeBackend({ user: null })
  const { router } = renderApp('/register')
  const user = userEvent.setup()

  await user.click(await screen.findByRole('link', { name: 'nest-egg' }))

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  expect(await screen.findByRole('heading', { name: '登入' })).toBeInTheDocument()
})

test('clicking the brand mark when logged in stays on the home page', async () => {
  fakeBackend({ user: 'me@example.com' })
  const { router } = renderApp('/')
  const user = userEvent.setup()

  await user.click(await screen.findByRole('link', { name: 'nest-egg' }))

  expect(router.state.location.pathname).toBe('/')
  expect(await screen.findByRole('heading', { level: 1, name: '系統狀態' })).toBeInTheDocument()
})

test('links to the change password page when logged in', async () => {
  fakeBackend({ user: 'me@example.com' })
  renderApp('/')

  expect(await screen.findByRole('link', { name: '變更密碼' })).toHaveAttribute('href', '/change-password')
})
