import { z } from 'zod'

export const passwordHint = '限定 8～64 個字元，只能用英文大小寫字母、數字與符號（不可有空白）。'

export const passwordSchema = z
  .string()
  .regex(/^[!-~]*$/, '只能用英文字母、數字與符號，不可有空白')
  .min(8, '至少需要 8 個字元')
  .max(64, '最多 64 個字元')
  .regex(/[A-Z]/, '缺少大寫英文字母')
  .regex(/[a-z]/, '缺少小寫英文字母')
  .regex(/[0-9]/, '缺少數字')
  .regex(/[^A-Za-z0-9]/, '缺少符號')
