import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { NewDesignBriefModal } from '../src/App'
import { createEmptyRequirementBrief, MAX_DESIGN_NAME_LENGTH } from '../src/designModel'

describe('NewDesignBriefModal', () => {
  it('requires only a name and progressively reveals encouraged context', () => {
    const onCancel = vi.fn()
    const onCreate = vi.fn()
    const { rerender } = render(
      <NewDesignBriefModal
        title=""
        brief={createEmptyRequirementBrief()}
        isCreating={false}
        onTitleChange={vi.fn()}
        onBriefChange={vi.fn()}
        onCancel={onCancel}
        onCreate={onCreate}
      />,
    )

    expect((screen.getByRole('button', { name: 'Create design' }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByLabelText('Design name') as HTMLInputElement).maxLength).toBe(MAX_DESIGN_NAME_LENGTH)
    expect(screen.getByText('120 max · 120 left')).toBeTruthy()
    expect(screen.queryByLabelText('Functional requirements')).toBeNull()
    expect(screen.queryByLabelText('Open questions')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onCancel).toHaveBeenCalledTimes(1)

    rerender(
      <NewDesignBriefModal
        title="Checkout platform"
        brief={createEmptyRequirementBrief()}
        isCreating={false}
        onTitleChange={vi.fn()}
        onBriefChange={vi.fn()}
        onCancel={onCancel}
        onCreate={onCreate}
      />,
    )
    expect((screen.getByRole('button', { name: 'Create design' }) as HTMLButtonElement).disabled).toBe(false)
    expect(screen.getByText('120 max · 103 left')).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: 'Add requirements and targets' }))
    const functionalRequirements = screen.getByLabelText('Functional requirements')
    const sla = screen.getByLabelText('SLA')
    expect(functionalRequirements.compareDocumentPosition(sla) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(screen.queryByLabelText('Open questions')).toBeNull()
  })
})
