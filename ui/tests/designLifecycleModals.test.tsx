import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { BackendDesignVersion } from '../src/backendApi'
import type { BackendDesign, BackendWorkspace } from '../src/backendSync'
import { createEmptyDesign } from '../src/designModel'
import { CloneDesignModal, ConfirmDeleteModal } from '../src/components/DesignLifecycleModals'

const workspace: BackendWorkspace = { id: 'workspace-one', name: 'Payments' }
const design: BackendDesign = {
  id: 'design-one', workspaceId: workspace.id, name: 'Checkout', access: 'private', title: 'Checkout',
  document: createEmptyDesign({ id: 'design-one', title: 'Checkout' }), versionNumber: 2, updatedAt: '2026-07-11T12:00:00Z',
}
const versions: BackendDesignVersion[] = [
  {
    id: 'version-two', designId: design.id, workspaceId: workspace.id, versionNumber: 2, status: 'reviewed',
    remarks: 'Approved', document: {}, createdBy: 'user-one', createdAt: '2026-07-11T12:00:00Z', updatedAt: '2026-07-11T12:00:00Z',
  },
]

describe('ConfirmDeleteModal', () => {
  it('confirms design deletion', async () => {
    const onCancel = vi.fn()
    const onConfirm = vi.fn()
    render(<ConfirmDeleteModal target={{ kind: 'design', design }} isDeleting={false} onCancel={onCancel} onConfirm={onConfirm} />)

    expect(screen.getByRole('heading', { name: 'Delete design?' })).toBeTruthy()
    expect(screen.getByText('Design')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('describes workspace and version deletion accurately', () => {
    const { rerender } = render(
      <ConfirmDeleteModal target={{ kind: 'workspace', workspace }} isDeleting={false} onCancel={vi.fn()} onConfirm={vi.fn()} />,
    )
    expect(screen.getByRole('heading', { name: 'Delete workspace?' })).toBeTruthy()
    expect(screen.getByText('Workspace')).toBeTruthy()

    rerender(<ConfirmDeleteModal target={{ kind: 'version', version: versions[0] }} isDeleting={false} onCancel={vi.fn()} onConfirm={vi.fn()} />)
    expect(screen.getByRole('heading', { name: 'Delete version?' })).toBeTruthy()
    expect(screen.getByText('Reviewed version')).toBeTruthy()
  })

  it('locks actions while deletion is in progress', () => {
    render(<ConfirmDeleteModal target={{ kind: 'design', design }} isDeleting onCancel={vi.fn()} onConfirm={vi.fn()} />)
    expect((screen.getByRole('button', { name: 'Cancel' }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: 'Deleting...' }) as HTMLButtonElement).disabled).toBe(true)
  })
})

describe('CloneDesignModal', () => {
  it('selects the source version, edits the title, and submits', async () => {
    const onTitleChange = vi.fn()
    const onVersionChange = vi.fn()
    const onClone = vi.fn()
    render(
      <CloneDesignModal sourceDesign={design} versions={versions} selectedVersionId="version-two" title="Checkout fork"
        state="idle" error={null} onTitleChange={onTitleChange} onVersionChange={onVersionChange}
        onCancel={vi.fn()} onClone={onClone} />,
    )

    expect(screen.getByText('1 saved version')).toBeTruthy()
    fireEvent.change(screen.getByRole('textbox', { name: 'New design name' }), { target: { value: 'Checkout fork 2' } })
    expect(onTitleChange).toHaveBeenCalledWith('Checkout fork 2')
    fireEvent.change(screen.getByRole('combobox', { name: 'Source version' }), { target: { value: 'version-two' } })
    expect(onVersionChange).toHaveBeenCalledWith('version-two')
    fireEvent.click(screen.getByRole('button', { name: 'Clone design' }))
    expect(onClone).toHaveBeenCalledTimes(1)
  })

  it('supports cloning the current document and reports failures', () => {
    render(
      <CloneDesignModal sourceDesign={design} versions={[]} selectedVersionId="current" title="" state="error"
        error="Could not clone design" onTitleChange={vi.fn()} onVersionChange={vi.fn()}
        onCancel={vi.fn()} onClone={vi.fn()} />,
    )
    expect(screen.getAllByText('Current design document')).toHaveLength(2)
    expect(screen.getByText('Could not clone design')).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Clone design' }) as HTMLButtonElement).disabled).toBe(true)
  })
})
