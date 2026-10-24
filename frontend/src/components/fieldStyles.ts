export function controlClassName(error?: string) {
  return `rounded-control border bg-card px-3 py-2.5 text-md text-foreground transition-colors focus-visible:outline-3 focus-visible:outline-offset-1 focus-visible:outline-ring ${
    error ? 'border-destructive' : 'border-border hover:border-ring focus-visible:border-brand'
  }`
}

export function describedBy(id: string, error?: string, hint?: string) {
  return [error && `${id}-error`, hint && `${id}-hint`].filter(Boolean).join(' ') || undefined
}
