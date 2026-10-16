import { useEffect, useState } from 'react'
import Button from '../components/Button'

type Health = { status: string; database: 'up' | 'down' }
type Status = 'checking' | 'ok' | 'error' | 'unknown'
type Row = { name: string; detail: string; status: Status }

async function fetchHealth(): Promise<Health | null> {
  try {
    const res = await fetch('/api/health')
    return (await res.json()) as Health
  } catch {
    return null
  }
}

function toRows(health: Health | null | undefined): Row[] {
  const backend: Row = { name: '後端 API', detail: 'Go 服務 · /api/health', status: 'ok' }
  const database: Row = { name: '資料庫', detail: 'PostgreSQL', status: 'ok' }

  if (health === undefined) {
    backend.status = 'checking'
    database.status = 'checking'
  } else if (health === null) {
    backend.status = 'error'
    backend.detail = '連不到後端服務'
    database.status = 'unknown'
    database.detail = '後端連不到，無法確認'
  } else if (health.database === 'down') {
    database.status = 'error'
    database.detail = '後端連不到資料庫'
  }
  return [backend, database]
}

// 燈號：實心圓點加一圈淡色光暈
const dotColors: Record<Status, string> = {
  ok: 'bg-success ring-success-soft',
  error: 'bg-destructive ring-destructive-soft',
  unknown: 'bg-neutral ring-neutral-soft',
  checking: 'bg-neutral ring-neutral-soft animate-pulse-dot motion-reduce:animate-none',
}

function StatusRow({ name, detail, status }: Row) {
  return (
    <li className="flex items-center gap-3.5 px-5 py-4">
      <span className={`size-2.5 flex-none rounded-full ring-4 ${dotColors[status]}`} aria-hidden="true" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="font-semibold text-heading">{name}</span>
        <span className="text-sm text-muted-foreground">{detail}</span>
      </span>
    </li>
  )
}

function StatusPage() {
  const [health, setHealth] = useState<Health | null>()

  useEffect(() => {
    fetchHealth().then(setHealth)
  }, [])

  const recheck = () => {
    setHealth(undefined)
    fetchHealth().then(setHealth)
  }

  const checking = health === undefined

  return (
    <main className="mx-auto max-w-170 px-5">
      <h1 className="mt-12 text-3xl font-bold tracking-[-0.02em] text-heading">系統狀態</h1>
      <p className="mt-2 mb-7 text-muted-foreground">確認前端、後端 API 與資料庫之間的連線是否正常。</p>

      <section className="rounded-card border border-border bg-card shadow-card">
        <div className="flex items-center justify-between gap-3 border-b border-border px-5 py-4">
          <h2 className="text-base font-semibold text-heading">連線檢查</h2>
          <Button type="button" onClick={recheck} disabled={checking}>
            {checking ? '檢查中…' : '重新檢查'}
          </Button>
        </div>
        <ul className="divide-y divide-border" aria-live="polite">
          {toRows(health).map((row) => (
            <StatusRow key={row.name} {...row} />
          ))}
        </ul>
      </section>
    </main>
  )
}

export default StatusPage
