import { ApiError } from './http'

export function fieldErrorsFrom(err: Error): Record<string, string> | undefined {
  return err instanceof ApiError && err.status === 422 ? err.fields : undefined
}

export function rejectedFields<Field extends string>(err: Error, messages: Record<Field, string>): [Field, string][] {
  const fields = fieldErrorsFrom(err) ?? {}
  const names = Object.keys(messages) as Field[]
  return names.filter((name) => fields[name]).map((name) => [name, messages[name]])
}
