export type User = { email: string }

export type LoginInput = { email: string; password: string }
export type RegisterInput = { email: string; password: string; confirm_password: string }
export type ResetPasswordInput = { token: string; password: string; confirm_password: string }

export class ApiError extends Error {
  status: number

  constructor(status: number) {
    super(`request failed with status ${status}`)
    this.name = 'ApiError'
    this.status = status
  }
}

const jsonHeaders = { 'Content-Type': 'application/json' }

async function post(path: string, body?: unknown): Promise<void> {
  const res = await fetch(path, {
    method: 'POST',
    headers: jsonHeaders,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!res.ok) throw new ApiError(res.status)
}

export async function getMe(): Promise<User | null> {
  const res = await fetch('/api/auth/me', { headers: jsonHeaders })
  if (res.status === 401) return null
  if (!res.ok) throw new ApiError(res.status)
  return (await res.json()) as User
}

export function login(input: LoginInput): Promise<void> {
  return post('/api/auth/login', input)
}

export function register(input: RegisterInput): Promise<void> {
  return post('/api/auth/register', input)
}

export function logout(): Promise<void> {
  return post('/api/auth/logout')
}

export function forgotPassword(email: string): Promise<void> {
  return post('/api/auth/password/forgot', { email })
}

export function resetPassword(input: ResetPasswordInput): Promise<void> {
  return post('/api/auth/password/reset', input)
}
