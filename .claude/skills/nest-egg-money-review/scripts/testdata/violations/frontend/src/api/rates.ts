export async function latest() {
  const res = await fetch('https://open.er-api.com/v6/latest/USD')
  return res.json()
}

export async function viaAxios() {
  return axios.get("https://example.com/rates")
}

const TWSE = 'openapi.twse.com.tw/v1/exchangeReport/STOCK_DAY_ALL'

const key = import.meta.env.VITE_FINNHUB_API_KEY

export async function own() {
  return fetch('/api/rates/latest?base=USD&quote=TWD')
}

const title = import.meta.env.VITE_APP_TITLE
