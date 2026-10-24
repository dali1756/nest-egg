import { useQuery } from '@tanstack/react-query'
import { getEntry, getMonth } from '../api/ledger'

export const ledgerEntriesKey = ['ledger-entries'] as const

export function useLedgerMonth(month: string) {
  return useQuery({ queryKey: [...ledgerEntriesKey, month], queryFn: () => getMonth(month) })
}

export function useLedgerEntry(id: number | undefined) {
  return useQuery({
    queryKey: [...ledgerEntriesKey, 'entry', id],
    queryFn: () => getEntry(id!),
    enabled: id !== undefined,
  })
}
