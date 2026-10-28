import { request, type DecimalString } from './http'

export type Kind = 'income' | 'expense'

export type Category = { id: number; kind: Kind; name: string }

export type Entry = {
  id: number
  occurred_on: string
  kind: Kind
  category: { id: number; name: string }
  currency: string
  amount: DecimalString
  market_rate: DecimalString | null
  market_rate_date: string | null
  note: string
}

export type Total = { currency: string; income: DecimalString; expense: DecimalString; net: DecimalString }

export type MonthLedger = { month: string; entries: Entry[]; totals: Total[] }

export type EntryInput = {
  occurred_on: string
  category_id: number
  currency: string
  amount: string
  note: string
}

export async function getCategories(): Promise<Category[]> {
  const res = await request<{ categories: Category[] }>('GET', '/api/categories')
  return res.categories
}

export function getMonth(month: string): Promise<MonthLedger> {
  return request('GET', `/api/ledger-entries?month=${encodeURIComponent(month)}`)
}

export function getEntry(id: number): Promise<Entry> {
  return request('GET', `/api/ledger-entries/${id}`)
}

export function createEntry(input: EntryInput): Promise<Entry> {
  return request('POST', '/api/ledger-entries', input)
}

export function updateEntry(id: number, input: EntryInput): Promise<Entry> {
  return request('PUT', `/api/ledger-entries/${id}`, input)
}

export function deleteEntry(id: number): Promise<void> {
  return request('DELETE', `/api/ledger-entries/${id}`)
}
