import type { InputHTMLAttributes, Ref } from 'react'
import FieldFrame from './FieldFrame'
import { controlClassName, describedBy } from './fieldStyles'

type FieldProps = InputHTMLAttributes<HTMLInputElement> & {
  id: string
  label: string
  error?: string
  hint?: string
  ref?: Ref<HTMLInputElement>
}

function Field({ id, label, error, hint, ref, ...input }: FieldProps) {
  return (
    <FieldFrame id={id} label={label} error={error} hint={hint}>
      <input
        id={id}
        ref={ref}
        className={controlClassName(error)}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy(id, error, hint)}
        {...input}
      />
    </FieldFrame>
  )
}

export default Field
