import { vi } from 'vitest'

type Call = { method: string; path: string; body?: Record<string, string> }

type Options = {
  user?: string | null
  mePending?: boolean
  loginStatus?: number
  loginPending?: boolean
  registerStatus?: number
}

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
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
        if (status === 200) state.user = body!.email.toLowerCase()
        return json(status, { status: status === 200 ? 'success' : 'failure' })
      }
      case 'POST /api/auth/register': {
        const status = opts.registerStatus ?? 200
        if (status === 200) state.user = body!.email.toLowerCase()
        return json(status, { status: status === 200 ? 'success' : 'failure' })
      }
      case 'POST /api/auth/logout':
        state.user = null
        return json(200, { status: 'success', location: '/login' })
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
