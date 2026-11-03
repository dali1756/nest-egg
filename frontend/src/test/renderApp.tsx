import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { createMemoryRouter, RouterProvider, type RouteObject } from 'react-router'
import { routes } from '../routes'

async function preloadLazyRoutes(routeObjects: RouteObject[]): Promise<void> {
  await Promise.all(
    routeObjects.map(async (route) => {
      if (typeof route.lazy === 'function') await route.lazy()
      if (route.children) await preloadLazyRoutes(route.children)
    }),
  )
}

// 頁面是 lazy 載入，每個測試第一次編譯頁面要好幾秒，會超過 findBy 的 1 秒等待，先載入一次，之後的 router 都從快取拿
await preloadLazyRoutes(routes)

export function renderApp(path: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { router }
}
