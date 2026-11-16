import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { freezeTime } from './test/clock'
import { fakeBackend, type LedgerOptions } from './test/fakeBackend'
import { renderApp } from './test/renderApp'

freezeTime('2026-10-23T04:00:00Z')

test('loading a page after the session has expired sends the user to the login page', async () => {
  const ledger: LedgerOptions = {}
  const backend = fakeBackend({ user: 'me@example.com', ledger })
  const { router } = renderApp('/')
  await screen.findByRole('heading', { level: 2, name: '2026 年 10 月' })

  backend.state.user = null
  ledger.listStatus = 401
  await userEvent.setup().click(screen.getByRole('button', { name: '上個月' }))

  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
})
