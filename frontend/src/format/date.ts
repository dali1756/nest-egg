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

function dateParts(date: string): number[] {
  return date.split('-').map((part) => Number.parseInt(part, 10))
}

function utcDate(date: string): Date {
  const [year, m, d] = dateParts(date)
  return new Date(Date.UTC(year, m - 1, d))
}

const datePattern = /^\d{4}-\d{2}-\d{2}$/

export function isDate(value: string): boolean {
  return datePattern.test(value) && utcDate(value).toISOString().startsWith(value)
}

export function pickableDate(value: string | null, today: string): string | undefined {
  return value !== null && isDate(value) && value <= today ? value : undefined
}

export function toCalendarDay(date: string): Date {
  const [year, m, d] = dateParts(date)
  return new Date(year, m - 1, d)
}

function twoDigits(value: number): string {
  return String(value).padStart(2, '0')
}

export function fromCalendarDay(day: Date): string {
  return `${day.getFullYear()}-${twoDigits(day.getMonth() + 1)}-${twoDigits(day.getDate())}`
}

export function shiftMonth(month: string, delta: number): string {
  const [year, m] = dateParts(month)
  const index = year * 12 + (m - 1) + delta
  const shiftedMonth = (index % 12) + 1
  return `${Math.floor(index / 12)}-${String(shiftedMonth).padStart(2, '0')}`
}

export function monthLabel(month: string): string {
  const [year, m] = dateParts(month)
  return `${year} 年 ${m} 月`
}

const weekdays = '日一二三四五六'

export function dayLabel(date: string): string {
  const [, m, d] = dateParts(date)
  return `${m}/${d}（${weekdays[utcDate(date).getUTCDay()]}）`
}
