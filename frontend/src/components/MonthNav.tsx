import { monthLabel, shiftMonth, taipeiMonth } from '../format/date'

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

export default MonthNav
