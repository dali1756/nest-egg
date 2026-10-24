const taipeiDate = new Intl.DateTimeFormat('en-CA', {
  timeZone: 'Asia/Taipei',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
})

export function taipeiToday(now = new Date()): string {
  return taipeiDate.format(now)
}

export function taipeiMonth(now = new Date()): string {
  return taipeiToday(now).slice(0, 7)
}

const monthPattern = /^\d{4}-(0[1-9]|1[0-2])$/

export function isMonth(value: string): boolean {
  return monthPattern.test(value)
}

function numbers(date: string): number[] {
  return date.split('-').map((part) => Number.parseInt(part, 10))
}

export function shiftMonth(month: string, delta: number): string {
  const [year, m] = numbers(month)
  const index = year * 12 + (m - 1) + delta
  const shiftedMonth = (index % 12) + 1
  return `${Math.floor(index / 12)}-${String(shiftedMonth).padStart(2, '0')}`
}

export function monthLabel(month: string): string {
  const [year, m] = numbers(month)
  return `${year} 年 ${m} 月`
}

const weekdays = '日一二三四五六'

export function dayLabel(date: string): string {
  const [year, m, d] = numbers(date)
  const weekday = new Date(Date.UTC(year, m - 1, d)).getUTCDay()
  return `${m}/${d}（${weekdays[weekday]}）`
}
