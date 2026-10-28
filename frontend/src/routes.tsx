import type { RouteObject } from 'react-router'
import AppLayout from './components/AppLayout'
import RequireAuth from './components/RequireAuth'
import ChangePasswordPage from './pages/ChangePasswordPage'
import EntryFormPage from './pages/EntryFormPage'
import ForgotPasswordPage from './pages/ForgotPasswordPage'
import FxFormPage from './pages/FxFormPage'
import FxPage from './pages/FxPage'
import LedgerPage from './pages/LedgerPage'
import LoginPage from './pages/LoginPage'
import RegisterPage from './pages/RegisterPage'
import ResetPasswordPage from './pages/ResetPasswordPage'
import StatusPage from './pages/StatusPage'

export const routes: RouteObject[] = [
  {
    path: '/',
    Component: AppLayout,
    children: [
      { path: 'login', Component: LoginPage },
      { path: 'register', Component: RegisterPage },
      { path: 'forgot-password', Component: ForgotPasswordPage },
      { path: 'reset-password', Component: ResetPasswordPage },
      {
        Component: RequireAuth,
        children: [
          { index: true, Component: LedgerPage },
          { path: 'status', Component: StatusPage },
          { path: 'entries/new', Component: EntryFormPage },
          { path: 'entries/:id', Component: EntryFormPage },
          { path: 'fx', Component: FxPage },
          { path: 'fx/new', Component: FxFormPage },
          { path: 'fx/:id', Component: FxFormPage },
          { path: 'change-password', Component: ChangePasswordPage },
        ],
      },
    ],
  },
]
