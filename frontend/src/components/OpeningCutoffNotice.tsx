import { openingCutoffs, openingCutoffText } from '../format/opening'
import { useOpeningBalances } from '../hooks/usePortfolio'

function OpeningCutoffNotice({ date, currencies }: { date: string; currencies: string[] }) {
  const openings = useOpeningBalances()
  const cutoffs = openingCutoffs(openings.data ?? [], date, currencies)
  if (cutoffs.length === 0) return null
  return (
    <div className="flex flex-col gap-1 rounded-control bg-destructive-soft px-3.5 py-2.5 text-sm text-destructive-text">
      {cutoffs.map((o) => (
        <p key={o.currency}>{openingCutoffText(o)}</p>
      ))}
    </div>
  )
}

export default OpeningCutoffNotice
