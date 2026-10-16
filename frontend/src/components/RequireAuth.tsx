import { Navigate, Outlet } from 'react-router'
import { useCurrentUser } from '../hooks/useCurrentUser'

function RequireAuth() {
  const { data: user, isPending, isError } = useCurrentUser()

  if (isPending) return <p className="container page-message">載入中…</p>
  if (isError) return <p className="container page-message">無法確認登入狀態，請稍後重新整理。</p>
  if (!user) return <Navigate to="/login" replace />
  return <Outlet />
}

export default RequireAuth
