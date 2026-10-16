import { describe, expect, test } from 'vitest'
import { passwordSchema } from './password'

describe('passwordSchema', () => {
  test.each(['Passw0rd!', 'Aa1!aaaa', 'Aa1' + '~'.repeat(61), 'Aa1`{}|\\"\''])('accepts %s', (password) => {
    expect(passwordSchema.safeParse(password).success).toBe(true)
  })

  test.each([
    ['', '至少需要 8 個字元'],
    ['Pa0rd!', '至少需要 8 個字元'],
    ['Aa1!' + 'a'.repeat(61), '最多 64 個字元'],
    ['Passw0rd', '缺少符號'],
    ['passw0rd!', '缺少大寫英文字母'],
    ['PASSW0RD!', '缺少小寫英文字母'],
    ['Password!', '缺少數字'],
    ['Pass w0rd!', '不可有空白'],
    ['Passw0rd!\t', '不可有空白'],
    ['Pässw0rd!', '不可有空白'],
    ['密碼Passw0rd!', '不可有空白'],
  ])('rejects %j (%s)', (password, message) => {
    const result = passwordSchema.safeParse(password)
    expect(result.success).toBe(false)
    expect(result.error?.issues[0].message).toContain(message)
  })
})
