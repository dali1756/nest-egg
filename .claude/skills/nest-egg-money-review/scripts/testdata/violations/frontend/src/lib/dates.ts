export function today(): string {
  return new Date().toISOString().slice(0, 10)
}

export function day(d: Date): string {
  return d.toISOString().split('T')[0]
}

export function stamp(d: Date): string {
  return d.toISOString()
}
