// 金額是字串，直接交給 Intl.NumberFormat 格式化，不轉成 number
const twd = new Intl.NumberFormat('zh-TW', { minimumFractionDigits: 2 })

export function formatAmount(amount: `${number}`): string {
  return twd.format(amount)
}

// 台北時區的今天（YYYY-MM-DD）
export function todayInTaipei(): string {
  return new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Taipei' }).format(new Date())
}
