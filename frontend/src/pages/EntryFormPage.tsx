import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { z } from 'zod'
import { ApiError } from '../api/http'
import {
  createEntry,
  deleteEntry,
  getCategories,
  getCurrencies,
  updateEntry,
  type Category,
  type Currency,
  type Entry,
  type EntryInput,
  type Kind,
} from '../api/ledger'
import Button from '../components/Button'
import ChoiceGroup from '../components/ChoiceGroup'
import Field from '../components/Field'
import Notice from '../components/Notice'
import PageMessage from '../components/PageMessage'
import SelectField from '../components/SelectField'
import TextLink from '../components/TextLink'
import { pickableDate, taipeiToday } from '../format/date'
import { decimalPlaces } from '../format/money'
import { clearUserIfUnauthorized } from '../hooks/useCurrentUser'
import { ledgerEntriesKey, useLedgerEntry } from '../hooks/useLedger'

const kinds: { value: Kind; label: string }[] = [
  { value: 'expense', label: '支出' },
  { value: 'income', label: '收入' },
]

const baseCurrency = 'TWD'

const noCategory = 0

const amountPattern = /^[0-9]{1,16}(\.[0-9]+)?$/

function entrySchema(today: string, currencies: Currency[]) {
  return z
    .object({
      kind: z.enum(['expense', 'income']),
      occurred_on: z.string().min(1, '請選擇日期').refine((date) => date <= today, '日期不能晚於今天'),
      category_id: z.int().positive('請選擇分類'),
      currency: z.string().min(1, '請選擇幣別'),
      amount: z
        .string()
        .min(1, '請輸入金額')
        .regex(amountPattern, '請輸入數字，例如 120 或 32.5')
        .refine((amount) => /[1-9]/.test(amount), '請輸入大於 0 的金額'),
      note: z.string().max(200, '備註最多 200 字'),
    })
    .superRefine((values, ctx) => {
      const minorUnit = currencies.find((c) => c.code === values.currency)?.minor_unit
      if (minorUnit !== undefined && decimalPlaces(values.amount) > minorUnit) {
        ctx.addIssue({ code: 'custom', path: ['amount'], message: `金額最多 ${minorUnit} 位小數` })
      }
    })
}

type EntryValues = z.infer<ReturnType<typeof entrySchema>>

function defaultValuesFor(entry: Entry | undefined, pickedDate: string | undefined): EntryValues {
  if (!entry) {
    const occurred_on = pickedDate ?? taipeiToday()
    return { kind: 'expense', occurred_on, category_id: noCategory, currency: baseCurrency, amount: '', note: '' }
  }
  return {
    kind: entry.kind,
    occurred_on: entry.occurred_on,
    category_id: entry.category.id,
    currency: entry.currency,
    amount: entry.amount,
    note: entry.note,
  }
}

function toEntryInput({ occurred_on, category_id, currency, amount, note }: EntryValues): EntryInput {
  return { occurred_on, category_id, currency, amount, note }
}

const serverFieldMessages: Record<keyof EntryInput, string> = {
  occurred_on: '日期不正確，請重新選擇',
  category_id: '找不到這個分類，請重新選擇',
  currency: '不支援這個幣別',
  amount: '金額格式不正確',
  note: '備註最多 200 字',
}

function fieldErrorsFrom(err: Error) {
  return err instanceof ApiError && err.status === 422 ? err.fields : undefined
}

function rejectedFields(err: Error): [keyof EntryInput, string][] {
  const fields = fieldErrorsFrom(err) ?? {}
  const names = Object.keys(serverFieldMessages) as (keyof EntryInput)[]
  return names.filter((name) => fields[name]).map((name) => [name, serverFieldMessages[name]])
}

function MarketRateNote({ entry }: { entry: Entry }) {
  if (entry.currency === baseCurrency) return null
  if (entry.market_rate === null) return <p className="text-sm text-muted-foreground">查不到這天的市場參考匯率</p>
  return (
    <div className="text-sm text-muted-foreground">
      <p>
        市場參考匯率 {entry.market_rate}（{entry.market_rate_date}）
      </p>
      <p>資料來源：中央銀行</p>
    </div>
  )
}

function DeleteEntryButton({ entry, onDeleted }: { entry: Entry; onDeleted: () => void }) {
  const queryClient = useQueryClient()
  const [confirming, setConfirming] = useState(false)
  const deletion = useMutation({
    mutationFn: () => deleteEntry(entry.id),
    onSuccess: onDeleted,
    onError: (err) => clearUserIfUnauthorized(queryClient, err),
  })

  if (!confirming) {
    return (
      <Button type="button" variant="quiet" onClick={() => setConfirming(true)}>
        刪除
      </Button>
    )
  }
  return (
    <div className="flex flex-col gap-3 rounded-control border border-destructive-soft bg-destructive-soft/40 p-4">
      <p className="text-sm font-semibold text-heading">確定要刪除這筆紀錄嗎？</p>
      {deletion.isError && <Notice tone="error">刪除失敗，請稍後再試</Notice>}
      <div className="flex gap-2">
        <Button type="button" variant="danger" disabled={deletion.isPending} onClick={() => deletion.mutate()}>
          確定刪除
        </Button>
        <Button type="button" variant="quiet" onClick={() => setConfirming(false)}>
          取消
        </Button>
      </div>
    </div>
  )
}

