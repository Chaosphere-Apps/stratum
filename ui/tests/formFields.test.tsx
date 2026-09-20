import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Field, NumberField, SelectField, TextareaField } from '../src/components/FormFields'

describe('form fields', () => {
  it('reports text and textarea changes', async () => {
    const user = userEvent.setup()
    const onText = vi.fn()
    const onArea = vi.fn()
    render(<><Field label="Name" value="" onChange={onText} /><TextareaField label="Notes" value="" onChange={onArea} /></>)
    await user.type(screen.getByLabelText('Name'), 'A')
    await user.type(screen.getByLabelText('Notes'), 'B')
    expect(onText).toHaveBeenLastCalledWith('A')
    expect(onArea).toHaveBeenLastCalledWith('B')
  })

  it('maps empty and numeric values', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    const { rerender } = render(<NumberField label="RPS" value={null} onChange={onChange} />)
    fireEvent.change(screen.getByLabelText('RPS'), { target: { value: '42' } })
    expect(onChange).toHaveBeenLastCalledWith(42)
    rerender(<NumberField label="RPS" value={42} onChange={onChange} />)
    await user.clear(screen.getByLabelText('RPS'))
    expect(onChange).toHaveBeenLastCalledWith(null)
  })

  it('renders simple and labelled select options', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<SelectField label="Role" value="member" values={['member', { value: 'reviewer', label: 'Review only' }]} onChange={onChange} />)
    await user.selectOptions(screen.getByLabelText('Role'), 'reviewer')
    expect(onChange).toHaveBeenCalledWith('reviewer')
  })
})
