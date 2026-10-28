import { z } from 'zod'

export function pastDateField(today: string) {
  return z.string().min(1, '請選擇日期').refine((date) => date <= today, '日期不能晚於今天')
}

export function noteField() {
  return z.string().max(200, '備註最多 200 字')
}
