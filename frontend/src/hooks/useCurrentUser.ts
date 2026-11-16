import { useQuery, type QueryClient } from '@tanstack/react-query'
import { getMe } from '../api/auth'
import { ApiError } from '../api/http'

export const currentUserKey = ['me'] as const

export function useCurrentUser() {
  return useQuery({ queryKey: currentUserKey, queryFn: getMe })
}

// 快取的 key 不分使用者，換人登入前一定要清掉，否則下一個人會先看到前一個人的資料
function removeUserData(queryClient: QueryClient) {
  queryClient.removeQueries({ predicate: (query) => query.queryKey[0] !== currentUserKey[0] })
}

export function signOutLocally(queryClient: QueryClient) {
  removeUserData(queryClient)
  queryClient.setQueryData(currentUserKey, null)
}

// session 也可能自己過期，所以登入時也要清一次
export function signInLocally(queryClient: QueryClient) {
  removeUserData(queryClient)
  return queryClient.invalidateQueries({ queryKey: currentUserKey })
}

export function clearUserIfUnauthorized(queryClient: QueryClient, err: Error) {
  if (err instanceof ApiError && err.status === 401) signOutLocally(queryClient)
}
