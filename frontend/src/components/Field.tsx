import type { InputHTMLAttributes, Ref } from 'react'

type FieldProps = InputHTMLAttributes<HTMLInputElement> & {
  id: string
  label: string
  error?: string
  hint?: string
  ref?: Ref<HTMLInputElement>
}

function Field({ id, label, error, hint, ref, ...input }: FieldProps) {
  const hintId = hint ? `${id}-hint` : undefined
  const errorId = error ? `${id}-error` : undefined
  const describedBy = [errorId, hintId].filter(Boolean).join(' ') || undefined

  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-sm font-semibold text-heading" htmlFor={id}>
        {label}
      </label>
      <input
        id={id}
        ref={ref}
        className={`rounded-control border bg-card px-3 py-2.5 text-md text-foreground transition-colors focus-visible:outline-3 focus-visible:outline-offset-1 focus-visible:outline-ring ${
          error ? 'border-destructive' : 'border-border hover:border-ring focus-visible:border-brand'
        }`}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        {...input}
      />
      {error && (
        <p id={errorId} className="text-xs text-destructive-text">
          {error}
        </p>
      )}
      {hint && (
        <p id={hintId} className="text-xs text-muted-foreground">
          {hint}
        </p>
      )}
    </div>
  )
}

export default Field
