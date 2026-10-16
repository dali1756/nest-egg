import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useId, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { logout } from '../api/auth'
import { currentUserKey } from '../hooks/useCurrentUser'

const itemClass =
  'block w-full cursor-pointer rounded-control px-3 py-2 text-left text-sm font-semibold text-heading transition-colors hover:bg-brand-soft focus-visible:outline-3 focus-visible:outline-offset-1 focus-visible:outline-ring disabled:cursor-default disabled:opacity-60'

function MenuIcon() {
  return (
    <svg
      className="block"
      width="20"
      height="20"
      viewBox="0 0 20 20"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      aria-hidden="true"
    >
      <path d="M3 5h14M3 10h14M3 15h14" />
    </svg>
  )
}

function UserMenu({ email }: { email: string }) {
  const [open, setOpen] = useState(false)
  const menuId = useId()
  const rootRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const mutation = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      queryClient.setQueryData(currentUserKey, null)
      navigate('/login', { replace: true })
    },
  })

  useEffect(() => {
    if (!open) return
    const onPointerDown = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      setOpen(false)
      buttonRef.current?.focus()
    }
    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open])

  return (
    <div
      ref={rootRef}
      className="relative ml-auto"
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget)) setOpen(false)
      }}
    >
      <button
        ref={buttonRef}
        type="button"
        aria-label="帳號選單"
        aria-expanded={open}
        aria-controls={menuId}
        onClick={() => setOpen((v) => !v)}
        className="flex cursor-pointer items-center justify-center rounded-control border border-border bg-card p-2 text-heading transition-colors hover:border-primary focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-ring"
      >
        <MenuIcon />
      </button>
      <div
        id={menuId}
        hidden={!open}
        className="absolute top-full right-0 z-10 mt-2 w-56 rounded-card border border-border bg-card p-2 shadow-card"
      >
        <p className="truncate border-b border-border px-3 pt-1 pb-2.5 text-sm text-muted-foreground">{email}</p>
        <ul className="mt-2 flex flex-col gap-0.5">
          <li>
            <Link to="/change-password" className={itemClass} onClick={() => setOpen(false)}>
              變更密碼
            </Link>
          </li>
          <li>
            <button type="button" className={itemClass} onClick={() => mutation.mutate()} disabled={mutation.isPending}>
              登出
            </button>
          </li>
        </ul>
      </div>
    </div>
  )
}

export default UserMenu
