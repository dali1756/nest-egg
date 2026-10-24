import { vi } from 'vitest'
import type { Category, Currency, DecimalString, Entry, EntryInput, Total } from '../api/ledger'

type Call = { method: string; path: string; body?: Record<string, unknown> }

type Options = {
  user?: string | null
  mePending?: boolean
  loginStatus?: number
  loginPending?: boolean
  registerStatus?: number
  forgotStatus?: number
  resetStatus?: number
  changeStatus?: number
  ledger?: LedgerOptions
}

export type LedgerOptions = {
  entries?: Entry[]
  totals?: Record<string, Total[]>
  listStatus?: number
  saveStatus?: number
  saveFields?: Record<string, string>
  savePending?: boolean
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

const categories: Category[] = [
  { id: 1, kind: 'expense', name: '飲食' },
  { id: 2, kind: 'expense', name: '生活用品' },
  { id: 3, kind: 'expense', name: '美容美髮' },
  { id: 4, kind: 'expense', name: '其他' },
  { id: 5, kind: 'income', name: '薪水' },
  { id: 6, kind: 'income', name: '獎金' },
  { id: 7, kind: 'income', name: '股息' },
  { id: 8, kind: 'income', name: '其他' },
]

const currencies: Currency[] = [
  { code: 'TWD', name: '新臺幣', minor_unit: 2 },
  { code: 'USD', name: '美元', minor_unit: 2 },
]

function twoDecimals(amount: string): DecimalString {
  const [whole, fraction = ''] = amount.split('.')
  return `${whole}.${fraction.padEnd(2, '0')}` as DecimalString
}

function byNewest(a: Entry, b: Entry) {
  if (a.occurred_on !== b.occurred_on) return a.occurred_on < b.occurred_on ? 1 : -1
  return b.id - a.id
}

function fakeLedger(opts: LedgerOptions = {}) {
  const entries = new Map((opts.entries ?? []).map((e) => [e.id, e]))
  let nextId = Math.max(0, ...entries.keys()) + 1

  function toEntry(id: number, input: EntryInput): Entry {
    const category = categories.find((c) => c.id === input.category_id)!
    return {
      id,
      occurred_on: input.occurred_on,
      kind: category.kind,
      category: { id: category.id, name: category.name },
      currency: input.currency,
      amount: twoDecimals(input.amount),
      market_rate: null,
      market_rate_date: null,
      note: input.note,
    }
  }

  function save(id: number, input: EntryInput, status: number): Response | Promise<Response> {
    if (opts.savePending) return new Promise<Response>(() => {})
    const failStatus = opts.saveStatus
    if (failStatus === 422) return json(422, { error: 'invalid input', fields: opts.saveFields ?? {} })
    if (failStatus) return json(failStatus, { error: 'save failed' })
    const entry = toEntry(id, input)
    entries.set(id, entry)
    return json(status, entry)
  }

  function handle(method: string, path: string, body: unknown): Response | Promise<Response> | undefined {
    const url = new URL(path, 'http://localhost')
    if (method === 'GET' && url.pathname === '/api/categories') return json(200, { categories })
    if (method === 'GET' && url.pathname === '/api/currencies') return json(200, { currencies })
    if (url.pathname === '/api/ledger-entries') {
      if (method === 'POST') return save(nextId++, body as EntryInput, 201)
      if (opts.listStatus) return json(opts.listStatus, { error: 'list failed' })
      const month = url.searchParams.get('month') ?? ''
      const list = [...entries.values()].filter((e) => e.occurred_on.startsWith(month)).sort(byNewest)
      return json(200, { month, entries: list, totals: opts.totals?.[month] ?? [] })
    }
    const match = url.pathname.match(/^\/api\/ledger-entries\/(\d+)$/)
    if (!match) return undefined
    const id = Number.parseInt(match[1], 10)
    const entry = entries.get(id)
    if (!entry) return json(404, { error: 'not found' })
    if (method === 'GET') return json(200, entry)
    if (method === 'PUT') return save(id, body as EntryInput, 200)
    if (method === 'DELETE') {
      entries.delete(id)
      return empty(204)
    }
    return undefined
  }

  return { handle }
}

export function fakeBackend(opts: Options = {}) {
  const state = { user: opts.user ?? null }
  const calls: Call[] = []
  const ledger = fakeLedger(opts.ledger)

  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    const path = String(input)
    const body = init?.body ? (JSON.parse(String(init.body)) as Record<string, unknown>) : undefined
    calls.push({ method, path, body })

    const ledgerResponse = ledger.handle(method, path, body)
    if (ledgerResponse) return ledgerResponse

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
        state.user = String(body!.email).trim().toLowerCase()
        return json(200, { email: state.user })
      }
      case 'POST /api/auth/register': {
        const status = opts.registerStatus ?? 201
        if (status !== 201) return json(status, { error: 'register failed' })
        state.user = String(body!.email).trim().toLowerCase()
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
