import { useEffect, useState } from 'react'

type Health = { status: string; database: 'up' | 'down' }
type Status = 'checking' | 'ok' | 'error' | 'unknown'
type Row = { name: string; detail: string; status: Status }

// 後端連不到時，Vite proxy 會回 502 且沒有 JSON，統一回傳 null
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

function BrandMark() {
  return (
    <svg className="brand-mark" width="32" height="32" viewBox="0 0 32 32" aria-hidden="true">
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

function StatusRow({ name, detail, status }: Row) {
  return (
    <li className="status-row">
      <span className={`status-dot ${status}`} aria-hidden="true" />
      <span className="status-text">
        <span className="status-name">{name}</span>
        <span className="status-detail">{detail}</span>
      </span>
    </li>
  )
}

function App() {
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
    <>
      <header className="site-header">
        <div className="container">
          <BrandMark />
          <span className="brand-name">nest-egg</span>
          <span className="brand-tagline">個人資產追蹤</span>
        </div>
      </header>

      <main className="container">
        <h1 className="page-title">系統狀態</h1>
        <p className="page-lead">確認前端、後端 API 與資料庫之間的連線是否正常。</p>

        <section className="card">
          <div className="card-header">
            <h2 className="card-title">連線檢查</h2>
            <button type="button" className="button" onClick={recheck} disabled={checking}>
              {checking ? '檢查中…' : '重新檢查'}
            </button>
          </div>
          <ul className="status-list" aria-live="polite">
            {toRows(health).map((row) => (
              <StatusRow key={row.name} {...row} />
            ))}
          </ul>
        </section>
      </main>
    </>
  )
}

export default App
