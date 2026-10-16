import { Navigate, Outlet } from 'react-router'
import { useCurrentUser } from '../hooks/useCurrentUser'
import PageMessage from './PageMessage'

function RequireAuth() {
  const { data: user, isPending, isError } = useCurrentUser()

  if (isPending) return <PageMessage>載入中…</PageMessage>
  if (isError) return <PageMessage>無法確認登入狀態，請稍後重新整理。</PageMessage>
  if (!user) return <Navigate to="/login" replace />
  return <Outlet />
}

export default RequireAuth
