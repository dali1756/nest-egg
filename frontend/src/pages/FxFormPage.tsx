import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { useNavigate, useParams } from 'react-router'
import { z } from 'zod'
import { getCurrencies, TWD, type Currency } from '../api/currencies'
import { fieldErrorsFrom, rejectedFields } from '../api/fieldErrors'
import { createFxTrade, deleteFxTrade, updateFxTrade, type FxRates, type FxTrade, type FxTradeInput, type Side } from '../api/fx'
import { ApiError } from '../api/http'
import Button from '../components/Button'
import ChoiceGroup from '../components/ChoiceGroup'
import ConfirmDeleteButton from '../components/ConfirmDeleteButton'
import Field from '../components/Field'
import Notice from '../components/Notice'
import PageMessage from '../components/PageMessage'
import SelectField from '../components/SelectField'
import TextLink from '../components/TextLink'
import { taipeiToday } from '../format/date'
import { spreadText } from '../format/fx'
import { parsePathId } from '../format/id'
import { clearUserIfUnauthorized } from '../hooks/useCurrentUser'
import { fxTradesKey, useFxPreview, useFxTrade, type FxPreviewInput } from '../hooks/useFx'
import { amountField, decimalPlacesProblem } from '../validation/amount'
import { noteField, pastDateField } from '../validation/fields'

const sides: { value: Side; label: string }[] = [
  { value: 'buy', label: '台幣換外幣' },
  { value: 'sell', label: '外幣換台幣' },
]

// 欄位標題跟著方向換，和銀行明細上「換到」「付出」的說法一致
const amountLabels: Record<Side, { foreign: string; twd: string }> = {
  buy: { foreign: '換到的外幣', twd: '付出的台幣' },
  sell: { foreign: '付出的外幣', twd: '換到的台幣' },
}

function foreignCurrencies(currencies: Currency[]): Currency[] {
  return currencies.filter((c) => c.code !== TWD)
}

function minorUnitOf(currencies: Currency[], code: string): number | undefined {
  return currencies.find((c) => c.code === code)?.minor_unit
}

function tradeSchema(today: string, currencies: Currency[]) {
  return z
    .object({
      side: z.enum(['buy', 'sell']),
      traded_on: pastDateField(today),
      currency: z.string().min(1, '請選擇幣別'),
      foreign_amount: amountField(),
      twd_amount: amountField(),
      note: noteField(),
    })
    .superRefine((values, ctx) => {
      const foreignProblem = decimalPlacesProblem(values.foreign_amount, minorUnitOf(currencies, values.currency))
      if (foreignProblem) ctx.addIssue({ code: 'custom', path: ['foreign_amount'], message: foreignProblem })
      const twdProblem = decimalPlacesProblem(values.twd_amount, minorUnitOf(currencies, TWD))
      if (twdProblem) ctx.addIssue({ code: 'custom', path: ['twd_amount'], message: twdProblem })
    })
}

type TradeValues = z.infer<ReturnType<typeof tradeSchema>>

function defaultValuesFor(trade: FxTrade | undefined, currencies: Currency[]): TradeValues {
  if (!trade) {
    const currency = foreignCurrencies(currencies)[0]?.code ?? ''
    return { side: 'buy', traded_on: taipeiToday(), currency, foreign_amount: '', twd_amount: '', note: '' }
  }
  const { side, traded_on, currency, foreign_amount, twd_amount, note } = trade
  return { side, traded_on, currency, foreign_amount, twd_amount, note }
}

const serverFieldMessages: Record<keyof FxTradeInput, string> = {
  traded_on: '日期不正確，請重新選擇',
  side: '請選擇換匯方向',
  currency: '不支援這個幣別',
  foreign_amount: '金額格式不正確',
  twd_amount: '金額格式不正確',
  note: '備註最多 200 字',
}

// 試算只看方向、日期、幣別與兩個金額；備註不合格不影響試算
function previewInputFrom(values: TradeValues, schema: ReturnType<typeof tradeSchema>): FxPreviewInput | undefined {
  const parsed = schema.safeParse({ ...values, note: '' })
  if (!parsed.success) return undefined
  const { side, traded_on, currency, foreign_amount, twd_amount } = parsed.data
  return { side, traded_on, currency, foreign_amount, twd_amount }
}

function RatesPreview({ side, rates }: { side: Side; rates: FxRates }) {
  return (
    <section aria-label="試算" className="flex flex-col gap-1 rounded-control bg-brand-soft px-4 py-3 text-sm text-heading">
      <p>成交匯率 {rates.trade_rate}</p>
      {rates.market_rate === null ? (
        <p className="text-muted-foreground">查不到市場參考匯率</p>
      ) : (
        <>
          <p>
            市場參考匯率 {rates.market_rate}（{rates.market_rate_date}）
          </p>
          <p className="font-semibold">{spreadText(side, rates.spread)}</p>
          <p className="text-xs text-muted-foreground">資料來源：中央銀行</p>
        </>
      )}
    </section>
  )
}