function ledgerPath(occurredOn: string, openedFromPickedDay: boolean): string {
  return openedFromPickedDay ? `/?date=${occurredOn}` : `/?month=${occurredOn.slice(0, 7)}`
}

type EntryFormProps = { categories: Category[]; currencies: Currency[]; entry?: Entry; pickedDate?: string }

function EntryForm({ categories, currencies, entry, pickedDate }: EntryFormProps) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const refreshAndShowLedger = (occurredOn: string) => {
    navigate(ledgerPath(occurredOn, pickedDate !== undefined))
    void queryClient.invalidateQueries({ queryKey: ledgerEntriesKey })
  }
  const { register, handleSubmit, control, setValue, setError, formState } = useForm<EntryValues>({
    resolver: zodResolver(entrySchema(taipeiToday(), currencies)),
    defaultValues: defaultValuesFor(entry, pickedDate),
  })
  const kind = useWatch({ control, name: 'kind' })
  const mutation = useMutation({
    mutationFn: (input: EntryInput) => (entry ? updateEntry(entry.id, input) : createEntry(input)),
    onSuccess: (saved) => refreshAndShowLedger(saved.occurred_on),
    onError: (err) => {
      clearUserIfUnauthorized(queryClient, err)
      for (const [field, message] of rejectedFields(err)) setError(field, { message })
    },
  })
  const { errors } = formState
  const showGeneralError = mutation.isError && !fieldErrorsFrom(mutation.error)

  return (
    <form
      className="mt-7 flex flex-col gap-5"
      noValidate
      onSubmit={handleSubmit((values) => mutation.mutate(toEntryInput(values)))}
    >
      {showGeneralError && <Notice tone="error">儲存失敗，請稍後再試</Notice>}
      <Controller
        control={control}
        name="kind"
        render={({ field }) => (
          <ChoiceGroup
            legend="種類"
            name="kind"
            options={kinds}
            value={field.value}
            onChange={(next) => {
              field.onChange(next)
              setValue('category_id', noCategory)
            }}
          />
        )}
      />
      <Controller
        control={control}
        name="category_id"
        render={({ field }) => (
          <ChoiceGroup
            legend="分類"
            name="category"
            options={categories.filter((c) => c.kind === kind).map((c) => ({ value: c.id, label: c.name }))}
            value={field.value}
            onChange={field.onChange}
            error={errors.category_id?.message}
          />
        )}
      />
      <Field
        id="entry-date"
        label="日期"
        type="date"
        max={taipeiToday()}
        error={errors.occurred_on?.message}
        {...register('occurred_on')}
      />
      <SelectField id="entry-currency" label="幣別" error={errors.currency?.message} {...register('currency')}>
        {currencies.map((c) => (
          <option key={c.code} value={c.code}>
            {c.code} {c.name}
          </option>
        ))}
      </SelectField>
      <Field
        id="entry-amount"
        label="金額"
        inputMode="decimal"
        autoComplete="off"
        error={errors.amount?.message}
        {...register('amount')}
      />
      <Field id="entry-note" label="備註" autoComplete="off" error={errors.note?.message} {...register('note')} />
      {entry && <MarketRateNote entry={entry} />}
      <Button type="submit" block disabled={mutation.isPending}>
        {mutation.isPending ? '儲存中…' : '儲存'}
      </Button>
      {entry && <DeleteEntryButton entry={entry} onDeleted={() => refreshAndShowLedger(entry.occurred_on)} />}
    </form>
  )
}

function NotFound() {
  return (
    <div className="mx-auto mt-12 flex max-w-170 flex-col items-start gap-2 px-5 text-muted-foreground">
      <p>找不到這筆紀錄</p>
      <TextLink to="/">回記帳頁</TextLink>
    </div>
  )
}

function parseEntryId(id: string | undefined): number | undefined {
  return id !== undefined && /^\d+$/.test(id) ? Number.parseInt(id, 10) : undefined
}

function EntryFormPage() {
  const { id } = useParams()
  const [searchParams] = useSearchParams()
  const editing = id !== undefined
  const pickedDate = editing ? undefined : pickableDate(searchParams.get('date'), taipeiToday())
  const entryId = parseEntryId(id)
  const categories = useQuery({ queryKey: ['categories'], queryFn: getCategories })
  const currencies = useQuery({ queryKey: ['currencies'], queryFn: getCurrencies })
  const entry = useLedgerEntry(entryId)

  const missing = entry.error instanceof ApiError && entry.error.status === 404
  if (editing && (entryId === undefined || missing)) return <NotFound />
  if (categories.isError || currencies.isError || entry.isError) {
    return <PageMessage>無法載入表單，請稍後重新整理。</PageMessage>
  }
  const ready = categories.isSuccess && currencies.isSuccess && (!editing || entry.isSuccess)
  return (
    <main className="mx-auto max-w-120 px-5 pb-16">
      <h1 className="mt-12 text-3xl font-bold tracking-[-0.02em] text-heading">{editing ? '修改紀錄' : '記一筆'}</h1>
      {ready ? (
        <EntryForm categories={categories.data} currencies={currencies.data} entry={entry.data} pickedDate={pickedDate} />
      ) : (
        <p className="mt-6 text-muted-foreground">載入中…</p>
      )}
    </main>
  )
}

export default EntryFormPage
