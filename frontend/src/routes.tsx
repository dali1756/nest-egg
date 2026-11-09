import type { ComponentType } from 'react'
import type { RouteObject } from 'react-router'
import AppLayout from './components/AppLayout'
import PageMessage from './components/PageMessage'
import RequireAuth from './components/RequireAuth'

// 每個頁面各自打包，第一次只載入外框與目前那一頁（表單、月曆套件只有用到的頁面才下載）
function lazyPage(load: () => Promise<{ default: ComponentType }>) {
  return async () => ({ Component: (await load()).default })
}

export const routes: RouteObject[] = [
  {
    path: '/',
    Component: AppLayout,
    HydrateFallback: () => <PageMessage>載入中...</PageMessage>,
    children: [
      { path: 'login', lazy: lazyPage(() => import('./pages/LoginPage')) },
      { path: 'register', lazy: lazyPage(() => import('./pages/RegisterPage')) },
      { path: 'forgot-password', lazy: lazyPage(() => import('./pages/ForgotPasswordPage')) },
      { path: 'reset-password', lazy: lazyPage(() => import('./pages/ResetPasswordPage')) },
      {
        Component: RequireAuth,
        children: [
          { index: true, lazy: lazyPage(() => import('./pages/LedgerPage')) },
          { path: 'status', lazy: lazyPage(() => import('./pages/StatusPage')) },
          { path: 'entries/new', lazy: lazyPage(() => import('./pages/EntryFormPage')) },
          { path: 'entries/:id', lazy: lazyPage(() => import('./pages/EntryFormPage')) },
          { path: 'fx', lazy: lazyPage(() => import('./pages/FxPage')) },
          { path: 'fx/new', lazy: lazyPage(() => import('./pages/FxFormPage')) },
          { path: 'fx/:id', lazy: lazyPage(() => import('./pages/FxFormPage')) },
          { path: 'assets', lazy: lazyPage(() => import('./pages/AssetsPage')) },
          { path: 'assets/opening-balances', lazy: lazyPage(() => import('./pages/OpeningBalancesPage')) },
          { path: 'assets/opening-balances/:currency', lazy: lazyPage(() => import('./pages/OpeningBalanceFormPage')) },
          { path: 'change-password', lazy: lazyPage(() => import('./pages/ChangePasswordPage')) },
        ],
      },
    ],
  },
]
