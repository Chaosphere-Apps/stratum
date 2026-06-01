import { useState } from 'react'
import { Copy, FilePlus2, FolderPlus, Search, Trash2 } from 'lucide-react'
import { defaultWorkspace } from '../app/navigation'
import type { BackendDesign } from '../backendSync'
import type { BackendWorkspace } from '../backendSync'

export function HomeScreen({
  workspaces,
  designs,
  activeWorkspaceId,
  newWorkspaceName,
  error,
  action,
  onWorkspaceNameChange,
  onSelectWorkspace,
  onCreateWorkspace,
  onDeleteWorkspace,
  onOpenDesign,
  onCloneDesign,
  onDeleteDesign,
  onNewDesign,
  onRefresh,
}: {
  workspaces: BackendWorkspace[]
  designs: BackendDesign[]
  activeWorkspaceId: string
  newWorkspaceName: string
  error: string | null
  action: 'workspace' | 'design' | 'delete-workspace' | 'delete-design' | 'delete-version' | null
  onWorkspaceNameChange: (value: string) => void
  onSelectWorkspace: (workspaceId: string) => void
  onCreateWorkspace: () => void | Promise<void>
  onDeleteWorkspace: (workspace: BackendWorkspace) => void
  onOpenDesign: (design: BackendDesign) => void
  onCloneDesign: (design: BackendDesign) => void
  onDeleteDesign: (design: BackendDesign) => void
  onNewDesign: () => void | Promise<void>
  onRefresh: () => void
}) {
  const activeWorkspace = workspaces.find((workspace) => workspace.id === activeWorkspaceId)
  const isCreatingWorkspace = action === 'workspace'
  const isCreatingDesign = action === 'design'
  const canDeleteActiveWorkspace = activeWorkspaceId !== defaultWorkspace.id && designs.length === 0
  const [workspaceSearch, setWorkspaceSearch] = useState('')
  const [designSearch, setDesignSearch] = useState('')
  const filteredWorkspaces = workspaces.filter((workspace) =>
    workspace.name.toLowerCase().includes(workspaceSearch.trim().toLowerCase()),
  )
  const filteredDesigns = designs.filter((backendDesign) => {
    const query = designSearch.trim().toLowerCase()
    if (!query) return true
    return `${backendDesign.name} ${backendDesign.document.title}`.toLowerCase().includes(query)
  })

  return (
    <main className="home-shell">
      <section className="home-hero">
        <div>
          <p className="eyebrow">Architecture canvas</p>
          <h1>Design, structure, and evolve system architecture.</h1>
          <p>
            Create workspaces for design explorations, open diagrams like a board, and save each design as structured
            architecture code ready for versioning and evaluation.
          </p>
        </div>
      </section>

      {error ? (
        <div className="home-error">
          <span>{error}</span>
          <button onClick={onRefresh}>Retry</button>
        </div>
      ) : null}

      <section className="home-grid">
        <aside className="workspace-panel">
          <div className="home-section-title">Workspaces</div>
          <label className="home-search">
            <Search size={15} />
            <input value={workspaceSearch} onChange={(event) => setWorkspaceSearch(event.target.value)} placeholder="Search workspaces..." />
          </label>
          <div className="workspace-list">
            {filteredWorkspaces.map((workspace) => (
              <div
                className={`workspace-card ${workspace.id === activeWorkspaceId ? 'active' : ''}`}
                key={workspace.id}
              >
                <button type="button" className="workspace-card-main" onClick={() => onSelectWorkspace(workspace.id)}>
                  <strong>{workspace.name}</strong>
                  <span>{workspace.id === 'guest-workspace' ? 'Default workspace' : 'Workspace'}</span>
                </button>
                {workspace.id === activeWorkspaceId && canDeleteActiveWorkspace ? (
                  <button
                    type="button"
                    className="icon-danger-button"
                    aria-label={`Delete ${workspace.name}`}
                    onClick={() => onDeleteWorkspace(workspace)}
                  >
                    <Trash2 size={15} />
                  </button>
                ) : null}
              </div>
            ))}
            {!filteredWorkspaces.length ? (
              <div className="empty-search-result">No matching workspaces</div>
            ) : null}
          </div>
          <form
            className="create-workspace"
            onSubmit={(event) => {
              event.preventDefault()
              void onCreateWorkspace()
            }}
          >
            <input
              value={newWorkspaceName}
              placeholder="New workspace name"
              aria-label="New workspace name"
              onChange={(event) => onWorkspaceNameChange(event.target.value)}
            />
            <button type="submit" disabled={isCreatingWorkspace}>
              <FolderPlus size={16} /> {isCreatingWorkspace ? 'Creating...' : 'Create'}
            </button>
          </form>
        </aside>

        <section className="designs-panel">
          <div className="designs-panel-heading">
            <div>
              <div className="home-section-title">Designs</div>
              <h2>{activeWorkspace?.name ?? 'Workspace'}</h2>
            </div>
            <button className="secondary-action" type="button" disabled={isCreatingDesign} onClick={() => void onNewDesign()}>
              <FilePlus2 size={16} /> {isCreatingDesign ? 'Creating...' : 'New design'}
            </button>
          </div>
          <label className="home-search design-search">
            <Search size={15} />
            <input value={designSearch} onChange={(event) => setDesignSearch(event.target.value)} placeholder="Search designs..." />
          </label>

          {designs.length ? (
            <div className="home-design-grid">
              {filteredDesigns.map((backendDesign) => (
                <article className="home-design-card" key={backendDesign.id}>
                  <button type="button" className="home-design-open" onClick={() => onOpenDesign(backendDesign)}>
                    <div className="design-card-preview">
                      <span>{backendDesign.document.components.length}</span>
                      <small>components</small>
                    </div>
                    <strong>{backendDesign.name || backendDesign.document.title || 'Untitled design'}</strong>
                    <span>
                      {(backendDesign.versionNumber ?? 0) > 0 ? `v${backendDesign.versionNumber}` : 'No versions'} • {new Date(backendDesign.updatedAt).toLocaleString()}
                    </span>
                  </button>
                  <button
                    type="button"
                    className="icon-danger-button design-delete-button"
                    aria-label={`Delete ${backendDesign.name || backendDesign.document.title || 'design'}`}
                    onClick={() => onDeleteDesign(backendDesign)}
                  >
                    <Trash2 size={15} />
                  </button>
                  <button
                    type="button"
                    className="icon-secondary-button design-clone-button"
                    aria-label={`Clone ${backendDesign.name || backendDesign.document.title || 'design'}`}
                    onClick={() => onCloneDesign(backendDesign)}
                  >
                    <Copy size={15} />
                  </button>
                </article>
              ))}
              {!filteredDesigns.length ? (
                <div className="empty-search-result">No matching designs</div>
              ) : null}
            </div>
          ) : (
            <div className="empty-designs">
              <strong>No designs yet</strong>
              <span>Create the first design in this workspace.</span>
              <button className="primary-action" type="button" disabled={isCreatingDesign} onClick={() => void onNewDesign()}>
                <FilePlus2 size={18} /> {isCreatingDesign ? 'Creating...' : 'Create design'}
              </button>
            </div>
          )}
        </section>
      </section>
    </main>
  )
}
