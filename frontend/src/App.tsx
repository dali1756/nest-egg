import { useEffect, useState } from 'react'

type Health = { status: string; database: 'up' | 'down' }

// 後端連不到時，Vite proxy 會回 502 且沒有 JSON，統一回傳 null
async function fetchHealth(): Promise<Health | null> {
  try {
    const res = await fetch('/api/health')
    return (await res.json()) as Health
  } catch {
    return null
  }
}

function App() {
  // undefined：檢查中；null：後端連不到
  const [health, setHealth] = useState<Health | null>()

  useEffect(() => {
    fetchHealth().then(setHealth)
  }, [])

  const recheck = () => {
    setHealth(undefined)
    fetchHealth().then(setHealth)
  }

  const backendUp = health !== null
  const databaseUp = health?.database === 'up'

  return (
    <main>
      <h1>nest-egg</h1>
      {health === undefined ? (
        <p>檢查中…</p>
      ) : (
        <ul className="status">
          <li className={backendUp ? 'up' : 'down'}>
            後端 API：{backendUp ? '正常' : '連不到'}
          </li>
          <li className={databaseUp ? 'up' : 'down'}>
            資料庫：{databaseUp ? '正常' : backendUp ? '連不到' : '未知'}
          </li>
        </ul>
      )}
      <button type="button" onClick={recheck}>
        重新檢查
      </button>
    </main>
  )
}

export default App
