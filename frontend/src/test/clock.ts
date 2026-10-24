import { afterEach, beforeEach, vi } from 'vitest'

export function freezeTime(instant: string) {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(instant))
  })
  afterEach(() => {
    vi.useRealTimers()
  })
}
