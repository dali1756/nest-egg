import { Link, Outlet } from 'react-router'
import { useCurrentUser } from '../hooks/useCurrentUser'
import UserMenu from './UserMenu'

function BrandMark() {
  return (
    <svg className="block flex-none" width="32" height="32" viewBox="0 0 32 32" aria-hidden="true">
      <defs>
        <linearGradient id="brand-gradient" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#fb923c" />
          <stop offset="1" stopColor="#ea580c" />
        </linearGradient>
      </defs>
      <rect width="32" height="32" rx="9" fill="url(#brand-gradient)" />
      <path
        d="M16 5.5c-3.9 0-7.5 6.4-7.5 12.1 0 5.1 3.3 8.9 7.5 8.9s7.5-3.8 7.5-8.9C23.5 11.9 19.9 5.5 16 5.5Z"
        fill="#fff"
      />
    </svg>
  )
}

function AppLayout() {
  const { data: user } = useCurrentUser()

  return (
    <>
      <header className="border-b border-border bg-white/85">
        <div className="flex h-16 items-center gap-2.5 px-6">
          <Link
            to="/"
            className="flex items-center gap-2.5 rounded-lg focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-ring"
          >
            <BrandMark />
            <span className="text-lg font-bold tracking-[-0.01em] text-heading">nest-egg</span>
          </Link>
          <span className="border-l border-border pl-2.5 text-xs text-muted-foreground max-[480px]:hidden">
            個人資產追蹤
          </span>
          {user && <UserMenu email={user.email} />}
        </div>
      </header>
      <Outlet />
    </>
  )
}

export default AppLayout
