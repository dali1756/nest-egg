import { render, screen, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import { fakeBackend } from '../test/fakeBackend'
import StatusPage from './StatusPage'

test('shows backend and database rows from /api/health', async () => {
  fakeBackend({ user: 'me@example.com' })

  render(<StatusPage />)

  expect(screen.getByRole('heading', { level: 1, name: '系統狀態' })).toBeInTheDocument()
  const list = screen.getByRole('list')
  expect(within(list).getByText('後端 API')).toBeInTheDocument()
  expect(within(list).getByText('資料庫')).toBeInTheDocument()
  expect(await screen.findByRole('button', { name: '重新檢查' })).toBeEnabled()
})
