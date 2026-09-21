import {
  Bell,
  ChevronDown,
  Eye,
  LayoutDashboard,
  LogOut,
  Settings,
  Trash2,
  UserCheck,
  UserPlus,
} from 'lucide-react'
import { versionStatusLabel } from '../app/designUtils'
import { initialsFor, roleLabel } from '../app/format'
import type { AppRoute } from '../app/navigation'
import type {
  BackendAIProviderConfig,
  BackendDesignVersion,
  BackendDesignVersionStatus,
  BackendDesignReviewRequest,
  BackendNotification,
  BackendProfile,
} from '../backendApi'
import type { BackendDesign } from '../backendSync'

export function AppNavbar({
  profile,
  route,
  workspaceName,
  designTitle,
  designs = [],
  selectedDesignId,
  designVersions = [],
  versionState = 'idle',
  previewVersion,
  reviews = [],
  currentUser,
  notifications,
  onHome,
  onWorkspace,
  onSelectDesign,
  onVersionStatusChange,
  onVersionReadyForReview,
  onDeleteVersion,
  onViewVersion,
  onAdmin,
  onOpenNotifications,
  onLogout,
}: {
  profile: BackendProfile | null
  route: AppRoute
  workspaceName?: string
  designTitle: string
  designs?: BackendDesign[]
  selectedDesignId?: string | null
  designVersions?: BackendDesignVersion[]
  versionState?: 'idle' | 'loading' | 'saving' | 'saved' | 'error'
  previewVersion?: BackendDesignVersion | null
  reviews?: BackendDesignReviewRequest[]
  aiConnection?: BackendAIProviderConfig | null
  currentUser: BackendProfile | null
  notifications: BackendNotification[]
  onHome: () => void
  onWorkspace: () => void
  onSelectDesign?: (design: BackendDesign) => void
  onVersionStatusChange?: (versionId: string, status: BackendDesignVersionStatus) => void
  onVersionReadyForReview?: (versionId: string) => void
  onDeleteVersion?: (versionId: string) => void
  onViewVersion?: (version: BackendDesignVersion) => void
  onAdmin: () => void
  onOpenNotifications: () => void
  onLogout: () => void
}) {
  const unreadCount = notifications.filter((notification) => !notification.read).length
  const activeUser = currentUser ?? profile
  const currentVersion = designVersions[0]
  const versionMenuVersion = previewVersion ?? currentVersion
  const reviewCountsByVersion = reviews.reduce<Record<string, { total: number; approved: number; changes: number }>>((counts, review) => {
    if (!review.versionId) return counts
    const current = counts[review.versionId] ?? { total: 0, approved: 0, changes: 0 }
    current.total += 1
    if (review.status === 'approved') current.approved += 1
    if (review.status === 'changes_requested') current.changes += 1
    counts[review.versionId] = current
    return counts
  }, {})

  return (
    <header className={`app-navbar ${route.screen === 'design' ? 'canvas-navbar' : ''}`}>
      <button className="app-brand" onClick={onHome} title="Go to Stratum home">
        <div className="home-logo">S</div>
        <div>
          <strong>Stratum</strong>
          <span>System design workspace</span>
        </div>
      </button>

      <nav className="app-breadcrumbs" aria-label="Current location">
        <button className={route.screen === 'home' ? 'active' : ''} onClick={onHome}>
          <LayoutDashboard size={16} /> Home
        </button>
        {route.screen !== 'home' && route.screen !== 'admin' ? (
          <button className={route.screen === 'workspace' ? 'active' : ''} onClick={onWorkspace}>
            {workspaceName ?? 'Workspace'}
          </button>
        ) : null}
        {route.screen === 'admin' ? <span>Admin</span> : null}
        {route.screen === 'design' ? (
          <select
            className="breadcrumb-design-select"
            aria-label="Switch design"
            value={selectedDesignId ?? ''}
            onChange={(event) => {
              const nextDesign = designs.find((item) => item.id === event.target.value)
              if (nextDesign) onSelectDesign?.(nextDesign)
            }}
          >
            {designs.map((backendDesign) => (
              <option key={backendDesign.id} value={backendDesign.id}>
                {backendDesign.name || backendDesign.document.title || designTitle || 'Untitled design'}
              </option>
            ))}
          </select>
        ) : null}
        {route.screen === 'design' ? (
          <details className="version-menu">
            <summary title="Design versions">
              <span>{versionMenuVersion ? `v${versionMenuVersion.versionNumber}` : 'No version'}</span>
              <strong>{versionMenuVersion ? versionStatusLabel(versionMenuVersion.status) : 'Unsaved'}</strong>
              <ChevronDown size={14} />
            </summary>
            <div className="version-menu-popover">
              <div className="version-menu-header">
                <strong>Versions</strong>
                <span>{versionState === 'loading' ? 'Loading...' : `${designVersions.length} saved`}</span>
              </div>
              {versionMenuVersion?.status === 'draft' && onVersionReadyForReview ? (
                <button
                  className="version-review-action"
                  type="button"
                  disabled={versionState === 'saving'}
                  onClick={() => onVersionReadyForReview(versionMenuVersion.id)}
                >
                  <UserPlus size={15} /> Ready for review v{versionMenuVersion.versionNumber}
                </button>
              ) : null}
              {designVersions.length ? (
                designVersions.map((version) => (
                  <article className={`version-menu-row ${previewVersion?.id === version.id ? 'previewing' : ''}`} key={version.id}>
                    <div>
                      <strong>v{version.versionNumber}</strong>
                      <span>{new Date(version.createdAt).toLocaleString()}</span>
                      {version.remarks ? <small>{version.remarks}</small> : null}
                      {reviewCountsByVersion[version.id] ? (
                        <small>
                          {reviewCountsByVersion[version.id].approved}/{reviewCountsByVersion[version.id].total} approved
                          {reviewCountsByVersion[version.id].changes ? ` · ${reviewCountsByVersion[version.id].changes} changes requested` : ''}
                        </small>
                      ) : null}
                    </div>
                    <div className="version-row-actions">
                      <button className="secondary-action compact-action" type="button" onClick={() => onViewVersion?.(version)}>
                        <Eye size={14} /> View
                      </button>
                      {version.status === 'draft' ? (
                        <button
                          className="secondary-action compact-action"
                          type="button"
                          disabled={versionState === 'saving' || !onVersionReadyForReview}
                          onClick={() => onVersionReadyForReview?.(version.id)}
                        >
                          <UserPlus size={14} /> Review
                        </button>
                      ) : null}
                      {version.status === 'reviewed' ? (
                        <button
                          className="secondary-action compact-action"
                          type="button"
                          disabled={versionState === 'saving' || !onVersionStatusChange}
                          onClick={() => onVersionStatusChange?.(version.id, 'live')}
                        >
                          Make live
                        </button>
                      ) : null}
                      <span className={`version-status-badge ${version.status}`}>{versionStatusLabel(version.status)}</span>
                      <button
                        className="icon-danger-action"
                        type="button"
                        disabled={versionState === 'saving' || !onDeleteVersion}
                        onClick={() => onDeleteVersion?.(version.id)}
                        title={`Delete v${version.versionNumber}`}
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </article>
                ))
              ) : (
                <p>No versions yet. Save the design and choose “new version”.</p>
              )}
            </div>
          </details>
        ) : null}
      </nav>

      <div className="navbar-actions">
        <button className="notification-button" type="button" onClick={onOpenNotifications} title="Notifications">
          <Bell size={18} />
          {unreadCount ? <span>{unreadCount}</span> : null}
        </button>
        <details className="user-menu">
          <summary title={activeUser?.email ?? 'Signed in'}>
            <span className="user-avatar" aria-hidden="true">{initialsFor(activeUser?.displayName || activeUser?.email || 'User')}</span>
            <span className="user-menu-label">
              <strong>{activeUser?.displayName ?? 'Signed in'}</strong>
              <small>{roleLabel(activeUser?.role ?? 'member')}</small>
            </span>
            <ChevronDown size={15} />
          </summary>
          <div className="user-menu-popover">
            <div className="user-menu-card">
              <span className="user-avatar large" aria-hidden="true">{initialsFor(activeUser?.displayName || activeUser?.email || 'User')}</span>
              <div>
                <strong>{activeUser?.displayName ?? 'Signed in'}</strong>
                <small>{activeUser?.email ?? roleLabel(activeUser?.role ?? 'member')}</small>
              </div>
            </div>
            <button type="button" disabled title="Profile editing is coming next">
              <UserCheck size={16} /> Edit profile
            </button>
            {activeUser?.role === 'admin' ? (
              <button type="button" onClick={onAdmin}>
                <Settings size={16} /> Admin console
              </button>
            ) : null}
            <button type="button" onClick={onLogout}>
              <LogOut size={16} /> Sign out
            </button>
          </div>
        </details>
      </div>
    </header>
  )
}
