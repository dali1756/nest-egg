import type { ReactNode } from 'react'

type FieldFrameProps = {
  id: string
  label: string
  error?: string
  hint?: string
  children: ReactNode
}

function FieldFrame({ id, label, error, hint, children }: FieldFrameProps) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-sm font-semibold text-heading" htmlFor={id}>
        {label}
      </label>
      {children}
      {error && (
        <p id={`${id}-error`} className="text-xs text-destructive-text">
          {error}
        </p>
      )}
      {hint && (
        <p id={`${id}-hint`} className="text-xs text-muted-foreground">
          {hint}
        </p>
      )}
    </div>
  )
}

export default FieldFrame
