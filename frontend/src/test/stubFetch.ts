import { vi } from 'vitest'

export function stubFetch(status: number, body?: unknown) {
  const fetchMock = vi.fn<typeof fetch>(async () => new Response(body === undefined ? null : JSON.stringify(body), { status }))
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}
