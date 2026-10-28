import { Link, Outlet, useLocation } from 'react-router'
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

const sections = [
  { to: '/', label: '記帳', prefix: '/entries/' },
  { to: '/fx', label: '換匯', prefix: '/fx/' },
]

const tabClass =
  'shrink-0 rounded-control px-3 py-1.5 text-sm font-semibold text-muted-foreground transition-colors hover:text-heading focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-ring aria-[current=page]:bg-brand-soft aria-[current=page]:text-primary'

function SectionTabs() {
  const { pathname } = useLocation()
  return (
    <nav
      aria-label="功能"
      className="flex gap-1 max-[480px]:order-last max-[480px]:-mx-1 max-[480px]:w-full max-[480px]:overflow-x-auto max-[480px]:pb-2 min-[481px]:ml-3"
    >
      {sections.map(({ to, label, prefix }) => {
        const current = pathname === to || pathname.startsWith(prefix)
        return (
          <Link key={to} to={to} aria-current={current ? 'page' : undefined} className={tabClass}>
            {label}
          </Link>
        )
      })}
    </nav>
  )
}

function AppLayout() {
  const { data: user } = useCurrentUser()

  return (
    <>
      <header className="border-b border-border bg-white/85">
        <div className="flex min-h-16 flex-wrap items-center gap-x-2.5 px-6">
          <Link
            to="/"
            className="flex items-center gap-2.5 rounded-lg focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-ring"
          >
            <BrandMark />
            <span className="text-lg font-bold tracking-[-0.01em] text-heading">nest-egg</span>
          </Link>
          <span className="border-l border-border pl-2.5 text-xs text-muted-foreground max-[480px]:hidden">
            記帳與資產
          </span>
          {user && <SectionTabs />}
          {user && <UserMenu email={user.email} />}
        </div>
      </header>
      <Outlet />
    </>
  )
}

export default AppLayout
