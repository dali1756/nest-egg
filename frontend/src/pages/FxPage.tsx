import { Link, useSearchParams } from 'react-router'
import type { FxMonth, FxTrade } from '../api/fx'
import Button from '../components/Button'
import { buttonClassName } from '../components/buttonStyles'
import MonthNav from '../components/MonthNav'
import Notice from '../components/Notice'
import { dayLabel, groupByDay, monthOrCurrent } from '../format/date'
import { directionLabel, receivedAndPaid, spreadText } from '../format/fx'
import { useFxMonth } from '../hooks/useFx'

function TradeItem({ trade }: { trade: FxTrade }) {
  const { received, paid } = receivedAndPaid(trade)
  return (
    <li>
      <Link
        to={`/fx/${trade.id}`}
        className="flex items-center gap-3 px-5 py-3.5 transition-colors hover:bg-brand-soft focus-visible:outline-3 focus-visible:outline-offset-1 focus-visible:outline-ring"
      >
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="font-semibold text-heading">{directionLabel(trade)}</span>
          <span className="text-sm text-muted-foreground tabular-nums">
            付出 {paid} · 成交匯率 {trade.trade_rate}
          </span>
          {trade.note && <span className="truncate text-sm text-muted-foreground">{trade.note}</span>}
        </span>
        <span className="flex flex-col items-end text-right">
          <span className="font-semibold text-heading tabular-nums">{received}</span>
          <span className="text-xs text-muted-foreground">{spreadText(trade.side, trade.spread)}</span>
        </span>
      </Link>
    </li>
  )
}

function TradeList({ data }: { data: FxMonth }) {
  return (
    <>
      {groupByDay(data.trades, (trade) => trade.traded_on).map(([date, trades]) => (
        <section key={date} aria-labelledby={`day-${date}`} className="mt-6">
          <h3 id={`day-${date}`} className="mb-2 text-sm font-semibold text-muted-foreground">
            {dayLabel(date)}
          </h3>
          <ul className="divide-y divide-border rounded-card border border-border bg-card shadow-card">
            {trades.map((trade) => (
              <TradeItem key={trade.id} trade={trade} />
            ))}
          </ul>
        </section>
      ))}
      <p className="mt-6 text-xs text-muted-foreground">市場參考匯率資料來源：中央銀行</p>
    </>
  )
}

function FxPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const month = monthOrCurrent(searchParams.get('month'))
  const query = useFxMonth(month)

  const newTradeLink = (
    <Link to="/fx/new" className={buttonClassName()}>
      ＋
    </Link>
  )

  return (
    <main className="mx-auto max-w-170 px-5 pb-16">
      <div className="mt-12 flex items-center justify-between gap-3">
        <h1 className="text-3xl font-bold tracking-[-0.02em] text-heading">換匯</h1>
        {newTradeLink}
      </div>
      <MonthNav month={month} onChange={(next) => setSearchParams({ month: next })} />

      {query.isPending && <p className="mt-6 text-muted-foreground">載入中…</p>}
      {query.isError && (
        <div className="mt-6 flex flex-col items-start gap-3">
          <Notice tone="error">無法載入換匯紀錄，請稍後再試。</Notice>
          <Button type="button" variant="quiet" onClick={() => query.refetch()}>
            重新載入
          </Button>
        </div>
      )}
      {query.isSuccess &&
        (query.data.trades.length > 0 ? (
          <TradeList data={query.data} />
        ) : (
          <div className="mt-10 flex flex-col items-center gap-4 text-muted-foreground">
            <p>這個月還沒有換匯紀錄</p>
            {newTradeLink}
          </div>
        ))}
    </main>
  )
}

export default FxPage
