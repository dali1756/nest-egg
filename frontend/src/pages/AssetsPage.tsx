import type { CashBalance } from '../api/portfolio'
import { TWD } from '../api/currencies'
import Button from '../components/Button'
import LoadingMessage from '../components/LoadingMessage'
import Notice from '../components/Notice'
import RateSource from '../components/RateSource'
import TextLink from '../components/TextLink'
import { monthDay } from '../format/date'
import { formatAmount, formatSignedAmount, formatSignedPercent, pnlDirection, type PnlDirection } from '../format/money'
import { usePortfolioSummary } from '../hooks/usePortfolio'

function CashDetails({ cash }: { cash: CashBalance }) {
  const details = [
    formatAmount(cash.balance),
    cash.average_cost && `均價 ${cash.average_cost}`,
    cash.market_rate && cash.market_rate_date && `匯率 ${cash.market_rate}（${monthDay(cash.market_rate_date)}）`,
  ].filter(Boolean)
  return (
    <span className="text-sm text-muted-foreground tabular-nums">
      {details.map((text, i) => (
        <span key={i}>
          {i > 0 && ' · '}
          <span>{text}</span>
        </span>
      ))}
    </span>
  )
}

const pnlColours: Record<PnlDirection, string> = { gain: 'text-gain', loss: 'text-loss', flat: 'text-foreground' }

function UnrealizedPnL({ cash }: { cash: CashBalance }) {
  if (cash.unrealized_pnl === null || cash.unrealized_pnl_percent === null) return null
  return (
    <span className={`text-sm font-semibold tabular-nums ${pnlColours[pnlDirection(cash.unrealized_pnl)]}`}>
      {`${formatSignedAmount(cash.unrealized_pnl)}（${formatSignedPercent(cash.unrealized_pnl_percent)}）`}
    </span>
  )
}

const unavailableMessages = {
  negative_balance: '餘額是負數，均價與損益算不出來',
  missing_latest_rate: '查不到最近的市場參考匯率，市值與損益算不出來',
}

function UnavailableNote({ cash }: { cash: CashBalance }) {
  const entry = cash.missing_rate_entry
  if (cash.unavailable_reason === 'missing_income_rate' && entry) {
    return (
      <span className="max-w-60 text-xs text-muted-foreground">
        {`${entry.occurred_on} 的收入查不到市場參考匯率，均價與損益算不出來`}
        <TextLink to={`/entries/${entry.id}`}>查看這筆</TextLink>
      </span>
    )
  }
  if (cash.unavailable_reason === 'negative_balance' || cash.unavailable_reason === 'missing_latest_rate') {
    return <span className="max-w-60 text-xs text-muted-foreground">{unavailableMessages[cash.unavailable_reason]}</span>
  }
  return null
}

function CashItem({ cash }: { cash: CashBalance }) {
  const foreign = cash.currency !== TWD
  return (
    <li className="flex items-center gap-3 px-5 py-3.5">
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="font-semibold text-heading">{`${cash.currency} ${cash.name}`}</span>
        {foreign && <CashDetails cash={cash} />}
      </span>
      <span className="flex flex-col items-end text-right">
        <span className="font-semibold text-heading tabular-nums">{cash.twd_value === null ? '—' : formatAmount(cash.twd_value)}</span>
        {foreign && (cash.unavailable_reason ? <UnavailableNote cash={cash} /> : <UnrealizedPnL cash={cash} />)}
      </span>
    </li>
  )
}

const openingBalancesPath = '/assets/opening-balances'

function RateFootnote({ cash }: { cash: CashBalance[] }) {
  if (!cash.some((c) => c.market_rate_date !== null)) return null
  return <RateSource className="mt-6" />
}

function CashSection({ cash }: { cash: CashBalance[] }) {
  return (
    <>
      <section aria-labelledby="cash-heading" className="mt-6">
        <h2 id="cash-heading" className="mb-2 text-sm font-semibold text-muted-foreground">
          現金
        </h2>
        <ul className="divide-y divide-border rounded-card border border-border bg-card shadow-card">
          {cash.map((c) => (
            <CashItem key={c.currency} cash={c} />
          ))}
        </ul>
        <p className="mt-3 text-right text-sm">
          <TextLink to={openingBalancesPath}>期初餘額 ›</TextLink>
        </p>
      </section>
      <RateFootnote cash={cash} />
    </>
  )
}

function AssetsPage() {
  const query = usePortfolioSummary()

  return (
    <main className="mx-auto max-w-170 px-5 pb-16">
      <h1 className="mt-12 text-3xl font-bold tracking-[-0.02em] text-heading">資產</h1>
      {query.isPending && <LoadingMessage />}
      {query.isError && (
        <div className="mt-6 flex flex-col items-start gap-3">
          <Notice tone="error">無法載入資產，請稍後再試。</Notice>
          <Button type="button" variant="quiet" onClick={() => query.refetch()}>
            重新載入
          </Button>
        </div>
      )}
      {query.isSuccess &&
        (query.data.cash.length > 0 ? (
          <CashSection cash={query.data.cash} />
        ) : (
          <div className="mt-10 flex flex-col items-center gap-2 text-muted-foreground">
            <p>還沒有任何紀錄</p>
            <p className="text-sm">
              開始用 nest-egg 前已經有的錢，可以先
              <TextLink to={openingBalancesPath}>設定期初餘額</TextLink>
            </p>
          </div>
        ))}
    </main>
  )
}

export default AssetsPage
