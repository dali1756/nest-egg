import type { Ref, SelectHTMLAttributes } from 'react'
import FieldFrame from './FieldFrame'
import { controlClassName, describedBy } from './fieldStyles'

type SelectFieldProps = SelectHTMLAttributes<HTMLSelectElement> & {
  id: string
  label: string
  error?: string
  ref?: Ref<HTMLSelectElement>
}

function SelectField({ id, label, error, ref, children, ...select }: SelectFieldProps) {
  return (
    <FieldFrame id={id} label={label} error={error}>
      <select
        id={id}
        ref={ref}
        className={controlClassName(error)}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy(id, error)}
        {...select}
      >
        {children}
      </select>
    </FieldFrame>
  )
}

export default SelectField
