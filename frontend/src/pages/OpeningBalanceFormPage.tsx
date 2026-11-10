import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { useForm, useWatch } from 'react-hook-form'
import { useNavigate, useParams } from 'react-router'
import { z } from 'zod'
import { TWD, type Currency } from '../api/currencies'
import { deleteOpeningBalance, putOpeningBalance, type OpeningBalance, type OpeningBalanceInput } from '../api/portfolio'
import ConfirmDeleteButton from '../components/ConfirmDeleteButton'
import Field from '../components/Field'
import LoadingMessage from '../components/LoadingMessage'
import NotFoundMessage from '../components/NotFoundMessage'
import Notice from '../components/Notice'
import PageMessage from '../components/PageMessage'
import RateSource from '../components/RateSource'
import SaveButton from '../components/SaveButton'
import TextLink from '../components/TextLink'
import { monthDay, taipeiToday } from '../format/date'
import { formatAmount, formatAmountWithPlaces } from '../format/money'
import { useCurrencies } from '../hooks/useCurrencies'
import { portfolioKey, useOpeningBalance, useOpeningBalancePreview, type OpeningPreview } from '../hooks/usePortfolio'
import { useSaveMutation } from '../hooks/useSaveMutation'
import { amountField, decimalPlacesProblem, optionalAmountField } from '../validation/amount'
import { pastDateField } from '../validation/fields'

const openingBalancesPath = '/assets/opening-balances'
const needsTWDCostMessage = '查不到這天的市場參考匯率，請填台幣成本'

function openingSchema(today: string, currency: Currency, twd: Currency | undefined) {
  return z
    .object({
      opened_on: pastDateField(today),
      amount: amountField(),
      twd_cost: optionalAmountField(),
    })
    .superRefine((values, ctx) => {
      const amountProblem = decimalPlacesProblem(values.amount, currency.minor_unit)
      if (amountProblem) ctx.addIssue({ code: 'custom', path: ['amount'], message: amountProblem })
      const costProblem = decimalPlacesProblem(values.twd_cost, twd?.minor_unit)
      if (costProblem) ctx.addIssue({ code: 'custom', path: ['twd_cost'], message: costProblem })
    })
}

type OpeningValues = z.infer<ReturnType<typeof openingSchema>>

function defaultValuesFor(opening: OpeningBalance | undefined): OpeningValues {
  if (!opening) return { opened_on: taipeiToday(), amount: '', twd_cost: '' }
  return { opened_on: opening.opened_on, amount: opening.amount, twd_cost: opening.twd_cost ?? '' }
}

const serverFieldMessages: Record<keyof OpeningBalanceInput, string> = {
  opened_on: '日期不正確，請重新選擇',
  amount: '金額格式不正確',
  twd_cost: needsTWDCostMessage,
}

function previewInputFrom(values: OpeningValues, schema: ReturnType<typeof openingSchema>): OpeningBalanceInput | undefined {
  const parsed = schema.safeParse(values)
  return parsed.success ? parsed.data : undefined
}

function CostPreview({ currency, values, preview }: { currency: Currency; values: OpeningValues; preview: OpeningPreview }) {
  const { cost, needsTWDCost } = preview
  if (needsTWDCost) {
    return <Notice tone="error">{needsTWDCostMessage}</Notice>
  }
  if (!cost) return null
  const { market_rate: rate, market_rate_date: rateDate, cost: total, average_cost: average } = cost
  const atMarketRate = values.twd_cost === '' && rate !== null && rateDate !== null && total !== null
  return (
    <section aria-label="試算" className="flex flex-col gap-1 rounded-control bg-brand-soft px-4 py-3 text-sm text-heading">
      {atMarketRate ? (
        <>
          <p>{`留空時用 ${monthDay(rateDate)} 的市場參考匯率換算：${formatAmountWithPlaces(values.amount, currency.minor_unit)} × ${rate} = ${formatAmount(total)}`}</p>
          <RateSource />
        </>
      ) : (
        average !== null && <p>{`均價 ${average}`}</p>
      )}
    </section>
  )
}

