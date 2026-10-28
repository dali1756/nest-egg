import { describe, expect, test } from 'vitest'
import { makeEntry } from '../test/entries'
import { stubFetch } from '../test/stubFetch'
import { ApiError } from './http'
import { createEntry, deleteEntry, getCategories, getEntry, getMonth, updateEntry } from './ledger'

const jsonHeaders = { 'Content-Type': 'application/json' }

const lunch = makeEntry({ occurred_on: '2026-10-23' })

const input = { occurred_on: '2026-10-23', category_id: 1, currency: 'TWD', amount: '120', note: '午餐' }

describe('reading', () => {
  test('getCategories returns the categories list', async () => {
    const fetchMock = stubFetch(200, { categories: [{ id: 1, kind: 'expense', name: '飲食' }] })
    await expect(getCategories()).resolves.toEqual([{ id: 1, kind: 'expense', name: '飲食' }])
    expect(fetchMock).toHaveBeenCalledWith('/api/categories', { method: 'GET', headers: jsonHeaders, body: undefined })
  })

  test('getMonth asks for the given month', async () => {
    const month = { month: '2026-10', entries: [lunch], totals: [] }
    const fetchMock = stubFetch(200, month)
    await expect(getMonth('2026-10')).resolves.toEqual(month)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/ledger-entries?month=2026-10')
  })

  test('getEntry asks for one entry', async () => {
    const fetchMock = stubFetch(200, lunch)
    await expect(getEntry(1)).resolves.toEqual(lunch)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/ledger-entries/1')
  })

  test('throws ApiError with the status when the entry is missing', async () => {
    stubFetch(404, { error: 'not found' })
    await expect(getEntry(9)).rejects.toMatchObject({ name: 'ApiError', status: 404 })
  })
})

describe('writing', () => {
  test('createEntry posts the input and returns the entry', async () => {
    const fetchMock = stubFetch(201, lunch)
    await expect(createEntry(input)).resolves.toEqual(lunch)
    expect(fetchMock).toHaveBeenCalledWith('/api/ledger-entries', {
      method: 'POST',
      headers: jsonHeaders,
      body: JSON.stringify(input),
    })
  })

  test('updateEntry puts the input to the entry', async () => {
    const fetchMock = stubFetch(200, lunch)
    await updateEntry(1, input)
    expect(fetchMock).toHaveBeenCalledWith('/api/ledger-entries/1', {
      method: 'PUT',
      headers: jsonHeaders,
      body: JSON.stringify(input),
    })
  })

  test('deleteEntry sends DELETE and resolves on 204', async () => {
    const fetchMock = stubFetch(204)
    await expect(deleteEntry(1)).resolves.toBeUndefined()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/ledger-entries/1')
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ method: 'DELETE' })
  })

  test('a 422 carries the invalid fields', async () => {
    stubFetch(422, { error: 'invalid input', fields: { category_id: 'unknown category' } })
    const err = await createEntry(input).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 422, fields: { category_id: 'unknown category' } })
  })
})
