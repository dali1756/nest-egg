import { screen, waitFor } from '@testing-library/react'
import { expect, test } from 'vitest'
import { fakeBackend } from '../test/fakeBackend'
import { renderApp } from '../test/renderApp'

const loadedLedger = (backend: ReturnType<typeof fakeBackend>) =>
  backend.calls.some((c) => c.path.startsWith('/api/ledger-entries'))

test('redirects to /login when not logged in, without loading the ledger', async () => {
  const backend = fakeBackend({ user: null })

  const { router } = renderApp('/')

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  expect(screen.queryByRole('heading', { name: '記帳' })).not.toBeInTheDocument()
  expect(loadedLedger(backend)).toBe(false)
})

test('shows the ledger page at / when logged in', async () => {
  fakeBackend({ user: 'me@example.com' })

  const { router } = renderApp('/')

  expect(await screen.findByRole('heading', { level: 1, name: '記帳' })).toBeInTheDocument()
  expect(router.state.location.pathname).toBe('/')
})

test('shows the status page at /status when logged in', async () => {
  fakeBackend({ user: 'me@example.com' })

  renderApp('/status')

  expect(await screen.findByRole('heading', { level: 1, name: '系統狀態' })).toBeInTheDocument()
})

test('redirects /status to /login when not logged in, without checking health', async () => {
  const backend = fakeBackend({ user: null })

  const { router } = renderApp('/status')

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  expect(backend.called('GET', '/api/health')).toBe(false)
})

test('shows a loading message while checking the session', async () => {
  const backend = fakeBackend({ mePending: true })

  renderApp('/')

  expect(await screen.findByText('載入中…')).toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: '記帳' })).not.toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: '登入' })).not.toBeInTheDocument()
  expect(loadedLedger(backend)).toBe(false)
})
