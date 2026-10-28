import { describe, expect, test } from 'vitest'
import { noteField, pastDateField } from './fields'

describe('pastDateField', () => {
  test.each(['2026-10-28', '2026-01-01'])('accepts %s on or before today', (date) => {
    expect(pastDateField('2026-10-28').safeParse(date).success).toBe(true)
  })

  test('rejects a future date', () => {
    expect(pastDateField('2026-10-28').safeParse('2026-10-29').error?.issues[0].message).toBe('日期不能晚於今天')
  })

  test('asks for a date when it is empty', () => {
    expect(pastDateField('2026-10-28').safeParse('').error?.issues[0].message).toBe('請選擇日期')
  })
})

describe('noteField', () => {
  test('allows 200 characters', () => {
    expect(noteField().safeParse('字'.repeat(200)).success).toBe(true)
  })

  test('rejects 201 characters', () => {
    expect(noteField().safeParse('字'.repeat(201)).success).toBe(false)
  })
})
