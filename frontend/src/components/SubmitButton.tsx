import type { ReactNode } from 'react'
import Button from './Button'

type SubmitButtonProps = { pending: boolean; pendingLabel: string; children: ReactNode }

function SubmitButton({ pending, pendingLabel, children }: SubmitButtonProps) {
  return (
    <Button type="submit" block disabled={pending}>
      {pending ? pendingLabel : children}
    </Button>
  )
}

export default SubmitButton
