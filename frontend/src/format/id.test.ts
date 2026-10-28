import { expect, test } from 'vitest'
import { parsePathId } from './id'

test.each([
  ['12', 12],
  ['1', 1],
])('parses %s', (raw, id) => {
  expect(parsePathId(raw)).toBe(id)
})

test.each(['abc', '1.5', '-1', '', undefined])('ignores %j', (raw) => {
  expect(parsePathId(raw)).toBeUndefined()
})
