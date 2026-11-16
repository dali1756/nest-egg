import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'
import { clearUserIfUnauthorized } from './hooks/useCurrentUser'

// 任何請求收到 401 都代表 session 已經失效，在這裡統一登出，頁面不用各自處理
export function createQueryClient() {
  const queryClient: QueryClient = new QueryClient({
    queryCache: new QueryCache({ onError: (err) => clearUserIfUnauthorized(queryClient, err) }),
    mutationCache: new MutationCache({ onError: (err) => clearUserIfUnauthorized(queryClient, err) }),
    defaultOptions: {
      queries: { retry: false },
    },
  })
  return queryClient
}