function fxMonthPath(tradedOn: string): string {
  return `/fx?month=${tradedOn.slice(0, 7)}`
}

type FxFormProps = { currencies: Currency[]; trade?: FxTrade }

function FxForm({ currencies, trade }: FxFormProps) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const refreshAndShowMonth = (tradedOn: string) => {
    navigate(fxMonthPath(tradedOn))
    void queryClient.invalidateQueries({ queryKey: fxTradesKey })
  }
  const schema = tradeSchema(taipeiToday(), currencies)
  const { register, handleSubmit, control, setError, formState } = useForm<TradeValues>({
    resolver: zodResolver(schema),
    defaultValues: defaultValuesFor(trade, currencies),
  })
  const values = useWatch({ control }) as TradeValues
  const rates = useFxPreview(previewInputFrom(values, schema), trade?.id)
  const labels = amountLabels[values.side]
  const mutation = useMutation({
    mutationFn: (input: FxTradeInput) => (trade ? updateFxTrade(trade.id, input) : createFxTrade(input)),
    onSuccess: (saved) => refreshAndShowMonth(saved.traded_on),
    onError: (err) => {
      clearUserIfUnauthorized(queryClient, err)
      for (const [field, message] of rejectedFields(err, serverFieldMessages)) setError(field, { message })
    },
  })
  const { errors } = formState
  const showGeneralError = mutation.isError && !fieldErrorsFrom(mutation.error)

  return (
    <form className="mt-7 flex flex-col gap-5" noValidate onSubmit={handleSubmit((input) => mutation.mutate(input))}>
      {showGeneralError && <Notice tone="error">儲存失敗，請稍後再試</Notice>}
      <Controller
        control={control}
        name="side"
        render={({ field }) => (
          <ChoiceGroup legend="方向" name="side" options={sides} value={field.value} onChange={field.onChange} error={errors.side?.message} />
        )}
      />
      <Field id="fx-date" label="日期" type="date" max={taipeiToday()} error={errors.traded_on?.message} {...register('traded_on')} />
      <SelectField id="fx-currency" label="幣別" error={errors.currency?.message} {...register('currency')}>
        {foreignCurrencies(currencies).map((c) => (
          <option key={c.code} value={c.code}>
            {c.code} {c.name}
          </option>
        ))}
      </SelectField>
      <Field
        id="fx-foreign-amount"
        label={labels.foreign}
        inputMode="decimal"
        autoComplete="off"
        error={errors.foreign_amount?.message}
        {...register('foreign_amount')}
      />
      <Field
        id="fx-twd-amount"
        label={labels.twd}
        inputMode="decimal"
        autoComplete="off"
        error={errors.twd_amount?.message}
        {...register('twd_amount')}
      />
      <Field id="fx-note" label="備註" autoComplete="off" error={errors.note?.message} {...register('note')} />
      {rates && <RatesPreview side={values.side} rates={rates} />}
      <Button type="submit" block disabled={mutation.isPending}>
        {mutation.isPending ? '儲存中…' : '儲存'}
      </Button>
      {trade && (
        <ConfirmDeleteButton
          question="確定要刪除這筆換匯嗎？"
          onDelete={() => deleteFxTrade(trade.id)}
          onDeleted={() => refreshAndShowMonth(trade.traded_on)}
        />
      )}
    </form>
  )
}

function NotFound() {
  return (
    <div className="mx-auto mt-12 flex max-w-170 flex-col items-start gap-2 px-5 text-muted-foreground">
      <p>找不到這筆換匯</p>
      <TextLink to="/fx">回換匯頁</TextLink>
    </div>
  )
}

function FxFormPage() {
  const { id } = useParams()
  const editing = id !== undefined
  const tradeId = parsePathId(id)
  const currencies = useQuery({ queryKey: ['currencies'], queryFn: getCurrencies })
  const trade = useFxTrade(tradeId)

  const missing = trade.error instanceof ApiError && trade.error.status === 404
  if (editing && (tradeId === undefined || missing)) return <NotFound />
  if (currencies.isError || trade.isError) {
    return <PageMessage>無法載入表單，請稍後重新整理。</PageMessage>
  }
  const ready = currencies.isSuccess && (!editing || trade.isSuccess)
  return (
    <main className="mx-auto max-w-120 px-5 pb-16">
      <h1 className="mt-12 text-3xl font-bold tracking-[-0.02em] text-heading">{editing ? '修改換匯' : '＋'}</h1>
      {ready ? <FxForm currencies={currencies.data} trade={trade.data} /> : <p className="mt-6 text-muted-foreground">載入中…</p>}
    </main>
  )
}

export default FxFormPage
