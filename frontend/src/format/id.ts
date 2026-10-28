export function parsePathId(id: string | undefined): number | undefined {
  return id !== undefined && /^\d+$/.test(id) ? Number.parseInt(id, 10) : undefined
}
