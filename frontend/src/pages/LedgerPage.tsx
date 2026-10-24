import { Link, useSearchParams } from 'react-router'
import type { Entry, Total } from '../api/ledger'
import Button from '../components/Button'
import { buttonClassName } from '../components/buttonStyles'
import Notice from '../components/Notice'
import { dayLabel, isMonth, monthLabel, shiftMonth, taipeiMonth } from '../format/date'
import { formatAmount } from '../format/money'
import { useLedgerMonth } from '../hooks/useLedger'

function groupByDate(entries: Entry[]): [string, Entry[]][] {
  const groups = new Map<string, Entry[]>()
  for (const entry of entries) groups.set(entry.occurred_on, [...(groups.get(entry.occurred_on) ?? []), entry])
  return [...groups]
}

function SignedAmount({ entry }: { entry: Entry }) {
  const expense = entry.kind === 'expense'
  return (
    <span className={`font-semibold tabular-nums ${expense ? 'text-destructive-text' : 'text-success-text'}`}>
      {expense ? '−' : '+'}
      {formatAmount(entry.amount)}
    </span>
  )
}

function MonthNav({ month, onChange }: { month: string; onChange: (month: string) => void }) {
  const navButton =
    'flex size-9 cursor-pointer items-center justify-center rounded-control border border-border bg-card text-lg text-heading transition-colors enabled:hover:border-primary focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-default disabled:opacity-40'
  return (
    <nav aria-label="切換月份" className="mt-6 flex items-center gap-3">
      <button type="button" aria-label="上個月" className={navButton} onClick={() => onChange(shiftMonth(month, -1))}>
        ‹
      </button>
      <h2 className="min-w-32 text-center text-lg font-semibold text-heading">{monthLabel(month)}</h2>
      <button
        type="button"
        aria-label="下個月"
        className={navButton}
        disabled={month >= taipeiMonth()}
        onClick={() => onChange(shiftMonth(month, 1))}
      >
        ›
      </button>
    </nav>
  )
}

function Totals({ totals }: { totals: Total[] }) {
  return (
    <section aria-label="本月合計" className="mt-5 rounded-card border border-border bg-card px-5 py-4 shadow-card">
      {totals.map((total) => (
        <div key={total.currency} className="flex flex-wrap items-baseline gap-x-6 gap-y-1 py-1.5">
          <span className="w-12 text-sm font-semibold text-muted-foreground">{total.currency}</span>
          <dl className="grid flex-1 grid-cols-3 gap-3">
            {(
              [
                ['收入', total.income],
                ['支出', total.expense],
                ['結餘', total.net],
              ] as const
            ).map(([label, value]) => (
              <div key={label}>
                <dt className="text-xs text-muted-foreground">{label}</dt>
                <dd className="font-semibold text-heading tabular-nums">{formatAmount(value)}</dd>
              </div>
            ))}
          </dl>
        </div>
      ))}
    </section>
  )
}

function EntryList({ entries }: { entries: Entry[] }) {
  return groupByDate(entries).map(([date, dayEntries]) => (
    <section key={date} aria-labelledby={`day-${date}`} className="mt-6">
      <h3 id={`day-${date}`} className="mb-2 text-sm font-semibold text-muted-foreground">
        {dayLabel(date)}
      </h3>
      <ul className="divide-y divide-border rounded-card border border-border bg-card shadow-card">
        {dayEntries.map((entry) => (
          <li key={entry.id}>
            <Link
              to={`/entries/${entry.id}`}
              className="flex items-center gap-3 px-5 py-3.5 transition-colors hover:bg-brand-soft focus-visible:outline-3 focus-visible:outline-offset-1 focus-visible:outline-ring"
            >
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="font-semibold text-heading">{entry.category.name}</span>
                {entry.note && <span className="truncate text-sm text-muted-foreground">{entry.note}</span>}
              </span>
              <span className="text-xs text-muted-foreground">{entry.currency}</span>
              <SignedAmount entry={entry} />
            </Link>
          </li>
        ))}
      </ul>
    </section>
  ))
}

function LedgerPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const requested = searchParams.get('month') ?? ''
  const month = isMonth(requested) ? requested : taipeiMonth()
  const query = useLedgerMonth(month)

  const newEntryLink = (
    <Link to="/entries/new" className={buttonClassName()}>
      記一筆
    </Link>
  )

  return (
    <main className="mx-auto max-w-170 px-5 pb-16">
      <div className="mt-12 flex items-center justify-between gap-3">
        <h1 className="text-3xl font-bold tracking-[-0.02em] text-heading">記帳</h1>
        {newEntryLink}
      </div>
      <MonthNav month={month} onChange={(next) => setSearchParams({ month: next })} />

      {query.isPending && <p className="mt-6 text-muted-foreground">載入中…</p>}
      {query.isError && (
        <div className="mt-6 flex flex-col items-start gap-3">
          <Notice tone="error">無法載入紀錄，請稍後再試。</Notice>
          <Button type="button" variant="quiet" onClick={() => query.refetch()}>
            重新載入
          </Button>
        </div>
      )}
      {query.isSuccess && query.data.entries.length === 0 && (
        <div className="mt-10 flex flex-col items-center gap-4 text-muted-foreground">
          <p>這個月還沒有紀錄</p>
          {newEntryLink}
        </div>
      )}
      {query.isSuccess && query.data.entries.length > 0 && (
        <>
          <Totals totals={query.data.totals} />
          <EntryList entries={query.data.entries} />
        </>
      )}
    </main>
  )
}

export default LedgerPage
