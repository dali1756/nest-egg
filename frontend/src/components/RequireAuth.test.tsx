import { screen, waitFor } from '@testing-library/react'
import { expect, test } from 'vitest'
import { fakeBackend } from '../test/fakeBackend'
import { renderApp } from '../test/renderApp'

test('redirects to /login when not logged in, without checking health', async () => {
  const backend = fakeBackend({ user: null })

  const { router } = renderApp('/')

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  expect(screen.queryByRole('heading', { name: '系統狀態' })).not.toBeInTheDocument()
  expect(backend.called('GET', '/api/health')).toBe(false)
})

test('shows the status page when logged in', async () => {
  fakeBackend({ user: 'me@example.com' })

  const { router } = renderApp('/')

  expect(await screen.findByRole('heading', { name: '系統狀態' })).toBeInTheDocument()
  expect(router.state.location.pathname).toBe('/')
})

test('shows a loading message while checking the session', async () => {
  const backend = fakeBackend({ mePending: true })

  renderApp('/')

  expect(await screen.findByText('載入中…')).toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: '系統狀態' })).not.toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: '登入' })).not.toBeInTheDocument()
  expect(backend.called('GET', '/api/health')).toBe(false)
})
