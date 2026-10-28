import type { ReactNode } from 'react'
import { Link, useSearchParams } from 'react-router'
import type { Entry, MonthLedger, Total } from '../api/ledger'
import Button from '../components/Button'
import { buttonClassName } from '../components/buttonStyles'
import MonthCalendar from '../components/MonthCalendar'
import MonthNav from '../components/MonthNav'
import Notice from '../components/Notice'
import { dayLabel, groupByDay, monthOrCurrent, pickableDate, taipeiToday } from '../format/date'
import { formatAmount } from '../format/money'
import { useLedgerMonth } from '../hooks/useLedger'

function SignedAmount({ entry }: { entry: Entry }) {
  const expense = entry.kind === 'expense'
  return (
    <span className={`font-semibold tabular-nums ${expense ? 'text-destructive-text' : 'text-success-text'}`}>
      {expense ? '−' : '+'}
      {formatAmount(entry.amount)}
    </span>
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

const dayHeadingClassName = 'text-sm font-semibold text-muted-foreground'

function EntryItems({ entries }: { entries: Entry[] }) {
  return (
    <ul className="divide-y divide-border rounded-card border border-border bg-card shadow-card">
      {entries.map((entry) => (
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
  )
}

function EntryList({ entries }: { entries: Entry[] }) {
  return groupByDay(entries, (entry) => entry.occurred_on).map(([date, dayEntries]) => (
    <section key={date} aria-labelledby={`day-${date}`} className="mt-6">
      <h3 id={`day-${date}`} className={`mb-2 ${dayHeadingClassName}`}>
        {dayLabel(date)}
      </h3>
      <EntryItems entries={dayEntries} />
    </section>
  ))
}

type SelectedDayProps = { date: string; entries: Entry[]; onShowMonth: () => void; newEntryLink: ReactNode }

function SelectedDay({ date, entries, onShowMonth, newEntryLink }: SelectedDayProps) {
  return (
    <section aria-labelledby="selected-day" className="mt-6">
      <div className="mb-2 flex items-center justify-between gap-3">
        <h3 id="selected-day" className={dayHeadingClassName}>
          {dayLabel(date)}
        </h3>
        <Button type="button" variant="quiet" onClick={onShowMonth}>
          顯示整個月
        </Button>
      </div>
      {entries.length > 0 ? (
        <EntryItems entries={entries} />
      ) : (
        <div className="mt-6 flex flex-col items-center gap-4 text-muted-foreground">
          <p>這天還沒有紀錄</p>
          {newEntryLink}
        </div>
      )}
    </section>
  )
}

type MonthEntriesProps = { data: MonthLedger; selectedDate?: string; onShowMonth: () => void; newEntryLink: ReactNode }

function MonthEntries({ data, selectedDate, onShowMonth, newEntryLink }: MonthEntriesProps) {
  const hasEntries = data.entries.length > 0
  if (!hasEntries && !selectedDate) {
    return (
      <div className="mt-10 flex flex-col items-center gap-4 text-muted-foreground">
        <p>這個月還沒有紀錄</p>
        {newEntryLink}
      </div>
    )
  }
  return (
    <>
      {hasEntries && <Totals totals={data.totals} />}
      {selectedDate ? (
        <SelectedDay
          date={selectedDate}
          entries={data.entries.filter((entry) => entry.occurred_on === selectedDate)}
          onShowMonth={onShowMonth}
          newEntryLink={newEntryLink}
        />
      ) : (
        <EntryList entries={data.entries} />
      )}
    </>
  )
}

type LedgerView = { month: string; selectedDate?: string }

function viewFrom(searchParams: URLSearchParams, today: string): LedgerView {
  const date = pickableDate(searchParams.get('date'), today)
  if (date) return { month: date.slice(0, 7), selectedDate: date }
  return { month: monthOrCurrent(searchParams.get('month')) }
}

function LedgerPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const today = taipeiToday()
  const { month, selectedDate } = viewFrom(searchParams, today)
  const query = useLedgerMonth(month)
  const goToMonth = (next: string) => setSearchParams({ month: next })
  const showMonth = () => goToMonth(month)
  const toggleDate = (date: string) => (date === selectedDate ? showMonth() : setSearchParams({ date }))

  const newEntryLink = (
    <Link to={selectedDate ? `/entries/new?date=${selectedDate}` : '/entries/new'} className={buttonClassName()}>
      記一筆
    </Link>
  )

  return (
    <main className="mx-auto max-w-170 px-5 pb-16">
      <div className="mt-12 flex items-center justify-between gap-3">
        <h1 className="text-3xl font-bold tracking-[-0.02em] text-heading">記帳</h1>
        {newEntryLink}
      </div>
      <MonthNav month={month} onChange={goToMonth} />
      <MonthCalendar
        month={month}
        today={today}
        daysWithEntries={new Set(query.data?.entries.map((entry) => entry.occurred_on))}
        selectedDate={selectedDate}
        onSelect={toggleDate}
        onMonthChange={goToMonth}
      />

      {query.isPending && <p className="mt-6 text-muted-foreground">載入中…</p>}
      {query.isError && (
        <div className="mt-6 flex flex-col items-start gap-3">
          <Notice tone="error">無法載入紀錄，請稍後再試。</Notice>
          <Button type="button" variant="quiet" onClick={() => query.refetch()}>
            重新載入
          </Button>
        </div>
      )}
      {query.isSuccess && (
        <MonthEntries data={query.data} selectedDate={selectedDate} onShowMonth={showMonth} newEntryLink={newEntryLink} />
      )}
    </main>
  )
}

export default LedgerPage
