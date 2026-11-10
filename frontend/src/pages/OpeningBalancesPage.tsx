import { Link } from 'react-router'
import type { Currency } from '../api/currencies'
import type { OpeningBalance } from '../api/portfolio'
import { listRowLinkClassName } from '../components/listStyles'
import LoadError from '../components/LoadError'
import LoadingMessage from '../components/LoadingMessage'
import TextLink from '../components/TextLink'
import { formatAmount } from '../format/money'
import { useCurrencies } from '../hooks/useCurrencies'
import { useOpeningBalances } from '../hooks/usePortfolio'

function OpeningAmount({ opening }: { opening: OpeningBalance }) {
  return (
    <span className="flex flex-col items-end text-right">
      <span className="font-semibold text-heading tabular-nums">{formatAmount(opening.amount)}</span>
      {opening.cost !== null && opening.average_cost !== null && (
        <span className="text-xs text-muted-foreground tabular-nums">{`成本 ${formatAmount(opening.cost)}（均價 ${opening.average_cost}）`}</span>
      )}
    </span>
  )
}

function NotSet() {
  return (
    <span className="flex flex-col items-end text-right">
      <span className="text-muted-foreground">未設定</span>
      <span className="text-xs font-semibold text-primary">設定 ›</span>
    </span>
  )
}

function OpeningRow({ currency, opening }: { currency: Currency; opening: OpeningBalance | undefined }) {
  return (
    <li>
      <Link
        to={`/assets/opening-balances/${currency.code}`}
        className={listRowLinkClassName}
      >
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="font-semibold text-heading">{`${currency.code} ${currency.name}`}</span>
          {opening && <span className="text-sm text-muted-foreground tabular-nums">{opening.opened_on}</span>}
        </span>
        {opening ? <OpeningAmount opening={opening} /> : <NotSet />}
      </Link>
    </li>
  )
}

function OpeningBalancesPage() {
  const currencies = useCurrencies()
  const openings = useOpeningBalances()

  return (
    <main className="mx-auto max-w-170 px-5 pb-16">
      <p className="mt-8 text-sm">
        <TextLink to="/assets" className="ml-0">
          ‹ 資產
        </TextLink>
      </p>
      <h1 className="mt-4 text-3xl font-bold tracking-[-0.02em] text-heading">期初餘額</h1>
      <p className="mt-2 text-muted-foreground">開始用 nest-egg 前手上已經有的錢，每個幣別一筆。</p>

      {(currencies.isPending || openings.isPending) && <LoadingMessage />}
      {(currencies.isError || openings.isError) && (
        <LoadError
          message="無法載入期初餘額，請稍後再試。"
          onRetry={() => {
            void currencies.refetch()
            void openings.refetch()
          }}
        />
      )}
      {currencies.isSuccess && openings.isSuccess && (
        <ul className="mt-6 divide-y divide-border rounded-card border border-border bg-card shadow-card">
          {currencies.data.map((c) => (
            <OpeningRow key={c.code} currency={c} opening={openings.data.find((o) => o.currency === c.code)} />
          ))}
        </ul>
      )}
    </main>
  )
}

export default OpeningBalancesPage
