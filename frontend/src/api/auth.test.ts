import { describe, expect, test, vi } from 'vitest'
import { ApiError, getMe, login, logout, register } from './auth'

function stubFetch(status: number, body?: unknown) {
  const fetchMock = vi.fn(async () =>
    new Response(body === undefined ? null : JSON.stringify(body), { status }),
  )
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

describe('getMe', () => {
  test('returns null when not logged in', async () => {
    stubFetch(401)
    await expect(getMe()).resolves.toBeNull()
  })

  test('returns the user when logged in', async () => {
    stubFetch(200, { email: 'me@example.com' })
    await expect(getMe()).resolves.toEqual({ email: 'me@example.com' })
  })

  test('throws ApiError with the status on other failures', async () => {
    stubFetch(500, { error: 'internal error' })
    await expect(getMe()).rejects.toMatchObject({ name: 'ApiError', status: 500 })
  })
})

describe('login', () => {
  test('posts the credentials as JSON', async () => {
    const fetchMock = stubFetch(200, { status: 'success', location: '/' })

    await login({ email: 'me@example.com', password: 'Passw0rd!' })

    expect(fetchMock).toHaveBeenCalledWith('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: 'me@example.com', password: 'Passw0rd!' }),
    })
  })

  test('throws ApiError 401 on bad credentials', async () => {
    stubFetch(401, { status: 'failure' })
    const err = await login({ email: 'me@example.com', password: 'x' }).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(401)
  })
})

describe('register', () => {
  test('posts email, password and confirm_password', async () => {
    const fetchMock = stubFetch(200, { status: 'success', location: '/' })
    const input = { email: 'me@example.com', password: 'Passw0rd!', confirm_password: 'Passw0rd!' }

    await register(input)

    expect(fetchMock).toHaveBeenCalledWith('/api/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    })
  })

  test('throws ApiError 409 when the email exists', async () => {
    stubFetch(409, { status: 'failure' })
    await expect(
      register({ email: 'me@example.com', password: 'Passw0rd!', confirm_password: 'Passw0rd!' }),
    ).rejects.toMatchObject({ status: 409 })
  })
})

describe('logout', () => {
  // Authboss 依 Content-Type 決定回 JSON 而不是 302 轉址，沒有 body 也要帶
  test('posts with a JSON content type', async () => {
    const fetchMock = stubFetch(200, { status: 'success', location: '/login' })

    await logout()

    expect(fetchMock).toHaveBeenCalledWith('/api/auth/logout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: undefined,
    })
  })
})
