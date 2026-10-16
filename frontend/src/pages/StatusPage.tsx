import { useEffect, useState } from 'react'

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
  )
}

export default StatusPage
