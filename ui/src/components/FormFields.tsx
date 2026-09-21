export function Field({
  label,
  value,
  onChange,
  disabled = false,
  type = 'text',
  maxLength,
  showCharacterCount = false,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  type?: string
  maxLength?: number
  showCharacterCount?: boolean
}) {
  const characterCount = Array.from(value).length
  return (
    <label className="field">
      <span className="field-label-row">
        <span>{label}</span>
        {showCharacterCount && maxLength ? (
          <span className={`field-character-count ${characterCount >= maxLength ? 'at-limit' : ''}`} aria-live="polite">
            {maxLength} max · {maxLength - characterCount} left
          </span>
        ) : null}
      </span>
      <input aria-label={label} type={type} value={value} disabled={disabled} maxLength={maxLength} onChange={(event) => onChange(event.target.value)} />
    </label>
  )
}

export function TextareaField({
  label,
  value,
  onChange,
  disabled = false,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}) {
  return (
    <label className="field">
      <span>{label}</span>
      <textarea value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)} rows={4} />
    </label>
  )
}

export function NumberField({
  label,
  value,
  onChange,
  disabled = false,
}: {
  label: string
  value: number | null
  onChange: (value: number | null) => void
  disabled?: boolean
}) {
  return (
    <label className="field">
      <span>{label}</span>
      <input
        type="number"
        min="0"
        value={value ?? ''}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value === '' ? null : Number(event.target.value))}
      />
    </label>
  )
}

export function SelectField({
  label,
  value,
  values,
  onChange,
}: {
  label: string
  value: string
  values: string[] | Array<{ value: string; label: string }>
  onChange: (value: string) => void
}) {
  const options = values.map((item) => (typeof item === 'string' ? { value: item, label: item.replaceAll('_', ' ') } : item))
  return (
    <label className="field">
      <span>{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  )
}
