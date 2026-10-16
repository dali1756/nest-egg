import { useQuery } from '@tanstack/react-query'
import { getMe } from '../api/auth'

export const currentUserKey = ['me'] as const

export function useCurrentUser() {
  return useQuery({ queryKey: currentUserKey, queryFn: getMe })
}
