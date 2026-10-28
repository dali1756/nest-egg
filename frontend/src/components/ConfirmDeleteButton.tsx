import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { clearUserIfUnauthorized } from '../hooks/useCurrentUser'
import Button from './Button'
import Notice from './Notice'

type ConfirmDeleteButtonProps = { question: string; onDelete: () => Promise<unknown>; onDeleted: () => void }

function ConfirmDeleteButton({ question, onDelete, onDeleted }: ConfirmDeleteButtonProps) {
  const queryClient = useQueryClient()
  const [confirming, setConfirming] = useState(false)
  const deletion = useMutation({
    mutationFn: onDelete,
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
      <p className="text-sm font-semibold text-heading">{question}</p>
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

export default ConfirmDeleteButton
