const chipClass =
  'cursor-pointer rounded-control border border-border bg-card px-3.5 py-2 text-sm font-semibold text-heading transition-colors hover:border-primary has-checked:border-primary has-checked:bg-brand-soft has-checked:text-primary has-focus-visible:outline-3 has-focus-visible:outline-offset-2 has-focus-visible:outline-ring'

function ChoiceGroup<T extends string | number>({
  legend,
  options,
  value,
  onChange,
  name,
  error,
}: {
  legend: string
  options: { value: T; label: string }[]
  value: T
  onChange: (value: T) => void
  name: string
  error?: string
}) {
  return (
    <fieldset className="flex flex-col gap-1.5" aria-describedby={error ? `${name}-error` : undefined}>
      <legend className="mb-1.5 text-sm font-semibold text-heading">{legend}</legend>
      <div className="flex flex-wrap gap-2">
        {options.map((option) => (
          <label key={option.value} className={chipClass}>
            <input
              type="radio"
              name={name}
              className="sr-only"
              checked={value === option.value}
              onChange={() => onChange(option.value)}
            />
            {option.label}
          </label>
        ))}
      </div>
      {error && (
        <p id={`${name}-error`} className="text-xs text-destructive-text">
          {error}
        </p>
      )}
    </fieldset>
  )
}

export default ChoiceGroup
