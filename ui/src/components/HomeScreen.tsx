import { useMemo, useState } from 'react'
import { Boxes, Copy, FilePlus2, FolderPlus, LockKeyhole, Search, Trash2, UserRound, UsersRound, X } from 'lucide-react'
import { defaultWorkspace } from '../app/navigation'
import { fetchWorkspaceSharePrincipals } from '../backendApi'
import type { BackendPageInfo, BackendWorkspaceShareInput, BackendWorkspaceSharePrincipal } from '../backendApi'
import type { BackendDesign } from '../backendSync'
import type { BackendWorkspace } from '../backendSync'

export function HomeScreen({
  workspaces,
  designs,
  activeWorkspaceId,
  workspaceSearch,
  designSearch,
  workspacePage,
  designPage,
  loadingMore,
  error,
  action,
  onWorkspaceSearchChange,
  onDesignSearchChange,
  onSelectWorkspace,
  onCreateWorkspace,
  onDeleteWorkspace,
  onOpenDesign,
  onCloneDesign,
  onDeleteDesign,
  onNewDesign,
  onLoadMoreWorkspaces,
  onLoadMoreDesigns,
  onRefresh,
}: {
  workspaces: BackendWorkspace[]
  designs: BackendDesign[]
  activeWorkspaceId: string
  workspaceSearch: string
  designSearch: string
  workspacePage: BackendPageInfo | null
  designPage: BackendPageInfo | null
  loadingMore: 'workspaces' | 'designs' | null
  error: string | null
  action: 'workspace' | 'design' | 'delete-workspace' | 'delete-design' | 'delete-version' | null
  onWorkspaceSearchChange: (value: string) => void
  onDesignSearchChange: (value: string) => void
  onSelectWorkspace: (workspaceId: string) => void
  onCreateWorkspace: (input: { name: string; shares: BackendWorkspaceShareInput[] }) => boolean | Promise<boolean>
  onDeleteWorkspace: (workspace: BackendWorkspace) => void
  onOpenDesign: (design: BackendDesign) => void
  onCloneDesign: (design: BackendDesign) => void
  onDeleteDesign: (design: BackendDesign) => void
  onNewDesign: () => void | Promise<void>
  onLoadMoreWorkspaces: () => void | Promise<void>
  onLoadMoreDesigns: () => void | Promise<void>
  onRefresh: () => void
}) {
  const [createWorkspaceOpen, setCreateWorkspaceOpen] = useState(false)
  const [workspaceName, setWorkspaceName] = useState('')
  const [shareSearch, setShareSearch] = useState('')
  const [sharePrincipals, setSharePrincipals] = useState<BackendWorkspaceSharePrincipal[]>([])
  const [workspaceShares, setWorkspaceShares] = useState<BackendWorkspaceShareInput[]>([])
  const [principalLoading, setPrincipalLoading] = useState(false)
  const [principalError, setPrincipalError] = useState<string | null>(null)
  const activeWorkspace = workspaces.find((workspace) => workspace.id === activeWorkspaceId)
  const isCreatingWorkspace = action === 'workspace'
  const isCreatingDesign = action === 'design'
  const canDeleteActiveWorkspace =
    activeWorkspaceId !== defaultWorkspace.id && !designSearch.trim() && !designPage?.hasMore && designs.length === 0
  const filteredPrincipals = useMemo(() => {
    const query = shareSearch.trim().toLowerCase()
    return sharePrincipals.filter((principal) => {
      if (workspaceShares.some((share) => share.principalType === principal.type && share.principalId === principal.id)) return false
      return !query || `${principal.name} ${principal.description}`.toLowerCase().includes(query)
    }).slice(0, 8)
  }, [sharePrincipals, shareSearch, workspaceShares])

  async function openCreateWorkspace() {
    setCreateWorkspaceOpen(true)
    setPrincipalLoading(true)
    setPrincipalError(null)
    try {
      const response = await fetchWorkspaceSharePrincipals()
      setSharePrincipals(response.principals)
    } catch (loadError) {
      setPrincipalError(loadError instanceof Error ? loadError.message : 'Could not load people and groups')
    } finally {
      setPrincipalLoading(false)
    }
  }

  function closeCreateWorkspace() {
    if (isCreatingWorkspace) return
    setCreateWorkspaceOpen(false)
    setWorkspaceName('')
    setShareSearch('')
    setWorkspaceShares([])
    setPrincipalError(null)
  }

  async function submitWorkspace() {
    if (!workspaceName.trim() || isCreatingWorkspace) return
    const created = await onCreateWorkspace({ name: workspaceName.trim(), shares: workspaceShares })
    if (created) closeCreateWorkspace()
  }

  return (
    <main className="home-shell">
      <section className="home-command-header">
        <div className="home-command-copy">
          <p className="eyebrow">Architecture workspace</p>
          <h1>Make system design a shared source of truth.</h1>
          <p>
            Model systems, explain request journeys, and move architecture through review with the context teams need
            to make confident decisions.
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
          <div className="workspace-panel-heading">
            <div>
              <div className="home-section-title">Workspaces</div>
              <strong>{workspaces.length} available</strong>
            </div>
          </div>
          <label className="home-search">
            <Search size={15} />
            <input value={workspaceSearch} onChange={(event) => onWorkspaceSearchChange(event.target.value)} placeholder="Search workspaces..." />
          </label>
          <div className="workspace-list">
            {workspaces.map((workspace) => (
              <div
                className={`workspace-card ${workspace.id === activeWorkspaceId ? 'active' : ''}`}
                key={workspace.id}
              >
                <button type="button" className="workspace-card-main" onClick={() => onSelectWorkspace(workspace.id)}>
                  <span className="workspace-card-icon"><Boxes size={16} /></span>
                  <span className="workspace-card-copy">
                    <strong>{workspace.name}</strong>
                    <small>{workspace.id === 'guest-workspace' ? 'Default workspace' : 'Team workspace'}</small>
                  </span>
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
            {!workspaces.length ? (
              <div className="empty-search-result">No matching workspaces</div>
            ) : null}
            {workspacePage?.hasMore ? (
              <button className="load-more-button" type="button" disabled={loadingMore === 'workspaces'} onClick={() => void onLoadMoreWorkspaces()}>
                {loadingMore === 'workspaces' ? 'Loading...' : 'Load more workspaces'}
              </button>
            ) : null}
          </div>
          <button className="create-workspace-command" type="button" onClick={() => void openCreateWorkspace()}>
            <FolderPlus size={16} /> New workspace
          </button>
        </aside>

        <section className="designs-panel">
          <div className="designs-panel-heading">
            <div>
              <div className="home-section-title">Designs</div>
              <h2>{activeWorkspace?.name ?? 'Workspace'}</h2>
              <p>{designs.length} design{designs.length === 1 ? '' : 's'} in this workspace</p>
            </div>
            <button className="primary-action home-new-design" type="button" disabled={isCreatingDesign} onClick={() => void onNewDesign()}>
              <FilePlus2 size={16} /> {isCreatingDesign ? 'Creating...' : 'New design'}
            </button>
          </div>
          <label className="home-search design-search">
            <Search size={15} />
            <input value={designSearch} onChange={(event) => onDesignSearchChange(event.target.value)} placeholder="Search designs..." />
          </label>

          {designs.length ? (
            <div className="home-design-grid">
              {designs.map((backendDesign) => (
                <article className="home-design-card" key={backendDesign.id}>
                  <button type="button" className="home-design-open" onClick={() => onOpenDesign(backendDesign)}>
                    <div className="design-card-preview">
                      <div className="design-preview-map" aria-hidden="true">
                        <i /><i /><i /><i />
                      </div>
                      <div className="design-preview-count">
                        <strong>{backendDesign.document.components.length}</strong>
                        <span>components</span>
                      </div>
                    </div>
                    <div className="design-card-copy">
                      <strong>{backendDesign.name || backendDesign.document.title || 'Untitled design'}</strong>
                      <span>Updated {new Date(backendDesign.updatedAt).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}</span>
                    </div>
                    <div className="design-card-meta">
                      <span>{(backendDesign.versionNumber ?? 0) > 0 ? `v${backendDesign.versionNumber}` : 'Unversioned'}</span>
                      <span>{backendDesign.document.connectors.length} connections</span>
                      {(backendDesign.document.journeys?.length ?? 0) > 0 ? <span>{backendDesign.document.journeys.length} journeys</span> : null}
                    </div>
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
              {designPage?.hasMore ? (
                <button className="load-more-button design-load-more" type="button" disabled={loadingMore === 'designs'} onClick={() => void onLoadMoreDesigns()}>
                  {loadingMore === 'designs' ? 'Loading...' : 'Load more designs'}
                </button>
              ) : null}
            </div>
          ) : designSearch.trim() ? (
            <div className="empty-designs">
              <strong>No matching designs</strong>
              <span>Adjust the search term or clear it to see this workspace.</span>
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
      {createWorkspaceOpen ? (
        <div className="modal-backdrop workspace-create-backdrop" role="presentation" onMouseDown={(event) => {
          if (event.target === event.currentTarget) closeCreateWorkspace()
        }}>
          <section className="workspace-create-modal" role="dialog" aria-modal="true" aria-labelledby="workspace-create-title">
            <header className="workspace-create-header">
              <div>
                <span className="workspace-create-icon"><FolderPlus size={20} /></span>
                <div>
                  <p className="eyebrow">New workspace</p>
                  <h2 id="workspace-create-title">Create a focused space for a system</h2>
                </div>
              </div>
              <button className="icon-command" type="button" aria-label="Close workspace dialog" onClick={closeCreateWorkspace}><X size={18} /></button>
            </header>

            <label className="workspace-create-name">
              <span>Workspace name</span>
              <input autoFocus value={workspaceName} maxLength={120} placeholder="e.g. Payments platform" onChange={(event) => setWorkspaceName(event.target.value)} />
            </label>

            <div className="workspace-private-note">
              <LockKeyhole size={18} />
              <div><strong>Private by default</strong><span>Only you can access this workspace until you share it. You will remain its manager.</span></div>
            </div>

            <section className="workspace-share-section">
              <div className="workspace-share-heading">
                <div><strong>Share now</strong><span>Optional. Add people or groups and choose what they can do.</span></div>
                <span>{workspaceShares.length} selected</span>
              </div>
              <label className="home-search workspace-share-search">
                <Search size={15} />
                <input value={shareSearch} placeholder="Find a person or group" onChange={(event) => setShareSearch(event.target.value)} />
              </label>
              {principalError ? <div className="workspace-share-error">{principalError}</div> : null}
              {shareSearch.trim() ? (
                <div className="workspace-principal-results">
                  {principalLoading ? <span>Loading directory...</span> : filteredPrincipals.map((principal) => (
                    <button key={`${principal.type}:${principal.id}`} type="button" onClick={() => {
                      setWorkspaceShares((current) => [...current, { principalType: principal.type, principalId: principal.id, accessLevel: 'viewer' }])
                      setShareSearch('')
                    }}>
                      <i>{principal.type === 'group' ? <UsersRound size={16} /> : <UserRound size={16} />}</i>
                      <span><strong>{principal.name}</strong><small>{principal.description || (principal.type === 'group' ? 'Access group' : 'User')}</small></span>
                      <em>{principal.type}</em>
                    </button>
                  ))}
                  {!principalLoading && !filteredPrincipals.length ? <span>No matching people or groups</span> : null}
                </div>
              ) : null}
              {workspaceShares.length ? (
                <div className="workspace-share-list">
                  {workspaceShares.map((share) => {
                    const principal = sharePrincipals.find((item) => item.id === share.principalId && item.type === share.principalType)
                    return (
                      <div key={`${share.principalType}:${share.principalId}`}>
                        <span className="workspace-share-avatar">{share.principalType === 'group' ? <UsersRound size={16} /> : <UserRound size={16} />}</span>
                        <span className="workspace-share-copy"><strong>{principal?.name ?? 'Selected principal'}</strong><small>{principal?.description}</small></span>
                        <select aria-label={`Access for ${principal?.name ?? share.principalId}`} value={share.accessLevel} onChange={(event) => setWorkspaceShares((current) => current.map((item) => item.principalId === share.principalId && item.principalType === share.principalType ? { ...item, accessLevel: event.target.value as BackendWorkspaceShareInput['accessLevel'] } : item))}>
                          <option value="viewer">Viewer</option>
                          <option value="contributor">Contributor</option>
                          <option value="manager">Manager</option>
                        </select>
                        <button className="icon-command" type="button" aria-label={`Remove ${principal?.name ?? 'share'}`} onClick={() => setWorkspaceShares((current) => current.filter((item) => item !== share))}><X size={16} /></button>
                      </div>
                    )
                  })}
                </div>
              ) : null}
            </section>
            <footer className="workspace-create-actions">
              <button className="secondary-action" type="button" onClick={closeCreateWorkspace}>Cancel</button>
              <button className="primary-action" type="button" disabled={!workspaceName.trim() || isCreatingWorkspace} onClick={() => void submitWorkspace()}>
                <FolderPlus size={16} /> {isCreatingWorkspace ? 'Creating...' : 'Create workspace'}
              </button>
            </footer>
          </section>
        </div>
      ) : null}
    </main>
  )
}
