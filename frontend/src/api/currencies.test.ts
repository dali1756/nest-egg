import { expect, test } from 'vitest'
import { stubFetch } from '../test/stubFetch'
import { getCurrencies } from './currencies'

test('getCurrencies returns the currencies list', async () => {
  const fetchMock = stubFetch(200, { currencies: [{ code: 'TWD', name: '新臺幣', minor_unit: 2 }] })
  await expect(getCurrencies()).resolves.toEqual([{ code: 'TWD', name: '新臺幣', minor_unit: 2 }])
  expect(fetchMock.mock.calls[0][0]).toBe('/api/currencies')
})
