import { screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import { fakeBackend } from './test/fakeBackend'
import { renderApp } from './test/renderApp'

test('shows a loading message while the page code is still loading', async () => {
  fakeBackend({ user: null })

  renderApp('/login')

  expect(screen.getByText('載入中...')).toBeInTheDocument()
  expect(await screen.findByRole('heading', { name: '登入' })).toBeInTheDocument()
})
