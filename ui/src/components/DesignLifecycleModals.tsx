import { Copy, Trash2 } from 'lucide-react'
import { versionStatusLabel } from '../app/designUtils'
import type { BackendDesignVersion } from '../backendApi'
import type { BackendDesign } from '../backendSync'
import type { BackendWorkspace } from '../backendSync'

export type DeleteTarget =
  | { kind: 'workspace'; workspace: BackendWorkspace }
  | { kind: 'design'; design: BackendDesign }
  | { kind: 'version'; version: BackendDesignVersion }

export function ConfirmDeleteModal({
  target,
  isDeleting,
  onCancel,
  onConfirm,
}: {
  target: DeleteTarget
  isDeleting: boolean
  onCancel: () => void
  onConfirm: () => void
}) {
  const isWorkspace = target.kind === 'workspace'
  const isVersion = target.kind === 'version'
  const name = isWorkspace
    ? target.workspace.name
    : isVersion
      ? `v${target.version.versionNumber}`
      : target.design.name || target.design.document.title || 'Untitled design'
  const title = isWorkspace ? 'Delete workspace?' : isVersion ? 'Delete version?' : 'Delete design?'
  const detail = isWorkspace
    ? 'This workspace is empty. Deleting it cannot be undone.'
    : isVersion
      ? 'This will delete the saved version and its review requests. The current editable design is not deleted.'
      : 'This will delete the design and its saved versions. This action cannot be undone.'

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="confirm-delete-modal" role="dialog" aria-modal="true" aria-labelledby="delete-title">
        <div className="modal-heading">
          <p className="eyebrow">Confirm delete</p>
          <h2 id="delete-title">{title}</h2>
          <span>{detail}</span>
        </div>
        <div className="delete-summary">
          <strong>{name}</strong>
          <span>{isWorkspace ? 'Workspace' : isVersion ? `${versionStatusLabel(target.version.status)} version` : 'Design'}</span>
        </div>
        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onCancel} disabled={isDeleting}>
            Cancel
          </button>
          <button className="danger-action" type="button" onClick={onConfirm} disabled={isDeleting}>
            <Trash2 size={16} /> {isDeleting ? 'Deleting...' : 'Delete'}
          </button>
        </div>
      </section>
    </div>
  )
}

export function CloneDesignModal({
  sourceDesign,
  versions,
  selectedVersionId,
  title,
  state,
  error,
  onTitleChange,
  onVersionChange,
  onCancel,
  onClone,
}: {
  sourceDesign: BackendDesign
  versions: BackendDesignVersion[]
  selectedVersionId: string
  title: string
  state: 'idle' | 'loading' | 'cloning' | 'error'
  error: string | null
  onTitleChange: (value: string) => void
  onVersionChange: (value: string) => void
  onCancel: () => void
  onClone: () => void
}) {
  const sourceName = sourceDesign.name || sourceDesign.document.title || 'Untitled design'
  const isBusy = state === 'loading' || state === 'cloning' || state === 'error'
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="clone-design-modal" role="dialog" aria-modal="true" aria-labelledby="clone-title">
        <div className="modal-heading">
          <p className="eyebrow">Fork design</p>
          <h2 id="clone-title">Clone design</h2>
          <span>Creates a new design from one selected version. Version history, reviews, comments, and docs are not copied.</span>
        </div>
        <div className="clone-summary">
          <strong>{sourceName}</strong>
          <span>{versions.length ? `${versions.length} saved version${versions.length === 1 ? '' : 's'}` : 'Current design document'}</span>
        </div>
        <label className="field">
          <span>New design name</span>
          <input value={title} onChange={(event) => onTitleChange(event.target.value)} placeholder={`${sourceName} Copy`} />
        </label>
        <label className="field">
          <span>Source version</span>
          <select value={selectedVersionId} onChange={(event) => onVersionChange(event.target.value)} disabled={state === 'loading'}>
            {versions.length ? versions.map((version) => (
              <option key={version.id} value={version.id}>
                v{version.versionNumber} · {versionStatusLabel(version.status)} · {new Date(version.createdAt).toLocaleString()}
              </option>
            )) : (
              <option value="current">Current design document</option>
            )}
          </select>
        </label>
        {error ? <div className="admin-inline-error">{error}</div> : null}
        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onCancel} disabled={state === 'cloning'}>
            Cancel
          </button>
          <button className="primary-action" type="button" onClick={onClone} disabled={isBusy || !title.trim()}>
            <Copy size={16} /> {state === 'loading' ? 'Loading versions...' : state === 'cloning' ? 'Cloning...' : 'Clone design'}
          </button>
        </div>
      </section>
    </div>
  )
}
