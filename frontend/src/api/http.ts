export class ApiError extends Error {
  status: number
  fields?: Record<string, string>

  constructor(status: number, fields?: Record<string, string>) {
    super(`request failed with status ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.fields = fields
  }
}

export const jsonHeaders = { 'Content-Type': 'application/json' }

async function toApiError(res: Response): Promise<ApiError> {
  if (res.status !== 422) return new ApiError(res.status)
  const body = (await res.json().catch(() => ({}))) as { fields?: Record<string, string> }
  return new ApiError(res.status, body.fields)
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: jsonHeaders,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!res.ok) throw await toApiError(res)
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}
