import type { RouteObject } from 'react-router'
import AppLayout from './components/AppLayout'
import RequireAuth from './components/RequireAuth'
import LoginPage from './pages/LoginPage'
import RegisterPage from './pages/RegisterPage'
import StatusPage from './pages/StatusPage'

export const routes: RouteObject[] = [
  {
    path: '/',
    Component: AppLayout,
    children: [
      { path: 'login', Component: LoginPage },
      { path: 'register', Component: RegisterPage },
      {
        Component: RequireAuth,
        children: [{ index: true, Component: StatusPage }],
      },
    ],
  },
]
