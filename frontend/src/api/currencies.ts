import { request } from './http'

export const TWD = 'TWD'

export type Currency = { code: string; name: string; minor_unit: number }

export async function getCurrencies(): Promise<Currency[]> {
  const res = await request<{ currencies: Currency[] }>('GET', '/api/currencies')
  return res.currencies
}