type OpeningFormProps = { currency: Currency; twd: Currency | undefined; opening: OpeningBalance | undefined }

function OpeningForm({ currency, twd, opening }: OpeningFormProps) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const refreshAndShowList = () => {
    navigate(openingBalancesPath)
    void queryClient.invalidateQueries({ queryKey: portfolioKey })
  }
  const foreign = currency.code !== TWD
  const schema = openingSchema(taipeiToday(), currency, twd)
  const { register, handleSubmit, control, setError, formState } = useForm<OpeningValues>({
    resolver: zodResolver(schema),
    defaultValues: defaultValuesFor(opening),
  })
  const values = useWatch({ control }) as OpeningValues
  const preview = useOpeningBalancePreview(currency.code, foreign ? previewInputFrom(values, schema) : undefined)
  const { mutation, showGeneralError } = useSaveMutation({
    mutationFn: (input: OpeningBalanceInput) => putOpeningBalance(currency.code, input),
    onSuccess: refreshAndShowList,
    serverFieldMessages,
    setError,
  })
  const { errors } = formState

  return (
    <form className="mt-7 flex flex-col gap-5" noValidate onSubmit={handleSubmit((input) => mutation.mutate(input))}>
      {showGeneralError && <Notice tone="error">儲存失敗，請稍後再試</Notice>}
      <Field
        id="opening-date"
        label="日期"
        type="date"
        max={taipeiToday()}
        hint="填這天結束時的餘額；這天（含）以前的收支與換匯不算進餘額"
        error={errors.opened_on?.message}
        {...register('opened_on')}
      />
      <Field
        id="opening-amount"
        label={`金額（${currency.code}）`}
        inputMode="decimal"
        autoComplete="off"
        error={errors.amount?.message}
        {...register('amount')}
      />
      {foreign && (
        <>
          <Field
            id="opening-twd-cost"
            label="台幣成本（選填）"
            inputMode="decimal"
            autoComplete="off"
            error={errors.twd_cost?.message}
            {...register('twd_cost')}
          />
          <CostPreview currency={currency} values={values} preview={preview} />
        </>
      )}
      <SaveButton pending={mutation.isPending} />
      {opening && (
        <ConfirmDeleteButton
          question={`確定要刪除 ${currency.code} 的期初餘額嗎？`}
          onDelete={() => deleteOpeningBalance(currency.code)}
          onDeleted={refreshAndShowList}
        />
      )}
    </form>
  )
}

function OpeningBalanceFormPage() {
  const { currency: code = '' } = useParams()
  const currencies = useCurrencies()
  const opening = useOpeningBalance(code)
  const currency = currencies.data?.find((c) => c.code === code)

  if (currencies.isSuccess && !currency) {
    return <NotFoundMessage message="找不到這個幣別" backTo={openingBalancesPath} backLabel="回期初餘額" />
  }
  if (currencies.isError || (opening.isError && !opening.isNotSet)) {
    return <PageMessage>無法載入表單，請稍後重新整理。</PageMessage>
  }
  const ready = currency !== undefined && (opening.isSuccess || opening.isNotSet)
  return (
    <main className="mx-auto max-w-120 px-5 pb-16">
      <p className="mt-8 text-sm">
        <TextLink to={openingBalancesPath} className="ml-0">
          ‹ 期初餘額
        </TextLink>
      </p>
      {ready ? (
        <>
          <h1 className="mt-4 text-3xl font-bold tracking-[-0.02em] text-heading">{`${currency.code} ${currency.name} 期初餘額`}</h1>
          <OpeningForm currency={currency} twd={currencies.data?.find((c) => c.code === TWD)} opening={opening.data} />
        </>
      ) : (
        <LoadingMessage />
      )}
    </main>
  )
}

export default OpeningBalanceFormPage
