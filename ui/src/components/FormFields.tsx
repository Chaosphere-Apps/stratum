export function Field({
  label,
  value,
  onChange,
  disabled = false,
  type = 'text',
}: {
  label: string
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  type?: string
}) {
  return (
    <label className="field">
      <span>{label}</span>
      <input type={type} value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)} />
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
