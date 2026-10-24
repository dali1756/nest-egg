import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Controller, useForm, useWatch, type Resolver } from 'react-hook-form'
import { useNavigate, useParams } from 'react-router'
import { z } from 'zod'
import { ApiError } from '../api/http'
import {
  createEntry,
  deleteEntry,
  getCategories,
  getCurrencies,
  getEntry,
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
import { taipeiToday } from '../format/date'
import { currentUserKey } from '../hooks/useCurrentUser'
import { ledgerEntriesKey } from '../hooks/useLedgerMonth'

const kinds: { value: Kind; label: string }[] = [
  { value: 'expense', label: '支出' },
  { value: 'income', label: '收入' },
]

const baseCurrency = 'TWD'

const amountPattern = /^[0-9]{1,16}(\.[0-9]+)?$/

function decimalPlaces(amount: string) {
  const dot = amount.indexOf('.')
  return dot < 0 ? 0 : amount.length - dot - 1
}

function entrySchema(today: string, minorUnit?: number) {
  return z.object({
    kind: z.enum(['expense', 'income']),
    occurred_on: z.string().min(1, '請選擇日期').refine((date) => date <= today, '日期不能晚於今天'),
    category_id: z.int().positive('請選擇分類'),
    currency: z.string().min(1, '請選擇幣別'),
    amount: z
      .string()
      .min(1, '請輸入金額')
      .regex(amountPattern, '請輸入數字，例如 120 或 32.5')
      .refine((amount) => /[1-9]/.test(amount), '請輸入大於 0 的金額')
      .refine((amount) => minorUnit === undefined || decimalPlaces(amount) <= minorUnit, `金額最多 ${minorUnit} 位小數`),
    note: z.string().max(200, '備註最多 200 字'),
  })
}

type EntryValues = z.infer<ReturnType<typeof entrySchema>>

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

function DeleteEntry({ entry, onDeleted }: { entry: Entry; onDeleted: () => void }) {
  const queryClient = useQueryClient()
  const [confirming, setConfirming] = useState(false)
  const deletion = useMutation({
    mutationFn: () => deleteEntry(entry.id),
    onSuccess: onDeleted,
    onError: (err) => {
      if (err instanceof ApiError && err.status === 401) queryClient.setQueryData(currentUserKey, null)
    },
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

function EntryForm({ categories, currencies, entry }: { categories: Category[]; currencies: Currency[]; entry?: Entry }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const backToMonth = (occurredOn: string) => {
    navigate(`/?month=${occurredOn.slice(0, 7)}`)
    void queryClient.invalidateQueries({ queryKey: ledgerEntriesKey })
  }
  const resolver: Resolver<EntryValues> = (values, context, options) => {
    const minorUnit = currencies.find((c) => c.code === values.currency)?.minor_unit
    return (zodResolver(entrySchema(taipeiToday(), minorUnit)) as Resolver<EntryValues>)(values, context, options)
  }
  const { register, handleSubmit, control, setValue, setError, formState } = useForm<EntryValues>({
    resolver,
    defaultValues: entry
      ? {
          kind: entry.kind,
          occurred_on: entry.occurred_on,
          category_id: entry.category.id,
          currency: entry.currency,
          amount: entry.amount,
          note: entry.note,
        }
      : { kind: 'expense', occurred_on: taipeiToday(), category_id: 0, currency: baseCurrency, amount: '', note: '' },
  })
  const kind = useWatch({ control, name: 'kind' })
  const mutation = useMutation({
    mutationFn: (input: EntryInput) => (entry ? updateEntry(entry.id, input) : createEntry(input)),
    onSuccess: (saved) => backToMonth(saved.occurred_on),
    onError: (err) => {
      if (err instanceof ApiError && err.status === 401) queryClient.setQueryData(currentUserKey, null)
      const fields = fieldErrorsFrom(err) ?? {}
      for (const field of Object.keys(serverFieldMessages) as (keyof EntryInput)[]) {
        if (fields[field]) setError(field, { message: serverFieldMessages[field] })
      }
    },
  })
  const { errors } = formState
  const showGeneralError = mutation.isError && !fieldErrorsFrom(mutation.error)

  const save = ({ kind: _kind, ...input }: EntryValues) => mutation.mutate(input)

  return (
    <form className="mt-7 flex flex-col gap-5" noValidate onSubmit={handleSubmit(save)}>
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
              setValue('category_id', 0)
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
      {entry && <DeleteEntry entry={entry} onDeleted={() => backToMonth(entry.occurred_on)} />}
    </form>
  )
}

function NotFound() {
  return (
    <div className="mx-auto mt-12 flex max-w-170 flex-col items-start gap-2 px-5 text-muted-foreground">
      <p>找不到這筆紀錄</p>
      <TextLink to="/" className="ml-0">
        回記帳頁
      </TextLink>
    </div>
  )
}

function parseEntryId(id: string | undefined): number | undefined {
  return id !== undefined && /^\d+$/.test(id) ? Number.parseInt(id, 10) : undefined
}

function EntryFormPage() {
  const { id } = useParams()
  const editing = id !== undefined
  const entryId = parseEntryId(id)
  const categories = useQuery({ queryKey: ['categories'], queryFn: getCategories })
  const currencies = useQuery({ queryKey: ['currencies'], queryFn: getCurrencies })
  const entry = useQuery({
    queryKey: [...ledgerEntriesKey, 'entry', entryId],
    queryFn: () => getEntry(entryId!),
    enabled: entryId !== undefined,
  })

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
        <EntryForm categories={categories.data} currencies={currencies.data} entry={entry.data} />
      ) : (
        <p className="mt-6 text-muted-foreground">載入中…</p>
      )}
    </main>
  )
}

export default EntryFormPage
