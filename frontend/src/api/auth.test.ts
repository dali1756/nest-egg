import { describe, expect, test, vi } from 'vitest'
import { ApiError, forgotPassword, getMe, login, logout, register, resetPassword } from './auth'

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
    const fetchMock = stubFetch(200, { email: 'me@example.com' })

    await login({ email: 'me@example.com', password: 'Passw0rd!' })

    expect(fetchMock).toHaveBeenCalledWith('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: 'me@example.com', password: 'Passw0rd!' }),
    })
  })

  test('throws ApiError 401 on bad credentials', async () => {
    stubFetch(401, { error: 'invalid credentials' })
    const err = await login({ email: 'me@example.com', password: 'x' }).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(401)
  })
})

describe('register', () => {
  test('posts email, password and confirm_password', async () => {
    const fetchMock = stubFetch(201, { email: 'me@example.com' })
    const input = { email: 'me@example.com', password: 'Passw0rd!', confirm_password: 'Passw0rd!' }

    await register(input)

    expect(fetchMock).toHaveBeenCalledWith('/api/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    })
  })

  test('throws ApiError 409 when the email exists', async () => {
    stubFetch(409, { error: 'email already registered' })
    await expect(
      register({ email: 'me@example.com', password: 'Passw0rd!', confirm_password: 'Passw0rd!' }),
    ).rejects.toMatchObject({ status: 409 })
  })
})

describe('logout', () => {
  test('posts without a body and accepts 204', async () => {
    const fetchMock = stubFetch(204)

    await logout()

    expect(fetchMock).toHaveBeenCalledWith('/api/auth/logout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: undefined,
    })
  })
})

describe('forgotPassword', () => {
  test('posts the email and accepts 202', async () => {
    const fetchMock = stubFetch(202)

    await forgotPassword('me@example.com')

    expect(fetchMock).toHaveBeenCalledWith('/api/auth/password/forgot', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: 'me@example.com' }),
    })
  })

  test('throws ApiError 503 while email is not set up', async () => {
    stubFetch(503, { error: 'password reset unavailable' })
    await expect(forgotPassword('me@example.com')).rejects.toMatchObject({ status: 503 })
  })
})

describe('resetPassword', () => {
  test('posts the token and the new password', async () => {
    const fetchMock = stubFetch(204)
    const input = { token: 'TOKEN', password: 'N3w-password', confirm_password: 'N3w-password' }

    await resetPassword(input)

    expect(fetchMock).toHaveBeenCalledWith('/api/auth/password/reset', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    })
  })

  test('throws ApiError 400 for an invalid token', async () => {
    stubFetch(400, { error: 'invalid or expired token' })
    await expect(
      resetPassword({ token: 'x', password: 'N3w-password', confirm_password: 'N3w-password' }),
    ).rejects.toMatchObject({ status: 400 })
  })
})
