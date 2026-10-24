import { useQuery } from '@tanstack/react-query'
import { getMonth } from '../api/ledger'

export const ledgerEntriesKey = ['ledger-entries'] as const

export function useLedgerMonth(month: string) {
  return useQuery({ queryKey: [...ledgerEntriesKey, month], queryFn: () => getMonth(month) })
}
