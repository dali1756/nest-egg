import { useQuery, type QueryClient } from '@tanstack/react-query'
import { getMe } from '../api/auth'
import { ApiError } from '../api/http'

export const currentUserKey = ['me'] as const

export function useCurrentUser() {
  return useQuery({ queryKey: currentUserKey, queryFn: getMe })
}

export function clearUserIfUnauthorized(queryClient: QueryClient, err: Error) {
  if (err instanceof ApiError && err.status === 401) queryClient.setQueryData(currentUserKey, null)
}
