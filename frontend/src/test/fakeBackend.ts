import { vi } from 'vitest'

type Call = { method: string; path: string; body?: Record<string, string> }

type Options = {
  user?: string | null
  mePending?: boolean
  loginStatus?: number
  loginPending?: boolean
  registerStatus?: number
  forgotStatus?: number
  resetStatus?: number
  changeStatus?: number
}

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function empty(status: number) {
  return new Response(null, { status })
}

export function fakeBackend(opts: Options = {}) {
  const state = { user: opts.user ?? null }
  const calls: Call[] = []

  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    const path = String(input)
    const body = init?.body ? (JSON.parse(String(init.body)) as Record<string, string>) : undefined
    calls.push({ method, path, body })

    switch (`${method} ${path}`) {
      case 'GET /api/auth/me':
        if (opts.mePending) return new Promise<Response>(() => {})
        return state.user ? json(200, { email: state.user }) : json(401, { error: 'unauthorized' })
      case 'GET /api/health':
        return json(200, { status: 'ok', database: 'up' })
      case 'POST /api/auth/login': {
        if (opts.loginPending) return new Promise<Response>(() => {})
        const status = opts.loginStatus ?? 200
        if (status !== 200) return json(status, { error: 'login failed' })
        state.user = body!.email.trim().toLowerCase()
        return json(200, { email: state.user })
      }
      case 'POST /api/auth/register': {
        const status = opts.registerStatus ?? 201
        if (status !== 201) return json(status, { error: 'register failed' })
        state.user = body!.email.trim().toLowerCase()
        return json(201, { email: state.user })
      }
      case 'POST /api/auth/logout':
        state.user = null
        return empty(204)
      case 'POST /api/auth/password/forgot': {
        const status = opts.forgotStatus ?? 202
        return status === 202 ? empty(202) : json(status, { error: 'forgot failed' })
      }
      case 'POST /api/auth/password/reset': {
        const status = opts.resetStatus ?? 204
        if (status !== 204) return json(status, { error: 'reset failed' })
        state.user = null
        return empty(204)
      }
      case 'POST /api/auth/password/change': {
        const status = opts.changeStatus ?? 204
        if (status === 401) state.user = null
        return status === 204 ? empty(204) : json(status, { error: 'change failed' })
      }
    }
    return json(404, { error: 'not found' })
  })
  vi.stubGlobal('fetch', fetchMock)

  return {
    state,
    calls,
    called: (method: string, path: string) => calls.some((c) => c.method === method && c.path === path),
  }
}
