import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test } from 'vitest'
import { fakeBackend } from '../test/fakeBackend'
import { renderApp } from '../test/renderApp'

async function openMenu() {
  const user = userEvent.setup()
  const toggle = await screen.findByRole('button', { name: '帳號選單' })
  await user.click(toggle)
  return { user, toggle }
}

test('keeps the account actions inside the menu until it is opened', async () => {
  fakeBackend({ user: 'me@example.com' })
  renderApp('/')

  const banner = await screen.findByRole('banner')
  const toggle = await screen.findByRole('button', { name: '帳號選單' })
  expect(banner).toContainElement(toggle)
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByRole('button', { name: '登出' })).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '變更密碼' })).not.toBeInTheDocument()
  expect(screen.getByText('me@example.com')).not.toBeVisible()

  await userEvent.setup().click(toggle)

  expect(toggle).toHaveAttribute('aria-expanded', 'true')
  expect(screen.getByText('me@example.com')).toBeVisible()
  expect(screen.getByRole('link', { name: '變更密碼' })).toHaveAttribute('href', '/change-password')
  expect(screen.getByRole('button', { name: '登出' })).toBeVisible()
})

test('does not show the menu when logged out', async () => {
  fakeBackend({ user: null })
  renderApp('/login')

  await screen.findByRole('heading', { name: '登入' })
  expect(screen.queryByRole('button', { name: '帳號選單' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '登出' })).not.toBeInTheDocument()
})

test('clicking the menu button again closes the menu', async () => {
  fakeBackend({ user: 'me@example.com' })
  renderApp('/')
  const { user, toggle } = await openMenu()

  await user.click(toggle)

  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByRole('button', { name: '登出' })).not.toBeInTheDocument()
})

test('Escape closes the menu and returns focus to the menu button', async () => {
  fakeBackend({ user: 'me@example.com' })
  renderApp('/')
  const { user, toggle } = await openMenu()
  await user.tab()
  expect(screen.getByRole('link', { name: '變更密碼' })).toHaveFocus()

  await user.keyboard('{Escape}')

  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  expect(toggle).toHaveFocus()
})

test('clicking outside closes the menu', async () => {
  fakeBackend({ user: 'me@example.com' })
  renderApp('/')
  const { user, toggle } = await openMenu()

  await user.click(await screen.findByRole('heading', { level: 1, name: '記帳' }))

  expect(toggle).toHaveAttribute('aria-expanded', 'false')
})

test('clicking outside closes the menu even when the button never got focus', async () => {
  fakeBackend({ user: 'me@example.com' })
  renderApp('/')
  const toggle = await screen.findByRole('button', { name: '帳號選單' })

  fireEvent.click(toggle)
  expect(toggle).toHaveAttribute('aria-expanded', 'true')
  fireEvent.pointerDown(document.body)

  expect(toggle).toHaveAttribute('aria-expanded', 'false')
})

test('tabbing past the last item closes the menu', async () => {
  fakeBackend({ user: 'me@example.com' })
  renderApp('/')
  const { user, toggle } = await openMenu()

  await user.tab()
  expect(screen.getByRole('link', { name: '變更密碼' })).toHaveFocus()
  await user.tab()
  expect(screen.getByRole('link', { name: '系統狀態' })).toHaveFocus()
  await user.tab()
  expect(screen.getByRole('button', { name: '登出' })).toHaveFocus()
  expect(toggle).toHaveAttribute('aria-expanded', 'true')
  await user.tab()

  expect(toggle).toHaveAttribute('aria-expanded', 'false')
})

test('logs out and goes back to the login page', async () => {
  const backend = fakeBackend({ user: 'me@example.com' })
  const { router } = renderApp('/')
  const { user } = await openMenu()

  await user.click(screen.getByRole('button', { name: '登出' }))

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  expect(backend.called('POST', '/api/auth/logout')).toBe(true)
  expect(await screen.findByRole('heading', { name: '登入' })).toBeInTheDocument()
  expect(screen.queryByText('me@example.com')).not.toBeInTheDocument()

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
  expect(await screen.findByRole('heading', { level: 1, name: '記帳' })).toBeInTheDocument()
})

test('choosing change password opens the page and closes the menu', async () => {
  fakeBackend({ user: 'me@example.com' })
  const { router } = renderApp('/')
  const { user, toggle } = await openMenu()

  await user.click(screen.getByRole('link', { name: '變更密碼' }))

  await waitFor(() => expect(router.state.location.pathname).toBe('/change-password'))
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
})

test('choosing system status opens the status page and closes the menu', async () => {
  fakeBackend({ user: 'me@example.com' })
  const { router } = renderApp('/')
  const { user, toggle } = await openMenu()

  await user.click(screen.getByRole('link', { name: '系統狀態' }))

  await waitFor(() => expect(router.state.location.pathname).toBe('/status'))
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  expect(await screen.findByRole('heading', { level: 1, name: '系統狀態' })).toBeInTheDocument()
})

describe('section tabs', () => {
  async function tabs() {
    const nav = await screen.findByRole('navigation', { name: '功能' })
    return {
      ledger: within(nav).getByRole('link', { name: '記帳' }),
      fx: within(nav).getByRole('link', { name: '換匯' }),
    }
  }

  test.each(['/', '/entries/new'])('marks 記帳 as the current section on %s', async (path) => {
    fakeBackend({ user: 'me@example.com' })
    renderApp(path)

    const { ledger, fx } = await tabs()
    expect(ledger).toHaveAttribute('href', '/')
    expect(ledger).toHaveAttribute('aria-current', 'page')
    expect(fx).not.toHaveAttribute('aria-current')
  })

  test.each(['/fx', '/fx/12'])('marks 換匯 as the current section on %s', async (path) => {
    fakeBackend({ user: 'me@example.com' })
    renderApp(path)

    const { ledger, fx } = await tabs()
    expect(fx).toHaveAttribute('href', '/fx')
    expect(fx).toHaveAttribute('aria-current', 'page')
    expect(ledger).not.toHaveAttribute('aria-current')
  })

  test('marks no section on the status page', async () => {
    fakeBackend({ user: 'me@example.com' })
    renderApp('/status')

    const { ledger, fx } = await tabs()
    expect(ledger).not.toHaveAttribute('aria-current')
    expect(fx).not.toHaveAttribute('aria-current')
  })

  test('switches to the fx page', async () => {
    fakeBackend({ user: 'me@example.com' })
    const { router } = renderApp('/')

    await userEvent.setup().click((await tabs()).fx)

    await waitFor(() => expect(router.state.location.pathname).toBe('/fx'))
    expect((await tabs()).fx).toHaveAttribute('aria-current', 'page')
  })

  test('are hidden when logged out', async () => {
    fakeBackend({ user: null })
    renderApp('/login')

    await screen.findByRole('heading', { name: '登入' })
    expect(screen.queryByRole('navigation', { name: '功能' })).not.toBeInTheDocument()
  })
})
