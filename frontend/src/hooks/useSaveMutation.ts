import { useMutation, useQueryClient } from '@tanstack/react-query'
import { fieldErrorsFrom, rejectedFields } from '../api/fieldErrors'
import { clearUserIfUnauthorized } from './useCurrentUser'

type SaveMutationOptions<Input, Saved, Field extends string> = {
  mutationFn: (input: Input) => Promise<Saved>
  onSuccess: (saved: Saved) => void
  serverFieldMessages: Record<Field, string>
  setError: (field: Field, error: { message: string }) => void
}

// 後端退回的欄位錯誤顯示在欄位旁，其他錯誤才顯示一般的錯誤訊息
export function useSaveMutation<Input, Saved, Field extends string>({
  mutationFn,
  onSuccess,
  serverFieldMessages,
  setError,
}: SaveMutationOptions<Input, Saved, Field>) {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn,
    onSuccess,
    onError: (err) => {
      clearUserIfUnauthorized(queryClient, err)
      for (const [field, message] of rejectedFields(err, serverFieldMessages)) setError(field, { message })
    },
  })
  const showGeneralError = mutation.isError && !fieldErrorsFrom(mutation.error)
  return { mutation, showGeneralError }
}
