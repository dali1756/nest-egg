import type { Ref, SelectHTMLAttributes } from 'react'

type SelectFieldProps = SelectHTMLAttributes<HTMLSelectElement> & {
  id: string
  label: string
  error?: string
  ref?: Ref<HTMLSelectElement>
}

function SelectField({ id, label, error, ref, children, ...select }: SelectFieldProps) {
  const errorId = error ? `${id}-error` : undefined

  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-sm font-semibold text-heading" htmlFor={id}>
        {label}
      </label>
      <select
        id={id}
        ref={ref}
        className={`rounded-control border bg-card px-3 py-2.5 text-md text-foreground transition-colors focus-visible:outline-3 focus-visible:outline-offset-1 focus-visible:outline-ring ${
          error ? 'border-destructive' : 'border-border hover:border-ring focus-visible:border-brand'
        }`}
        aria-invalid={error ? true : undefined}
        aria-describedby={errorId}
        {...select}
      >
        {children}
      </select>
      {error && (
        <p id={errorId} className="text-xs text-destructive-text">
          {error}
        </p>
      )}
    </div>
  )
}

export default SelectField
