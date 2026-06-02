import { useEffect, useMemo, useState } from 'react'
import {
  Activity,
  ArrowDown,
  ArrowUp,
  BadgeCheck,
  Boxes,
  Braces,
  CheckCircle2,
  ChevronRight,
  Cloud,
  Database,
  FilePlus2,
  Globe2,
  KeyRound,
  LayoutDashboard,
  LockKeyhole,
  LogOut,
  Search,
  RotateCcw,
  Server,
  Settings,
  ShieldCheck,
  Sparkles,
  Upload,
  Users,
  UserCheck,
  UserPlus,
} from 'lucide-react'

import { componentCatalog, getCatalogItem } from '../catalog'
import { adminSectionCopy, adminSectionOrder, type AdminSection } from '../app/navigation'
import { catalogAssetUsageLabel, isPotentialCatalogMatch, normalizedCatalogLabel } from '../app/designUtils'
import { formatCompactDateTime, initialsFor, roleLabel, userDisplayName } from '../app/format'
import {
  createAccessGroup,
  deleteAccessGroup,
  fetchAccessGroupMembers,
  fetchAccessGroups,
  fetchDesignAccess,
  fetchDesignGroupAccess,
  fetchUserAccessSummary,
  fetchWorkspaceAccess,
  fetchWorkspaceDesigns,
  fetchWorkspaceGroupAccess,
  grantDesignAccess,
  grantDesignGroupAccess,
  grantWorkspaceAccess,
  grantWorkspaceGroupAccess,
  replaceAccessGroupMembers,
  revokeDesignAccess,
  revokeDesignGroupAccess,
  revokeWorkspaceAccess,
  revokeWorkspaceGroupAccess,
  updateAccessGroup,
  type BackendAccessGroup,
  type BackendAccessGroupMember,
  type BackendAIProviderConfig,
  type BackendCatalogAsset,
  type BackendDesignAccess,
  type BackendDesignGroupAccess,
  type BackendMCPConfig,
  type BackendSignInConfig,
  type BackendStorageStatus,
  type BackendUser,
  type BackendUserAccessSummary,
  type BackendWorkspaceAccess,
  type BackendWorkspaceGroupAccess,
} from '../backendApi'
import type { BackendDesign, BackendWorkspace } from '../backendSync'
import type { ComponentType } from '../types'
import { CatalogGlyph, isCatalogComponentType } from './CatalogGlyph'
import { Field, TextareaField } from './FormFields'

function AdminSectionIcon({ section, size = 18 }: { section: AdminSection; size?: number }) {
  if (section === 'overview') return <LayoutDashboard size={size} />
  if (section === 'users') return <UserCheck size={size} />
  if (section === 'access') return <ShieldCheck size={size} />
  if (section === 'workspaces') return <Boxes size={size} />
  if (section === 'catalog') return <Database size={size} />
  if (section === 'identity') return <LockKeyhole size={size} />
  if (section === 'ai') return <Sparkles size={size} />
  if (section === 'integrations') return <Server size={size} />
  if (section === 'security') return <BadgeCheck size={size} />
  return <Settings size={size} />
}

export function AdminConsole({
  users,
  activeUserId,
  workspaces,
  storageStatus,
  signInConfig,
  aiProviderConfig,
  mcpConfig,
  catalogAssets,
  error,
  onAddUser,
  onSaveUser,
  onGeneratePasswordResetLink,
  onDeleteUser,
  onSaveSignIn,
  onSaveAIProvider,
  onSaveMCP,
  onAddCatalogAsset,
  onSaveCatalogAsset,
  onDeleteCatalogAsset,
  onRefreshStorage,
  onTestDatabase,
  onConfigureDatabase,
  onMigrateStorageUsers,
  activeSection,
  onSelectSection,
}: {
  users: BackendUser[]
  activeUserId: string
  workspaces: BackendWorkspace[]
  storageStatus: BackendStorageStatus | null
  signInConfig: BackendSignInConfig | null
  aiProviderConfig: BackendAIProviderConfig | null
  mcpConfig: BackendMCPConfig | null
  catalogAssets: BackendCatalogAsset[]
  error: string | null
  onAddUser: (input: { displayName: string; email: string; role: string }) => Promise<void>
  onSaveUser: (user: BackendUser) => Promise<void>
  onGeneratePasswordResetLink: (userId: string) => Promise<{ resetLink: string; expiresAt: string }>
  onDeleteUser: (userId: string) => void
  onSaveSignIn: (config: BackendSignInConfig & { clientSecret?: string }) => void
  onSaveAIProvider: (config: Partial<BackendAIProviderConfig> & { apiKey?: string }) => void
  onSaveMCP: (config: BackendMCPConfig) => void
  onAddCatalogAsset: (input: { name: string; type: string; owner?: string; description?: string; criticality?: string; tags?: string[] }) => Promise<BackendCatalogAsset>
  onSaveCatalogAsset: (asset: BackendCatalogAsset) => Promise<void>
  onDeleteCatalogAsset: (assetId: string) => Promise<void>
  onRefreshStorage: () => Promise<BackendStorageStatus>
  onTestDatabase: (databaseUrl: string) => Promise<{ ok: boolean; storage: BackendStorageStatus }>
  onConfigureDatabase: (databaseUrl: string) => Promise<BackendStorageStatus>
  onMigrateStorageUsers: () => Promise<{ storage: BackendStorageStatus; migratedUsers: number }>
  activeSection: AdminSection
  onSelectSection: (section: AdminSection) => void
}) {
  const [newUser, setNewUser] = useState({ displayName: '', email: '', role: 'member' })
  const [newAsset, setNewAsset] = useState({ name: '', type: 'compute.service', owner: '', description: '', criticality: 'medium' })
  const [draftUsers, setDraftUsers] = useState<BackendUser[]>(users)
  const [draftAssets, setDraftAssets] = useState<BackendCatalogAsset[]>(catalogAssets)
  const [editingUserId, setEditingUserId] = useState<string | null>(null)
  const [passwordResetLinks, setPasswordResetLinks] = useState<Record<string, { resetLink: string; expiresAt: string }>>({})
  const [passwordResetLoading, setPasswordResetLoading] = useState<string | null>(null)
  const [editingAssetId, setEditingAssetId] = useState<string | null>(null)
  const [userError, setUserError] = useState<string | null>(null)
  const [assetError, setAssetError] = useState<string | null>(null)
  const [signInDraft, setSignInDraft] = useState<(BackendSignInConfig & { clientSecret?: string }) | null>(signInConfig)
  const [aiDraft, setAIDraft] = useState<(BackendAIProviderConfig & { apiKey?: string }) | null>(aiProviderConfig)
  const [mcpDraft, setMCPDraft] = useState<BackendMCPConfig | null>(mcpConfig)
  const [storageDatabaseUrl, setStorageDatabaseUrl] = useState('')
  const [storageAction, setStorageAction] = useState<'idle' | 'refreshing' | 'testing' | 'configuring' | 'migrating'>('idle')
  const [storageMessage, setStorageMessage] = useState<string | null>(null)
  const [storageError, setStorageError] = useState<string | null>(null)
  const [adminSearch, setAdminSearch] = useState('')
  const [catalogAdminSearch, setCatalogAdminSearch] = useState('')
  const [accessScope, setAccessScope] = useState<'workspace' | 'design'>('workspace')
  const [accessWorkspaceId, setAccessWorkspaceId] = useState(workspaces[0]?.id ?? '')
  const [accessDesignId, setAccessDesignId] = useState('')
  const [accessDesigns, setAccessDesigns] = useState<BackendDesign[]>([])
  const [workspaceAccess, setWorkspaceAccess] = useState<BackendWorkspaceAccess[]>([])
  const [designAccess, setDesignAccess] = useState<BackendDesignAccess[]>([])
  const [workspaceGroupAccess, setWorkspaceGroupAccess] = useState<BackendWorkspaceGroupAccess[]>([])
  const [designGroupAccess, setDesignGroupAccess] = useState<BackendDesignGroupAccess[]>([])
  const [accessGroups, setAccessGroups] = useState<BackendAccessGroup[]>([])
  const [selectedGroupId, setSelectedGroupId] = useState('')
  const [groupMembers, setGroupMembers] = useState<BackendAccessGroupMember[]>([])
  const [accessSubject, setAccessSubject] = useState<'user' | 'group'>('user')
  const [accessUserId, setAccessUserId] = useState('')
  const [accessGroupId, setAccessGroupId] = useState('')
  const [newAccessGroup, setNewAccessGroup] = useState({ name: '', description: '', oktaGroupName: '' })
  const [editingGroupId, setEditingGroupId] = useState<string | null>(null)
  const [draftAccessGroups, setDraftAccessGroups] = useState<BackendAccessGroup[]>([])
  const [workspaceAccessDraft, setWorkspaceAccessDraft] = useState({ canRead: true, canCreateDesign: false, canManage: false })
  const [designAccessDraft, setDesignAccessDraft] = useState({
    canRead: true,
    canEdit: false,
    canComment: true,
    canReview: false,
    canManage: false,
  })
  const [accessError, setAccessError] = useState<string | null>(null)
  const [accessLoading, setAccessLoading] = useState(false)
  const [userAccessSummary, setUserAccessSummary] = useState<BackendUserAccessSummary[]>([])
  const [userAccessLoading, setUserAccessLoading] = useState(false)

  useEffect(() => setDraftUsers(users), [users])
  useEffect(() => setDraftAssets(catalogAssets), [catalogAssets])
  useEffect(() => setSignInDraft(signInConfig), [signInConfig])
  useEffect(() => setAIDraft(aiProviderConfig), [aiProviderConfig])
  useEffect(() => setMCPDraft(mcpConfig), [mcpConfig])
  useEffect(() => {
    if (!accessWorkspaceId && workspaces[0]) setAccessWorkspaceId(workspaces[0].id)
  }, [accessWorkspaceId, workspaces])

  const activeUsers = users.filter((user) => user.status !== 'disabled').length
  const adminUsers = users.filter((user) => user.role === 'admin' && user.status !== 'disabled').length
  const reviewerUsers = users.filter((user) => user.role === 'reviewer' && user.status !== 'disabled').length
  const architectUsers = users.filter((user) => user.role === 'architect' && user.status !== 'disabled').length
  const ssoMode = signInDraft?.ssoEnabled ? 'SSO ready' : 'Local only'
  const localPasswordManaged = signInDraft?.localPasswordEnabled ?? true
  const providerLabel = signInDraft?.provider?.trim() || 'Okta'
  const hasOktaCore = Boolean(signInDraft?.issuer?.trim() && signInDraft?.clientId?.trim())
  const aiStatus = aiDraft?.enabled && aiDraft.apiKeySet ? 'AI analysis enabled' : 'AI analysis off'
  const aiVerifiedAt = aiDraft?.verifiedAt && !aiDraft.verifiedAt.startsWith('0001-') ? aiDraft.verifiedAt : ''
  const enabledMCPCapabilities = mcpDraft
    ? [mcpDraft.readCatalog, mcpDraft.readDesigns, mcpDraft.createDraftDesign, mcpDraft.runAnalysis, mcpDraft.fetchImpactReport].filter(Boolean).length
    : 0
  const duplicateNewAsset = catalogAssets.find((asset) => normalizedCatalogLabel(asset.name) === normalizedCatalogLabel(newAsset.name))
  const newAssetSuggestions = catalogAssets.filter((asset) => isPotentialCatalogMatch(newAsset.name, asset.name))
  const activeSectionCopy = adminSectionCopy[activeSection]
  const visibleAdminSections = adminSectionOrder.filter((section) => {
    const query = adminSearch.trim().toLowerCase()
    if (!query) return true
    const copy = adminSectionCopy[section]
    return `${copy.label} ${copy.description} ${copy.group}`.toLowerCase().includes(query)
  })
  const dashboardAttention = [
    {
      section: 'identity' as AdminSection,
      title: hasOktaCore ? 'Identity provider configured' : 'Complete identity provider',
      detail: hasOktaCore ? 'Okta issuer and client ID are present.' : 'Okta issuer and client ID are still missing.',
      status: hasOktaCore ? 'ready' : 'warning',
    },
    {
      section: 'ai' as AdminSection,
      title: aiDraft?.enabled && aiDraft.apiKeySet ? 'AI provider active' : 'Configure analysis AI',
      detail: aiDraft?.enabled && aiDraft.apiKeySet ? `${aiDraft.provider} ${aiDraft.model}` : 'Analysis, vision, and draft generation need a central provider.',
      status: aiDraft?.enabled && aiDraft.apiKeySet ? 'ready' : 'warning',
    },
    {
      section: 'integrations' as AdminSection,
      title: mcpDraft?.enabled ? 'MCP readiness enabled' : 'Review MCP readiness',
      detail: mcpDraft?.enabled ? `${enabledMCPCapabilities} capabilities selected.` : 'MCP is disabled until an admin explicitly enables it.',
      status: mcpDraft?.enabled ? 'ready' : 'neutral',
    },
  ]
  const activeAccessWorkspace = workspaces.find((workspace) => workspace.id === accessWorkspaceId)
  const activeAccessDesign = accessDesigns.find((design) => design.id === accessDesignId)
  const selectedAccessGroup = accessGroups.find((group) => group.id === accessGroupId)
  const selectedGroup = accessGroups.find((group) => group.id === selectedGroupId)
  const accessRows = (accessScope === 'workspace' ? workspaceAccess : designAccess).filter((grant) =>
    'canCreateDesign' in grant
      ? grant.canRead || grant.canCreateDesign || grant.canManage
      : grant.canRead || grant.canEdit || grant.canComment || grant.canReview || grant.canManage,
  )
  const groupAccessRows = (accessScope === 'workspace' ? workspaceGroupAccess : designGroupAccess).filter((grant) =>
    'canCreateDesign' in grant
      ? grant.canRead || grant.canCreateDesign || grant.canManage
      : grant.canRead || grant.canEdit || grant.canComment || grant.canReview || grant.canManage,
  )
  const grantableUsers = useMemo(() => users.filter((user) => user.status !== 'disabled'), [users])
  const selectedAccessUser = grantableUsers.find((user) => user.id === accessUserId)
  const selectedGroupUserIds = new Set(groupMembers.map((member) => member.userId))
  const workspaceGrantHasPermissions = workspaceAccessDraft.canRead || workspaceAccessDraft.canCreateDesign || workspaceAccessDraft.canManage
  const designGrantHasPermissions =
    designAccessDraft.canRead || designAccessDraft.canEdit || designAccessDraft.canComment || designAccessDraft.canReview || designAccessDraft.canManage
  const accessGrantHasPermissions = accessScope === 'workspace' ? workspaceGrantHasPermissions : designGrantHasPermissions
  const catalogQuery = normalizedCatalogLabel(catalogAdminSearch)
  const filteredDraftAssets = draftAssets.filter((asset) => {
    if (!catalogQuery) return true
    return [asset.name, asset.type, asset.owner, asset.description, asset.normalizedName].some((value) =>
      normalizedCatalogLabel(value ?? '').includes(catalogQuery),
    )
  })

  useEffect(() => {
    if (grantableUsers.length && !accessUserId) setAccessUserId(grantableUsers[0].id)
  }, [accessUserId, grantableUsers])

  useEffect(() => {
    if (accessGroups.length && !accessGroupId) setAccessGroupId(accessGroups[0].id)
    if (accessGroups.length && !selectedGroupId) setSelectedGroupId(accessGroups[0].id)
  }, [accessGroupId, accessGroups, selectedGroupId])

  useEffect(() => {
    let cancelled = false
    async function loadAccessCenter() {
      if (activeSection !== 'access' || !accessWorkspaceId) return
      setAccessLoading(true)
      setAccessError(null)
      try {
        const [designResponse, workspaceAccessResponse, workspaceGroupAccessResponse, accessGroupsResponse] = await Promise.all([
          fetchWorkspaceDesigns(accessWorkspaceId, { limit: 100 }),
          fetchWorkspaceAccess(accessWorkspaceId),
          fetchWorkspaceGroupAccess(accessWorkspaceId),
          fetchAccessGroups(),
        ])
        if (cancelled) return
        setAccessDesigns(designResponse.designs)
        setWorkspaceAccess(workspaceAccessResponse.access)
        setWorkspaceGroupAccess(workspaceGroupAccessResponse.access)
        setAccessGroups(accessGroupsResponse.groups)
        setDraftAccessGroups(accessGroupsResponse.groups)
        const nextDesignId = accessDesignId || designResponse.designs[0]?.id || ''
        if (!accessDesignId && nextDesignId) setAccessDesignId(nextDesignId)
        if (nextDesignId) {
          const [designAccessResponse, designGroupAccessResponse] = await Promise.all([
            fetchDesignAccess(accessWorkspaceId, nextDesignId),
            fetchDesignGroupAccess(accessWorkspaceId, nextDesignId),
          ])
          if (!cancelled) {
            setDesignAccess(designAccessResponse.access)
            setDesignGroupAccess(designGroupAccessResponse.access)
          }
        } else {
          setDesignAccess([])
          setDesignGroupAccess([])
        }
      } catch (error) {
        if (!cancelled) setAccessError(error instanceof Error ? error.message : 'Could not load access settings.')
      } finally {
        if (!cancelled) setAccessLoading(false)
      }
    }
    void loadAccessCenter()
    return () => {
      cancelled = true
    }
  }, [accessDesignId, accessWorkspaceId, activeSection])

  useEffect(() => {
    let cancelled = false
    async function loadGroupMembers() {
      if (activeSection !== 'access' || !selectedGroupId) {
        setGroupMembers([])
        return
      }
      try {
        const response = await fetchAccessGroupMembers(selectedGroupId)
        if (!cancelled) setGroupMembers(response.members)
      } catch (error) {
        if (!cancelled) setAccessError(error instanceof Error ? error.message : 'Could not load group members.')
      }
    }
    void loadGroupMembers()
    return () => {
      cancelled = true
    }
  }, [activeSection, selectedGroupId])

  useEffect(() => {
    let cancelled = false
    async function loadUserAccessSummary() {
      if (activeSection !== 'access' || !accessUserId) return
      try {
        await refreshUserAccessSummary(accessUserId)
      } catch (error) {
        if (!cancelled) setAccessError(error instanceof Error ? error.message : 'Could not load user access summary.')
      }
    }
    void loadUserAccessSummary()
    return () => {
      cancelled = true
    }
  }, [accessUserId, activeSection])

  function updateDraftUser(user: BackendUser, patch: Partial<BackendUser>) {
    setDraftUsers((current) => current.map((item) => (item.id === user.id ? { ...item, ...patch } : item)))
  }

  function updateDraftAsset(asset: BackendCatalogAsset, patch: Partial<BackendCatalogAsset>) {
    setDraftAssets((current) => current.map((item) => (item.id === asset.id ? { ...item, ...patch } : item)))
  }

  function updateDraftAccessGroup(group: BackendAccessGroup, patch: Partial<BackendAccessGroup>) {
    setDraftAccessGroups((current) => current.map((item) => (item.id === group.id ? { ...item, ...patch } : item)))
  }

  async function submitNewUser() {
    const email = newUser.email.trim().toLowerCase()
    if (users.some((user) => user.email.toLowerCase() === email)) {
      setUserError('A user with this email already exists.')
      return
    }
    setUserError(null)
    await onAddUser({ ...newUser, email })
    setNewUser({ displayName: '', email: '', role: 'member' })
  }

  async function submitUserSave(user: BackendUser) {
    const email = user.email.trim().toLowerCase()
    if (draftUsers.some((item) => item.id !== user.id && item.email.trim().toLowerCase() === email)) {
      setUserError('A user with this email already exists.')
      return
    }
    setUserError(null)
    await onSaveUser({ ...user, email })
    setEditingUserId(null)
  }

  async function generateResetLink(user: BackendUser) {
    setUserError(null)
    setPasswordResetLoading(user.id)
    try {
      const resetLink = await onGeneratePasswordResetLink(user.id)
      setPasswordResetLinks((current) => ({ ...current, [user.id]: resetLink }))
      await navigator.clipboard?.writeText(resetLink.resetLink).catch(() => undefined)
    } catch (error) {
      setUserError(error instanceof Error ? error.message : 'Could not generate reset link.')
    } finally {
      setPasswordResetLoading(null)
    }
  }

  async function submitNewAsset() {
    if (duplicateNewAsset) {
      setAssetError(`"${duplicateNewAsset.name}" already exists in the catalog.`)
      return
    }
    setAssetError(null)
    try {
      await onAddCatalogAsset(newAsset)
      setNewAsset({ name: '', type: 'compute.service', owner: '', description: '', criticality: 'medium' })
    } catch (error) {
      setAssetError(error instanceof Error ? error.message : 'Could not add catalog asset.')
    }
  }

  async function submitAssetSave(asset: BackendCatalogAsset) {
    const duplicate = draftAssets.find(
      (item) => item.id !== asset.id && normalizedCatalogLabel(item.name) === normalizedCatalogLabel(asset.name),
    )
    if (duplicate) {
      setAssetError(`"${duplicate.name}" already exists in the catalog.`)
      return
    }
    setAssetError(null)
    try {
      await onSaveCatalogAsset(asset)
      setEditingAssetId(null)
    } catch (error) {
      setAssetError(error instanceof Error ? error.message : 'Could not save catalog asset.')
    }
  }

  async function refreshStorageSettings() {
    setStorageAction('refreshing')
    setStorageError(null)
    setStorageMessage(null)
    try {
      await onRefreshStorage()
      setStorageMessage('Storage status refreshed.')
    } catch (error) {
      setStorageError(error instanceof Error ? error.message : 'Could not refresh storage status.')
    } finally {
      setStorageAction('idle')
    }
  }

  async function testStorageSettingsDatabase() {
    const databaseUrl = storageDatabaseUrl.trim()
    if (!databaseUrl) {
      setStorageError('Database URL is required.')
      return
    }
    setStorageAction('testing')
    setStorageError(null)
    setStorageMessage(null)
    try {
      await onTestDatabase(databaseUrl)
      setStorageMessage('Database connection test passed.')
    } catch (error) {
      setStorageError(error instanceof Error ? error.message : 'Database connection test failed.')
    } finally {
      setStorageAction('idle')
    }
  }

  async function configureStorageSettingsDatabase() {
    const databaseUrl = storageDatabaseUrl.trim()
    if (!databaseUrl) {
      setStorageError('Database URL is required.')
      return
    }
    setStorageAction('configuring')
    setStorageError(null)
    setStorageMessage(null)
    try {
      const nextStatus = await onConfigureDatabase(databaseUrl)
      setStorageMessage(nextStatus.stateless ? 'Database is connected and ready for user migration.' : 'Database storage is active.')
    } catch (error) {
      setStorageError(error instanceof Error ? error.message : 'Could not configure database storage.')
    } finally {
      setStorageAction('idle')
    }
  }

  async function migrateStorageSettingsUsers() {
    if (!window.confirm('Migrate cached users into the configured database? This requires the target database to have no users.')) return
    setStorageAction('migrating')
    setStorageError(null)
    setStorageMessage(null)
    try {
      const response = await onMigrateStorageUsers()
      setStorageMessage(`Migrated ${response.migratedUsers} user${response.migratedUsers === 1 ? '' : 's'} and switched to database storage.`)
    } catch (error) {
      setStorageError(error instanceof Error ? error.message : 'Could not migrate users to database.')
    } finally {
      setStorageAction('idle')
    }
  }

  async function reloadAccessGroups() {
    const response = await fetchAccessGroups()
    setAccessGroups(response.groups)
    setDraftAccessGroups(response.groups)
    if (response.groups.length && !selectedGroupId) setSelectedGroupId(response.groups[0].id)
    if (response.groups.length && !accessGroupId) setAccessGroupId(response.groups[0].id)
    if (!response.groups.some((group) => group.id === selectedGroupId)) {
      setSelectedGroupId(response.groups[0]?.id ?? '')
    }
    if (!response.groups.some((group) => group.id === accessGroupId)) {
      setAccessGroupId(response.groups[0]?.id ?? '')
    }
  }

  async function submitNewAccessGroup() {
    const name = newAccessGroup.name.trim()
    if (!name) {
      setAccessError('Group name is required.')
      return
    }
    if (accessGroups.some((group) => group.name.trim().toLowerCase() === name.toLowerCase())) {
      setAccessError('An access group with this name already exists.')
      return
    }
    setAccessError(null)
    try {
      const response = await createAccessGroup({ ...newAccessGroup, name })
      setNewAccessGroup({ name: '', description: '', oktaGroupName: '' })
      await reloadAccessGroups()
      setSelectedGroupId(response.group.id)
      setAccessGroupId(response.group.id)
    } catch (error) {
      setAccessError(error instanceof Error ? error.message : 'Could not create access group.')
    }
  }

  async function submitAccessGroupSave(group: BackendAccessGroup) {
    const name = group.name.trim()
    if (!name) {
      setAccessError('Group name is required.')
      return
    }
    if (draftAccessGroups.some((item) => item.id !== group.id && item.name.trim().toLowerCase() === name.toLowerCase())) {
      setAccessError('An access group with this name already exists.')
      return
    }
    setAccessError(null)
    try {
      await updateAccessGroup(group.id, {
        name,
        description: group.description,
        oktaGroupName: group.oktaGroupName,
      })
      setEditingGroupId(null)
      await reloadAccessGroups()
    } catch (error) {
      setAccessError(error instanceof Error ? error.message : 'Could not save access group.')
    }
  }

  async function removeAccessGroup(groupId: string) {
    if (!window.confirm('Delete this access group and remove its workspace/design grants?')) return
    setAccessError(null)
    try {
      await deleteAccessGroup(groupId)
      await reloadAccessGroups()
      setGroupMembers([])
      setWorkspaceGroupAccess((current) => current.filter((item) => item.groupId !== groupId))
      setDesignGroupAccess((current) => current.filter((item) => item.groupId !== groupId))
    } catch (error) {
      setAccessError(error instanceof Error ? error.message : 'Could not delete access group.')
    }
  }

  async function toggleAccessGroupMember(userId: string, checked: boolean) {
    if (!selectedGroupId) return
    const next = checked
      ? Array.from(new Set([...groupMembers.map((member) => member.userId), userId]))
      : groupMembers.map((member) => member.userId).filter((id) => id !== userId)
    setAccessError(null)
    try {
      const response = await replaceAccessGroupMembers(selectedGroupId, next)
      setGroupMembers(response.members)
      await reloadAccessGroups()
    } catch (error) {
      setAccessError(error instanceof Error ? error.message : 'Could not update group membership.')
    }
  }

  async function submitAccessGrant() {
    if (!accessWorkspaceId) return
    if (accessSubject === 'user' && !accessUserId) return
    if (accessSubject === 'group' && !accessGroupId) return
    if (!accessGrantHasPermissions) {
      setAccessError('Select at least one permission before saving a grant.')
      return
    }
    setAccessError(null)
    setAccessLoading(true)
    try {
      if (accessScope === 'workspace') {
        if (accessSubject === 'user') {
          const response = await grantWorkspaceAccess(accessWorkspaceId, { userId: accessUserId, ...workspaceAccessDraft })
          setWorkspaceAccess((current) => {
            const existing = current.filter((item) => item.userId !== response.access.userId)
            return [...existing, response.access]
          })
        } else {
          const response = await grantWorkspaceGroupAccess(accessWorkspaceId, { groupId: accessGroupId, ...workspaceAccessDraft })
          setWorkspaceGroupAccess((current) => {
            const existing = current.filter((item) => item.groupId !== response.access.groupId)
            return [...existing, response.access]
          })
        }
      } else if (accessDesignId) {
        if (accessSubject === 'user') {
          const response = await grantDesignAccess(accessWorkspaceId, accessDesignId, { userId: accessUserId, ...designAccessDraft })
          setDesignAccess((current) => {
            const existing = current.filter((item) => item.userId !== response.access.userId)
            return [...existing, response.access]
          })
        } else {
          const response = await grantDesignGroupAccess(accessWorkspaceId, accessDesignId, { groupId: accessGroupId, ...designAccessDraft })
          setDesignGroupAccess((current) => {
            const existing = current.filter((item) => item.groupId !== response.access.groupId)
            return [...existing, response.access]
          })
        }
      }
      if (accessUserId) await refreshUserAccessSummary(accessUserId)
    } catch (error) {
      setAccessError(error instanceof Error ? error.message : 'Could not save access grant.')
    } finally {
      setAccessLoading(false)
    }
  }

  async function revokeGroupAccessGrant(groupId: string) {
    if (!accessWorkspaceId) return
    setAccessError(null)
    setAccessLoading(true)
    try {
      if (accessScope === 'workspace') {
        await revokeWorkspaceGroupAccess(accessWorkspaceId, groupId)
        setWorkspaceGroupAccess((current) => current.filter((item) => item.groupId !== groupId))
      } else if (accessDesignId) {
        await revokeDesignGroupAccess(accessWorkspaceId, accessDesignId, groupId)
        setDesignGroupAccess((current) => current.filter((item) => item.groupId !== groupId))
      }
      if (accessUserId) await refreshUserAccessSummary(accessUserId)
    } catch (error) {
      setAccessError(error instanceof Error ? error.message : 'Could not revoke group access grant.')
    } finally {
      setAccessLoading(false)
    }
  }

  async function revokeAccessGrant(userId: string) {
    if (!accessWorkspaceId) return
    setAccessError(null)
    setAccessLoading(true)
    try {
      if (accessScope === 'workspace') {
        await revokeWorkspaceAccess(accessWorkspaceId, userId)
        setWorkspaceAccess((current) => current.filter((item) => item.userId !== userId))
      } else if (accessDesignId) {
        await revokeDesignAccess(accessWorkspaceId, accessDesignId, userId)
        setDesignAccess((current) => current.filter((item) => item.userId !== userId))
      }
      await refreshUserAccessSummary(userId)
    } catch (error) {
      setAccessError(error instanceof Error ? error.message : 'Could not revoke access grant.')
    } finally {
      setAccessLoading(false)
    }
  }

  async function refreshUserAccessSummary(userId = accessUserId) {
    if (!userId) {
      setUserAccessSummary([])
      return
    }
    setUserAccessLoading(true)
    try {
      const response = await fetchUserAccessSummary(userId)
      setUserAccessSummary(response.access)
    } finally {
      setUserAccessLoading(false)
    }
  }

  return (
    <main className="admin-shell admin-center-shell">
      <section className="admin-command-bar">
        <div className="admin-command-copy">
          <p className="eyebrow">Enterprise administration</p>
          <h1>Admin console</h1>
          <p>Control identity, access, catalog governance, AI providers, and integrations from one command surface.</p>
        </div>
        <label className="admin-command-search">
          <Search size={18} />
          <input
            value={adminSearch}
            onChange={(event) => setAdminSearch(event.target.value)}
            placeholder="Search users, catalog, SSO, AI, integrations..."
          />
        </label>
      </section>
      {error ? <div className="home-error admin-page-error"><span>{error}</span></div> : null}

      <div className="admin-center-layout">
        <aside className="admin-section-nav" aria-label="Admin sections">
          {visibleAdminSections.map((section) => {
            const copy = adminSectionCopy[section]
            return (
              <button
                className={section === activeSection ? 'admin-section-button active' : 'admin-section-button'}
                key={section}
                type="button"
                onClick={() => onSelectSection(section)}
              >
                <AdminSectionIcon section={section} />
                <span>
                  <strong>{copy.label}</strong>
                  <small>{copy.group}</small>
                </span>
                {section === activeSection ? <CheckCircle2 size={15} /> : null}
              </button>
            )
          })}
        </aside>

        <section className="admin-section-content">
          <header className="admin-page-header">
            <span><AdminSectionIcon section={activeSection} /></span>
            <div>
              <p className="eyebrow">{activeSectionCopy.group}</p>
              <h2>{activeSectionCopy.label}</h2>
              <p>{activeSectionCopy.description}</p>
            </div>
          </header>

          {activeSection === 'overview' ? (
            <>
              <section className="admin-metrics" aria-label="Administration overview">
                <article>
                  <span>Total users</span>
                  <strong>{users.length}</strong>
                  <small>{activeUsers} active</small>
                </article>
                <article>
                  <span>Identity provider</span>
                  <strong>{providerLabel}</strong>
                  <small>{hasOktaCore ? 'Core settings present' : 'Configuration incomplete'}</small>
                </article>
                <article>
                  <span>Provisioning</span>
                  <strong>{signInDraft?.jitProvisioning ? 'JIT' : 'Manual'}</strong>
                  <small>{signInDraft?.jitProvisioning ? 'Users created on first login' : 'Admins add users manually'}</small>
                </article>
                <article>
                  <span>Analysis AI</span>
                  <strong>{aiDraft?.enabled ? 'Enabled' : 'Disabled'}</strong>
                  <small>{aiDraft?.provider ? `${aiDraft.provider} ${aiDraft.model}` : 'Central provider not configured'}</small>
                </article>
                <article>
                  <span>MCP server</span>
                  <strong>{mcpDraft?.enabled ? 'Enabled' : 'Disabled'}</strong>
                  <small>{mcpDraft ? `${enabledMCPCapabilities} configured capabilities` : 'Configuration not loaded'}</small>
                </article>
              </section>
              <section className="admin-dashboard-grid" aria-label="Administration status">
                <article className="admin-dashboard-panel">
                  <div className="admin-panel-heading compact">
                    <span><Activity size={18} /></span>
                    <div>
                      <strong>Needs attention</strong>
                      <p>Configuration items that affect enterprise readiness.</p>
                    </div>
                  </div>
                  <div className="admin-attention-list">
                    {dashboardAttention.map((item) => (
                      <button className={`admin-attention-row ${item.status}`} key={item.title} type="button" onClick={() => onSelectSection(item.section)}>
                        <span />
                        <div>
                          <strong>{item.title}</strong>
                          <small>{item.detail}</small>
                        </div>
                        <ChevronRight size={16} />
                      </button>
                    ))}
                  </div>
                </article>
                <article className="admin-dashboard-panel">
                  <div className="admin-panel-heading compact">
                    <span><ShieldCheck size={18} /></span>
                    <div>
                      <strong>Governance posture</strong>
                      <p>Current control model for users, workspaces, and designs.</p>
                    </div>
                  </div>
                  <div className="admin-governance-list">
                    <span><strong>{adminUsers}</strong> admin account{adminUsers === 1 ? '' : 's'} active</span>
                    <span>Workspace and design ACL guard layers are enforced by the backend.</span>
                    <span>Catalog updates are admin-managed to reduce duplicate shared-service registration.</span>
                  </div>
                </article>
              </section>
            </>
          ) : null}

          {activeSection === 'access' ? (
            <div className="admin-panel access-center-panel">
              <div className="admin-panel-heading">
                <span><ShieldCheck size={18} /></span>
                <div>
                  <strong>Access Center</strong>
                  <p>Grant workspace and design-level permissions without changing global product roles.</p>
                </div>
              </div>

              <div className="access-center-grid">
                <section className="access-control-card">
                  <div className="access-scope-switch" aria-label="Access scope">
                    <button className={accessScope === 'workspace' ? 'active' : ''} type="button" onClick={() => setAccessScope('workspace')}>
                      Workspace
                    </button>
                    <button className={accessScope === 'design' ? 'active' : ''} type="button" onClick={() => setAccessScope('design')}>
                      Design
                    </button>
                  </div>
                  <div className="access-selector-grid">
                    <label className="field">
                      <span>Workspace</span>
                      <select
                        value={accessWorkspaceId}
                        onChange={(event) => {
                          setAccessWorkspaceId(event.target.value)
                          setAccessDesignId('')
                        }}
                      >
                        {workspaces.map((workspace) => (
                          <option key={workspace.id} value={workspace.id}>
                            {workspace.name}
                          </option>
                        ))}
                      </select>
                    </label>
                    {accessScope === 'design' ? (
                      <label className="field">
                        <span>Design</span>
                        <select value={accessDesignId} onChange={(event) => setAccessDesignId(event.target.value)}>
                          {accessDesigns.map((design) => (
                            <option key={design.id} value={design.id}>
                              {design.name}
                            </option>
                          ))}
                        </select>
                      </label>
                    ) : null}
                  </div>
                  <div className="access-context-card">
                    <strong>{accessScope === 'workspace' ? activeAccessWorkspace?.name || 'Select workspace' : activeAccessDesign?.name || 'Select design'}</strong>
                    <span>
                      {accessScope === 'workspace'
                        ? 'Controls who can see this workspace, create designs, or manage grants.'
                        : 'Controls who can read, edit, comment, review, or manage this design.'}
                    </span>
                  </div>
                </section>

                <section className="access-control-card">
                  <div className="admin-section-title">
                    <span>Grant access</span>
                    <small>
                      {accessSubject === 'user'
                        ? selectedAccessUser?.email || 'Choose a user'
                        : selectedAccessGroup?.name || 'Choose a group'}
                    </small>
                  </div>
                  <div className="access-scope-switch compact" aria-label="Access subject">
                    <button className={accessSubject === 'user' ? 'active' : ''} type="button" onClick={() => setAccessSubject('user')}>
                      User
                    </button>
                    <button className={accessSubject === 'group' ? 'active' : ''} type="button" onClick={() => setAccessSubject('group')}>
                      Group
                    </button>
                  </div>
                  <label className="field">
                    <span>{accessSubject === 'user' ? 'User' : 'Group'}</span>
                    {accessSubject === 'user' ? (
                      <select value={accessUserId} onChange={(event) => setAccessUserId(event.target.value)}>
                        {grantableUsers.map((user) => (
                          <option key={user.id} value={user.id}>
                            {user.displayName} · {user.email}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <select value={accessGroupId} onChange={(event) => setAccessGroupId(event.target.value)}>
                        {accessGroups.map((group) => (
                          <option key={group.id} value={group.id}>
                            {group.name}{group.oktaGroupName ? ` · Okta: ${group.oktaGroupName}` : ''}
                          </option>
                        ))}
                      </select>
                    )}
                  </label>
                  <div className="permission-chip-grid">
                    {accessScope === 'workspace' ? (
                      <>
                        <label><input type="checkbox" checked={workspaceAccessDraft.canRead} onChange={(event) => setWorkspaceAccessDraft((current) => ({ ...current, canRead: event.target.checked }))} /> Read</label>
                        <label><input type="checkbox" checked={workspaceAccessDraft.canCreateDesign} onChange={(event) => setWorkspaceAccessDraft((current) => ({ ...current, canCreateDesign: event.target.checked }))} /> Create designs</label>
                        <label><input type="checkbox" checked={workspaceAccessDraft.canManage} onChange={(event) => setWorkspaceAccessDraft((current) => ({ ...current, canManage: event.target.checked }))} /> Manage</label>
                      </>
                    ) : (
                      <>
                        <label><input type="checkbox" checked={designAccessDraft.canRead} onChange={(event) => setDesignAccessDraft((current) => ({ ...current, canRead: event.target.checked }))} /> Read</label>
                        <label><input type="checkbox" checked={designAccessDraft.canEdit} onChange={(event) => setDesignAccessDraft((current) => ({ ...current, canEdit: event.target.checked }))} /> Edit</label>
                        <label><input type="checkbox" checked={designAccessDraft.canComment} onChange={(event) => setDesignAccessDraft((current) => ({ ...current, canComment: event.target.checked }))} /> Comment</label>
                        <label><input type="checkbox" checked={designAccessDraft.canReview} onChange={(event) => setDesignAccessDraft((current) => ({ ...current, canReview: event.target.checked }))} /> Review</label>
                        <label><input type="checkbox" checked={designAccessDraft.canManage} onChange={(event) => setDesignAccessDraft((current) => ({ ...current, canManage: event.target.checked }))} /> Manage</label>
                      </>
                    )}
                  </div>
                  {!accessGrantHasPermissions ? <p className="form-help danger">Select at least one permission. Empty grants are not saved.</p> : null}
                  {accessSubject === 'group' && !accessGroups.length ? (
                    <p className="form-help danger">Create a group before saving group-based grants.</p>
                  ) : null}
                  <button
                    className="primary-action full-width"
                    type="button"
                    disabled={
                      accessLoading ||
                      (accessSubject === 'user' ? !accessUserId : !accessGroupId) ||
                      !accessGrantHasPermissions ||
                      (accessScope === 'design' && !accessDesignId)
                    }
                    onClick={() => void submitAccessGrant()}
                  >
                    <ShieldCheck size={16} /> Save grant
                  </button>
                </section>

                <section className="access-control-card access-groups-card">
                  <div className="admin-section-title">
                    <span>Groups</span>
                    <small>Local groups can map to Okta group claim values.</small>
                  </div>
                  <div className="access-group-composer">
                    <label className="field">
                      <span>Group name</span>
                      <input value={newAccessGroup.name} onChange={(event) => setNewAccessGroup((current) => ({ ...current, name: event.target.value }))} />
                    </label>
                    <label className="field">
                      <span>Okta group claim value</span>
                      <input
                        placeholder="e.g. stratum-architects"
                        value={newAccessGroup.oktaGroupName}
                        onChange={(event) => setNewAccessGroup((current) => ({ ...current, oktaGroupName: event.target.value }))}
                      />
                    </label>
                    <label className="field">
                      <span>Description</span>
                      <input value={newAccessGroup.description} onChange={(event) => setNewAccessGroup((current) => ({ ...current, description: event.target.value }))} />
                    </label>
                    <button className="secondary-action" type="button" onClick={() => void submitNewAccessGroup()}>
                      <Users size={16} /> Create group
                    </button>
                  </div>

                  <div className="access-groups-layout">
                    <div className="access-group-list">
                      {draftAccessGroups.length ? draftAccessGroups.map((group) => {
                        const isEditing = editingGroupId === group.id
                        return (
                          <article className={`access-group-row ${selectedGroupId === group.id ? 'selected' : ''}`} key={group.id}>
                            {isEditing ? (
                              <div className="access-group-edit">
                                <input value={group.name} onChange={(event) => updateDraftAccessGroup(group, { name: event.target.value })} />
                                <input
                                  placeholder="Okta group claim value"
                                  value={group.oktaGroupName}
                                  onChange={(event) => updateDraftAccessGroup(group, { oktaGroupName: event.target.value })}
                                />
                                <input value={group.description} onChange={(event) => updateDraftAccessGroup(group, { description: event.target.value })} />
                              </div>
                            ) : (
                              <button type="button" onClick={() => setSelectedGroupId(group.id)}>
                                <span><Users size={16} /></span>
                                <div>
                                  <strong>{group.name}</strong>
                                  <small>{group.memberCount} member{group.memberCount === 1 ? '' : 's'}{group.oktaGroupName ? ` · Okta ${group.oktaGroupName}` : ''}</small>
                                </div>
                              </button>
                            )}
                            <div className="admin-row-actions compact">
                              {isEditing ? (
                                <button className="secondary-action" type="button" onClick={() => void submitAccessGroupSave(group)}>Save</button>
                              ) : (
                                <button className="secondary-action" type="button" onClick={() => setEditingGroupId(group.id)}>Edit</button>
                              )}
                              <button className="text-button danger" type="button" onClick={() => void removeAccessGroup(group.id)}>Delete</button>
                            </div>
                          </article>
                        )
                      }) : (
                        <div className="analysis-empty">
                          <strong>No groups yet</strong>
                          <span>Create groups for teams, Okta claims, and reusable permission bundles.</span>
                        </div>
                      )}
                    </div>

                    <div className="access-member-card">
                      <div className="admin-section-title">
                        <span>{selectedGroup?.name || 'Select a group'}</span>
                        <small>{selectedGroup?.oktaGroupName ? `Okta claim: ${selectedGroup.oktaGroupName}` : 'Manual membership'}</small>
                      </div>
                      <div className="access-member-list">
                        {grantableUsers.map((user) => (
                          <label key={user.id}>
                            <input
                              type="checkbox"
                              disabled={!selectedGroupId}
                              checked={selectedGroupUserIds.has(user.id)}
                              onChange={(event) => void toggleAccessGroupMember(user.id, event.target.checked)}
                            />
                            <span>{initialsFor(user.displayName || user.email)}</span>
                            <div>
                              <strong>{user.displayName}</strong>
                              <small>{user.email}</small>
                            </div>
                          </label>
                        ))}
                      </div>
                    </div>
                  </div>
                </section>
              </div>

              {accessError ? <div className="admin-inline-error">{accessError}</div> : null}

              <div className="access-grant-list">
                <div className="admin-section-title">
                  <span>Direct user grants</span>
                  <small>{accessRows.length} active</small>
                </div>
                {accessRows.length ? accessRows.map((grant) => {
                  const user = users.find((item) => item.id === grant.userId)
                  const permissions =
                    'canCreateDesign' in grant
                      ? [
                          grant.canRead ? 'Read' : '',
                          grant.canCreateDesign ? 'Create designs' : '',
                          grant.canManage ? 'Manage' : '',
                        ].filter(Boolean)
                      : [
                          grant.canRead ? 'Read' : '',
                          grant.canEdit ? 'Edit' : '',
                          grant.canComment ? 'Comment' : '',
                          grant.canReview ? 'Review' : '',
                          grant.canManage ? 'Manage' : '',
                        ].filter(Boolean)
                  return (
                    <article className="access-grant-row" key={`${grant.userId}-${grant.updatedAt}`}>
                      <div className="admin-user-identity">
                        <span>{initialsFor(user?.displayName || user?.email || grant.userId)}</span>
                        <div>
                          <strong>{user?.displayName || 'Unknown user'}</strong>
                          <small>{user?.email || grant.userId}</small>
                        </div>
                      </div>
                      <div className="access-permission-list">
                        {permissions.map((permission) => <span key={permission}>{permission}</span>)}
                      </div>
                      <button className="text-button danger" type="button" disabled={accessLoading} onClick={() => void revokeAccessGrant(grant.userId)}>
                        Revoke
                      </button>
                    </article>
                  )
                }) : (
                  <div className="analysis-empty">
                    <strong>No explicit grants yet</strong>
                    <span>Admins can add user-specific grants for this {accessScope}.</span>
                  </div>
                )}
              </div>

              <div className="access-grant-list">
                <div className="admin-section-title">
                  <span>Group grants</span>
                  <small>{groupAccessRows.length} active</small>
                </div>
                {groupAccessRows.length ? groupAccessRows.map((grant) => {
                  const group = accessGroups.find((item) => item.id === grant.groupId)
                  const permissions =
                    'canCreateDesign' in grant
                      ? [
                          grant.canRead ? 'Read' : '',
                          grant.canCreateDesign ? 'Create designs' : '',
                          grant.canManage ? 'Manage' : '',
                        ].filter(Boolean)
                      : [
                          grant.canRead ? 'Read' : '',
                          grant.canEdit ? 'Edit' : '',
                          grant.canComment ? 'Comment' : '',
                          grant.canReview ? 'Review' : '',
                          grant.canManage ? 'Manage' : '',
                        ].filter(Boolean)
                  return (
                    <article className="access-grant-row" key={`${grant.groupId}-${grant.updatedAt}`}>
                      <div className="admin-user-identity">
                        <span><Users size={18} /></span>
                        <div>
                          <strong>{group?.name || 'Unknown group'}</strong>
                          <small>{group?.oktaGroupName ? `Okta claim ${group.oktaGroupName}` : group?.description || grant.groupId}</small>
                        </div>
                      </div>
                      <div className="access-permission-list">
                        {permissions.map((permission) => <span key={permission}>{permission}</span>)}
                      </div>
                      <button className="text-button danger" type="button" disabled={accessLoading} onClick={() => void revokeGroupAccessGrant(grant.groupId)}>
                        Revoke
                      </button>
                    </article>
                  )
                }) : (
                  <div className="analysis-empty">
                    <strong>No group grants yet</strong>
                    <span>Group grants let Okta-backed teams inherit access without per-user rules.</span>
                  </div>
                )}
              </div>

              <section className="access-user-summary-card">
                <div className="admin-section-title">
                  <span>User grant explorer</span>
                  <small>{selectedAccessUser ? selectedAccessUser.email : 'Choose a user'}</small>
                </div>
                {userAccessLoading ? (
                  <div className="analysis-empty">
                    <strong>Loading grants</strong>
                    <span>Checking explicit workspace and design access for this user.</span>
                  </div>
                ) : userAccessSummary.length ? (
                  <div className="access-user-summary-list">
                    {userAccessSummary.map((grant) => (
                      <article className="access-user-summary-row" key={`${grant.scope}-${grant.workspaceId}-${grant.designId ?? 'workspace'}`}>
                        <div>
                          <strong>{grant.scope === 'workspace' ? grant.workspaceName : grant.designName}</strong>
                          <small>
                            {grant.scope === 'workspace'
                              ? 'Workspace grant'
                              : `${grant.workspaceName} / Design grant`}
                          </small>
                        </div>
                        {grant.source === 'group' ? <small className="access-source-label">Via {grant.groupName || grant.groupId}</small> : null}
                        <div className="access-permission-list">
                          {grant.permissions.map((permission) => <span key={permission}>{permission}</span>)}
                        </div>
                      </article>
                    ))}
                  </div>
                ) : (
                  <div className="analysis-empty">
                    <strong>No user-specific grants</strong>
                    <span>This user may still have access through role defaults, ownership, public designs, or workspace-level inheritance.</span>
                  </div>
                )}
              </section>
            </div>
          ) : null}

          {activeSection === 'workspaces' ? (
            <div className="admin-panel">
              <div className="admin-panel-heading">
                <span><Boxes size={18} /></span>
                <div>
                  <strong>Workspace governance</strong>
                  <p>Foundation for workspace ownership, default policies, and deletion readiness.</p>
                </div>
              </div>
              <div className="admin-policy-grid">
                <article>
                  <strong>Deletion guard</strong>
                  <span>Workspaces can only be deleted when empty and backend deletion is guarded for concurrent requests.</span>
                </article>
                <article>
                  <strong>Design creation</strong>
                  <span>Creating designs is now controlled by workspace-level ACL, not only broad product roles.</span>
                </article>
                <article>
                  <strong>Next UI step</strong>
                  <span>Add grant management tables once the access model stabilizes with real enterprise user groups.</span>
                </article>
              </div>
            </div>
          ) : null}

          {activeSection === 'security' ? (
            <div className="admin-panel">
              <div className="admin-panel-heading">
                <span><BadgeCheck size={18} /></span>
                <div>
                  <strong>Security and audit</strong>
                  <p>Security posture for auth, provider secrets, authorization, and future audit event streams.</p>
                </div>
              </div>
              <div className="admin-policy-grid">
                <article>
                  <strong>Token scope</strong>
                  <span>Backend APIs require authenticated sessions and role or ACL checks before sensitive operations.</span>
                </article>
                <article>
                  <strong>Secret handling</strong>
                  <span>AI and SSO secrets are written through admin-only endpoints and are not stored in browser local storage.</span>
                </article>
                <article>
                  <strong>Audit backlog</strong>
                  <span>Persisted admin and ACL change events are the next hardening layer before external compliance use.</span>
                </article>
              </div>
            </div>
          ) : null}

          {activeSection === 'settings' ? (
            <div className="admin-panel storage-admin-panel">
              <div className="admin-panel-heading">
                <span><Database size={18} /></span>
                <div>
                  <strong>Storage engine</strong>
                  <p>Run Stratum with cache-only stateless storage or attach a persistent database.</p>
                </div>
              </div>

              <div className="storage-status-grid">
                <article className={storageStatus?.stateless ? 'warning' : 'ready'}>
                  <strong>{storageStatus?.stateless ? 'Stateless cache' : 'Database'}</strong>
                  <span>Active storage mode</span>
                </article>
                <article className={storageStatus?.databaseConnected ? 'ready' : 'neutral'}>
                  <strong>{storageStatus?.databaseConnected ? 'Connected' : 'Not connected'}</strong>
                  <span>Database status</span>
                </article>
                <article>
                  <strong>{storageStatus?.cacheUserCount ?? 0}</strong>
                  <span>Cache users</span>
                </article>
                <article>
                  <strong>{storageStatus?.databaseUserCount ?? 0}</strong>
                  <span>Database users</span>
                </article>
              </div>

              {storageStatus?.stateless ? (
                <div className="storage-warning">
                  <Database size={18} />
                  <div>
                    <strong>Stateless mode is active</strong>
                    <span>{storageStatus.warning || 'Designs, users, and configuration live in process memory and are lost after backend restart.'}</span>
                  </div>
                </div>
              ) : (
                <div className="storage-ready">
                  <CheckCircle2 size={18} />
                  <div>
                    <strong>Persistent storage is active</strong>
                    <span>Backend reads and writes are using the configured database repository.</span>
                  </div>
                </div>
              )}

              <div className="storage-workbench">
                <section className="storage-card">
                  <div className="admin-section-title">
                    <span>Database connection</span>
                    <small>PostgreSQL connection string</small>
                  </div>
                  <div className="storage-current-database">
                    <span>Configured database</span>
                    <strong>
                      {storageStatus?.currentDatabaseUrl || storageStatus?.pendingDatabaseUrl || 'No database configured'}
                    </strong>
                    {storageStatus?.pendingDatabaseUrl ? <small>Pending migration</small> : null}
                  </div>
                  <label className="field">
                    <span>Database URL</span>
                    <input
                      type="password"
                      autoComplete="off"
                      placeholder="postgres://user:password@host:5432/stratum?sslmode=require"
                      value={storageDatabaseUrl}
                      onChange={(event) => setStorageDatabaseUrl(event.target.value)}
                    />
                  </label>
                  <div className="storage-actions">
                    <button className="secondary-action" type="button" disabled={storageAction !== 'idle'} onClick={() => void testStorageSettingsDatabase()}>
                      <BadgeCheck size={16} /> {storageAction === 'testing' ? 'Testing...' : 'Test database'}
                    </button>
                    <button className="primary-action" type="button" disabled={storageAction !== 'idle'} onClick={() => void configureStorageSettingsDatabase()}>
                      <Database size={16} /> {storageAction === 'configuring' ? 'Configuring...' : 'Configure database'}
                    </button>
                  </div>
                  <p className="form-help">
                    {storageStatus?.persistentConfigHint || 'Set DATABASE_URL in the deployment environment to keep database configuration across restarts.'}
                  </p>
                </section>

                <section className="storage-card">
                  <div className="admin-section-title">
                    <span>User migration</span>
                    <small>{storageStatus?.canMigrateUsers ? 'Ready' : 'Requires configured empty database'}</small>
                  </div>
                  <p className="form-help">
                    Admin onboarding can start in stateless mode. After database validation, migrate cached users into the database and switch the active repository.
                  </p>
                  <button
                    className="primary-action full-width"
                    type="button"
                    disabled={storageAction !== 'idle' || !storageStatus?.canMigrateUsers}
                    onClick={() => void migrateStorageSettingsUsers()}
                  >
                    <Upload size={16} /> {storageAction === 'migrating' ? 'Migrating...' : 'Migrate users to database'}
                  </button>
                  <button className="secondary-action full-width" type="button" disabled={storageAction !== 'idle'} onClick={() => void refreshStorageSettings()}>
                    <RotateCcw size={16} /> Refresh storage status
                  </button>
                </section>
              </div>

              {storageError ? <div className="admin-inline-error">{storageError}</div> : null}
              {storageMessage ? <div className="admin-inline-success">{storageMessage}</div> : null}

              <div className="admin-policy-grid">
                <article>
                  <strong>Stateless mode</strong>
                  <span>Uses the in-process repository for quick trials and first admin setup. It is intentionally marked unsafe for restart persistence.</span>
                </article>
                <article>
                  <strong>Database mode</strong>
                  <span>Uses the same repository contract behind the application, so the rest of Stratum is storage-engine agnostic.</span>
                </article>
                <article>
                  <strong>Migration scope</strong>
                  <span>This first migration moves users. Workspace and design migration can be added to the same storage engine without changing UI contracts.</span>
                </article>
              </div>
            </div>
          ) : null}

          <section className="admin-grid admin-section-grid" hidden={activeSection === 'overview' || activeSection === 'access' || activeSection === 'workspaces' || activeSection === 'security' || activeSection === 'settings'}>
        <div className="admin-panel users-admin-panel" hidden={activeSection !== 'users'}>
          <div className="admin-panel-heading">
            <span><UserCheck size={18} /></span>
            <div>
              <strong>User management</strong>
              <p>Add people, assign roles, and control account status.</p>
            </div>
          </div>
          <div className="admin-user-summary">
            <article><strong>{users.length}</strong><span>Total users</span></article>
            <article><strong>{activeUsers}</strong><span>Active</span></article>
            <article><strong>{adminUsers}</strong><span>Admins</span></article>
            <article><strong>{architectUsers + reviewerUsers}</strong><span>Review roles</span></article>
          </div>
          <div className="admin-add-user user-add-form">
            <Field label="Name" value={newUser.displayName} onChange={(displayName) => setNewUser((current) => ({ ...current, displayName }))} />
            <Field label="Email" value={newUser.email} onChange={(email) => setNewUser((current) => ({ ...current, email }))} />
            <label className="field">
              <span>Role</span>
              <select value={newUser.role} onChange={(event) => setNewUser((current) => ({ ...current, role: event.target.value }))}>
                <option value="member">Member</option>
                <option value="reviewer">Reviewer</option>
                <option value="architect">Architect</option>
                <option value="admin">Admin</option>
              </select>
            </label>
            <button
              className="primary-action"
              type="button"
              onClick={() => void submitNewUser()}
              disabled={!newUser.displayName.trim() || !newUser.email.trim()}
            >
              <UserPlus size={16} /> Add user
            </button>
          </div>
          <div className={`admin-password-policy ${localPasswordManaged ? '' : 'disabled'}`}>
            <ShieldCheck size={16} />
            <span>
              {localPasswordManaged
                ? 'Users set local passwords through one-time reset links. Admins do not handle user passwords directly.'
                : 'Local passwords are disabled. Password reset links are unavailable while SSO-only sign-in is active.'}
            </span>
          </div>
          {userError ? <div className="admin-inline-error">{userError}</div> : null}

          <div className="admin-role-strip polished" aria-label="Role model">
            <span><strong>Admin</strong><small>Platform control</small></span>
            <span><strong>Architect</strong><small>Create and evolve</small></span>
            <span><strong>Reviewer</strong><small>Review and comment</small></span>
            <span><strong>Member</strong><small>View workspace</small></span>
          </div>

          <section className="admin-user-directory">
            <div className="admin-section-title">
              <span>Directory</span>
              <small>{users.length} account{users.length === 1 ? '' : 's'}</small>
            </div>
          <div className="admin-user-list">
            {draftUsers.map((user) => (
              editingUserId === user.id ? (
                <article className="admin-user-row editing" key={user.id}>
                  <div className="admin-user-identity">
                    <span>{initialsFor(user.displayName || user.email)}</span>
                    <div>
                      <Field label="Name" value={user.displayName} onChange={(displayName) => updateDraftUser(user, { displayName })} />
                      <Field label="Email" value={user.email} onChange={(email) => updateDraftUser(user, { email })} />
                    </div>
                  </div>
                  <div className="admin-user-controls">
                    <label className="field">
                      <span>Role</span>
                      <select value={user.role} onChange={(event) => updateDraftUser(user, { role: event.target.value })}>
                        <option value="member">Member</option>
                        <option value="reviewer">Reviewer</option>
                        <option value="architect">Architect</option>
                        <option value="admin">Admin</option>
                      </select>
                    </label>
                    <label className="field">
                      <span>Status</span>
                      <select value={user.status} onChange={(event) => updateDraftUser(user, { status: event.target.value })}>
                        <option value="active">Active</option>
                        <option value="disabled">Disabled</option>
                      </select>
                    </label>
                  </div>
                  <div className="admin-row-actions">
                    <button
                      className="secondary-action compact-action"
                      type="button"
                      disabled={!localPasswordManaged || passwordResetLoading === user.id || user.status === 'disabled'}
                      onClick={() => void generateResetLink(user)}
                    >
                      {passwordResetLoading === user.id ? 'Generating...' : 'Reset link'}
                    </button>
                    <button className="secondary-action compact-action" type="button" onClick={() => void submitUserSave(user)}>
                      Save
                    </button>
                    <button
                      className="text-button"
                      type="button"
                      onClick={() => {
                        setDraftUsers(users)
                        setEditingUserId(null)
                        setUserError(null)
                      }}
                    >
                      Cancel
                    </button>
                  </div>
                </article>
              ) : (
                <article className="admin-user-row" key={user.id}>
                  <div className="admin-user-identity">
                    <span>{initialsFor(user.displayName || user.email)}</span>
                    <div>
                      <strong>{user.displayName}</strong>
                      <small>{user.email}</small>
                    </div>
                  </div>
                  <div className="admin-user-meta">
                    <span className={`admin-status-pill ${user.status}`}>{user.status}</span>
                    <span>{roleLabel(user.role)}</span>
                    <small>{user.passwordSet ? 'Password set' : 'Password not set'}</small>
                    <small>Last seen {formatCompactDateTime(user.lastSeenAt || user.updatedAt)}</small>
                  </div>
                  <div className="admin-user-meta secondary">
                    <span>Joined {formatCompactDateTime(user.createdAt)}</span>
                    <small>Updated {formatCompactDateTime(user.updatedAt)}</small>
                  </div>
                  <div className="admin-row-actions">
                    <button
                      className="secondary-action compact-action"
                      type="button"
                      disabled={!localPasswordManaged || passwordResetLoading === user.id || user.status === 'disabled'}
                      onClick={() => void generateResetLink(user)}
                    >
                      {passwordResetLoading === user.id ? 'Generating...' : 'Reset link'}
                    </button>
                    <button className="secondary-action compact-action" type="button" onClick={() => setEditingUserId(user.id)}>
                      Edit
                    </button>
                    <button className="text-button danger" type="button" disabled={user.id === activeUserId} onClick={() => onDeleteUser(user.id)}>
                      Delete
                    </button>
                  </div>
                  {passwordResetLinks[user.id] ? (
                    <div className="admin-reset-link">
                      <div>
                        <strong>Password reset link</strong>
                        <span>Expires {formatCompactDateTime(passwordResetLinks[user.id].expiresAt)}</span>
                      </div>
                      <input readOnly value={passwordResetLinks[user.id].resetLink} onFocus={(event) => event.currentTarget.select()} />
                      <button
                        className="secondary-action compact-action"
                        type="button"
                        onClick={() => void navigator.clipboard?.writeText(passwordResetLinks[user.id].resetLink)}
                      >
                        Copy
                      </button>
                    </div>
                  ) : null}
                </article>
              )
            ))}
          </div>
          </section>
        </div>

        <div className="admin-panel catalog-admin-panel" hidden={activeSection !== 'catalog'}>
          <div className="admin-panel-heading">
            <span><Boxes size={18} /></span>
            <div>
              <strong>Enterprise Catalog</strong>
              <p>Canonical services and infrastructure shared across architecture designs.</p>
            </div>
          </div>
          <div className="catalog-command-row">
            <div className="catalog-kpis">
              <article><strong>{catalogAssets.length}</strong><span>Catalog assets</span></article>
              <article><strong>{catalogAssets.reduce((count, asset) => count + asset.usedInDesignCount, 0)}</strong><span>Design links</span></article>
              <article><strong>{catalogAssets.filter((asset) => asset.criticality === 'critical' || asset.criticality === 'high').length}</strong><span>High criticality</span></article>
            </div>
            <label className="admin-command-search catalog-search">
              <Search size={17} />
              <input value={catalogAdminSearch} onChange={(event) => setCatalogAdminSearch(event.target.value)} placeholder="Search catalog assets..." />
            </label>
          </div>

          <div className="catalog-workbench">
            <section className="catalog-composer">
              <div className="admin-section-title">
                <span>Register asset</span>
                <small>Duplicate detection uses normalized names</small>
              </div>
              <Field label="Asset name" value={newAsset.name} onChange={(name) => setNewAsset((current) => ({ ...current, name }))} />
              <div className="admin-field-grid two">
                <label className="field">
                  <span>Type</span>
                  <select value={newAsset.type} onChange={(event) => setNewAsset((current) => ({ ...current, type: event.target.value }))}>
                    {componentCatalog.filter((item) => item.type !== 'note.sticky' && item.type !== 'frame.cloud' && item.type !== 'design.link').map((item) => (
                      <option key={item.type} value={item.type}>
                        {item.label}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="field">
                  <span>Criticality</span>
                  <select value={newAsset.criticality} onChange={(event) => setNewAsset((current) => ({ ...current, criticality: event.target.value }))}>
                    <option value="low">Low</option>
                    <option value="medium">Medium</option>
                    <option value="high">High</option>
                    <option value="critical">Critical</option>
                  </select>
                </label>
              </div>
              <Field label="Owner" value={newAsset.owner} onChange={(owner) => setNewAsset((current) => ({ ...current, owner }))} />
              <TextareaField
                label="Description"
                value={newAsset.description}
                onChange={(description) => setNewAsset((current) => ({ ...current, description }))}
              />
              {duplicateNewAsset ? (
                <div className="catalog-duplicate-warning">
                  Possible duplicate: <strong>{duplicateNewAsset.name}</strong>. Link existing assets instead of creating duplicates.
                </div>
              ) : null}
              {!duplicateNewAsset && newAssetSuggestions.length ? (
                <div className="catalog-suggestions">
                  <strong>Similar catalog assets</strong>
                  {newAssetSuggestions.slice(0, 4).map((asset) => (
                    <span className="catalog-suggestion readonly" key={asset.id}>
                      <span>{asset.name}</span>
                      <small>{asset.type} • normalized as {asset.normalizedName}</small>
                    </span>
                  ))}
                </div>
              ) : null}
              {assetError ? <div className="admin-inline-error">{assetError}</div> : null}
              <button className="primary-action full-width" type="button" disabled={!newAsset.name.trim() || Boolean(duplicateNewAsset)} onClick={() => void submitNewAsset()}>
                <FilePlus2 size={16} /> Add catalog asset
              </button>
            </section>

            <section className="catalog-library">
              <div className="admin-section-title">
                <span>Canonical assets</span>
                <small>{filteredDraftAssets.length} visible</small>
              </div>
              <div className="catalog-admin-list modern">
            {filteredDraftAssets.length ? filteredDraftAssets.map((asset) => (
              editingAssetId === asset.id ? (
                <article className="catalog-admin-row editing" key={asset.id}>
                  <Field label="Name" value={asset.name} onChange={(name) => updateDraftAsset(asset, { name })} />
                  <label className="field">
                    <span>Type</span>
                    <select value={asset.type} onChange={(event) => updateDraftAsset(asset, { type: event.target.value })}>
                      {componentCatalog.filter((item) => item.type !== 'note.sticky' && item.type !== 'frame.cloud' && item.type !== 'design.link').map((item) => (
                        <option key={item.type} value={item.type}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <Field label="Owner" value={asset.owner} onChange={(owner) => updateDraftAsset(asset, { owner })} />
                  <label className="field">
                    <span>Criticality</span>
                    <select value={asset.criticality} onChange={(event) => updateDraftAsset(asset, { criticality: event.target.value })}>
                      <option value="low">Low</option>
                      <option value="medium">Medium</option>
                      <option value="high">High</option>
                      <option value="critical">Critical</option>
                    </select>
                  </label>
                  <TextareaField label="Description" value={asset.description} onChange={(description) => updateDraftAsset(asset, { description })} />
                  <div className="admin-row-actions">
                    <button className="secondary-action compact-action" type="button" onClick={() => void submitAssetSave(asset)}>
                      Save
                    </button>
                    <button
                      className="text-button"
                      type="button"
                      onClick={() => {
                        setDraftAssets(catalogAssets)
                        setEditingAssetId(null)
                        setAssetError(null)
                      }}
                    >
                      Cancel
                    </button>
                  </div>
                </article>
              ) : (
                <article className="catalog-admin-row modern" key={asset.id}>
                  <span className="catalog-type-icon"><AdminSectionIcon section="catalog" size={18} /></span>
                  <div>
                    <strong>{asset.name}</strong>
                    <small>{getCatalogItem(asset.type as ComponentType)?.label || asset.type} • {asset.owner || 'No owner'}</small>
                  </div>
                  <span className={`admin-status-pill ${asset.criticality}`}>{asset.criticality}</span>
                  <span>{asset.usedInDesignCount} design{asset.usedInDesignCount === 1 ? '' : 's'}</span>
                  <div className="admin-row-actions">
                    <button className="secondary-action compact-action" type="button" onClick={() => setEditingAssetId(asset.id)}>
                      Edit
                    </button>
                    <button className="text-button danger" type="button" disabled={asset.usedInDesignCount > 0} onClick={() => void onDeleteCatalogAsset(asset.id)}>
                      Delete
                    </button>
                  </div>
                </article>
              )
            )) : (
              <div className="analysis-empty">
                <strong>No catalog assets yet</strong>
                <span>Users can promote design components, and admins can curate them here.</span>
              </div>
            )}
              </div>
            </section>
          </div>
        </div>

        <div className="admin-panel" hidden={activeSection !== 'identity'}>
          <div className="admin-panel-heading">
            <span><LockKeyhole size={18} /></span>
            <div>
              <strong>Sign-in options</strong>
              <p>Configure local access, Okta OIDC, and group-based role mapping.</p>
            </div>
          </div>
          {signInDraft ? (
            <div className="signin-form">
              <div className="auth-mode-grid">
                <label className={signInDraft.localPasswordEnabled ? 'auth-mode-card active' : 'auth-mode-card'}>
                  <input
                    type="checkbox"
                    checked={signInDraft.localPasswordEnabled}
                    onChange={(event) => setSignInDraft({ ...signInDraft, localPasswordEnabled: event.target.checked })}
                  />
                  <KeyRound size={18} />
                  <strong>Local password</strong>
                  <span>Keep direct sign-in available for controlled deployments.</span>
                </label>
                <label className={signInDraft.ssoEnabled ? 'auth-mode-card active' : 'auth-mode-card'}>
                  <input
                    type="checkbox"
                    checked={signInDraft.ssoEnabled}
                    onChange={(event) => setSignInDraft({ ...signInDraft, ssoEnabled: event.target.checked })}
                  />
                  <Globe2 size={18} />
                  <strong>Okta SSO</strong>
                  <span>Route users through enterprise identity and group claims.</span>
                </label>
              </div>

              <div className="admin-form-section">
                <div className="admin-section-title">
                  <span>Provider</span>
                  <small>OIDC web application</small>
                </div>
                <div className="admin-field-grid two">
                  <Field label="Okta domain" value={signInDraft.oktaDomain} onChange={(oktaDomain) => setSignInDraft({ ...signInDraft, oktaDomain })} />
                  <Field label="Issuer" value={signInDraft.issuer} onChange={(issuer) => setSignInDraft({ ...signInDraft, issuer })} />
                </div>
                <div className="admin-field-grid two">
                  <Field label="Client ID" value={signInDraft.clientId} onChange={(clientId) => setSignInDraft({ ...signInDraft, clientId })} />
                  <Field label={signInDraft.clientSecretSet ? 'Client secret (set)' : 'Client secret'} value={signInDraft.clientSecret ?? ''} onChange={(clientSecret) => setSignInDraft({ ...signInDraft, clientSecret })} />
                </div>
              </div>

              <div className="admin-form-section">
                <div className="admin-section-title">
                  <span>Redirects</span>
                  <small>Must match Okta exactly</small>
                </div>
                <Field label="Sign-in redirect URI" value={signInDraft.redirectUri} onChange={(redirectUri) => setSignInDraft({ ...signInDraft, redirectUri })} />
                <Field label="Sign-out redirect URI" value={signInDraft.postLogoutRedirectUri} onChange={(postLogoutRedirectUri) => setSignInDraft({ ...signInDraft, postLogoutRedirectUri })} />
              </div>

              <div className="admin-form-section">
                <div className="admin-section-title">
                  <span>Claims and role mapping</span>
                  <small>Used for enterprise access control</small>
                </div>
                <div className="admin-field-grid two">
                  <Field label="Scopes" value={signInDraft.scopes} onChange={(scopes) => setSignInDraft({ ...signInDraft, scopes })} />
                  <Field label="Groups claim" value={signInDraft.groupsClaim} onChange={(groupsClaim) => setSignInDraft({ ...signInDraft, groupsClaim })} />
                </div>
                <div className="admin-field-grid two">
                  <Field label="Admin group" value={signInDraft.adminGroup} onChange={(adminGroup) => setSignInDraft({ ...signInDraft, adminGroup })} />
                  <Field label="Reviewer group" value={signInDraft.reviewerGroup} onChange={(reviewerGroup) => setSignInDraft({ ...signInDraft, reviewerGroup })} />
                </div>
                <label className="toggle-row enterprise-toggle">
                  <input
                    type="checkbox"
                    checked={signInDraft.jitProvisioning}
                    onChange={(event) => setSignInDraft({ ...signInDraft, jitProvisioning: event.target.checked })}
                  />
                  <span>
                    <strong>Just-in-time provisioning</strong>
                    <small>Create allowed users when they first authenticate with SSO.</small>
                  </span>
                </label>
              </div>

              <div className="okta-guidance">
                <strong>Okta setup notes</strong>
                <span>Create an OIDC Web Application in Okta, use Authorization Code flow, register exact sign-in/sign-out redirect URIs, then copy Client ID, Client Secret, and issuer. Include openid profile email; add groups when group-based role mapping is configured.</span>
              </div>
              <button className="primary-action full-width" type="button" onClick={() => onSaveSignIn(signInDraft)}>
                Save sign-in settings
              </button>
            </div>
          ) : (
            <div className="analysis-empty">
              <strong>Loading sign-in settings</strong>
              <span>Admin settings will appear once the backend responds.</span>
            </div>
          )}
        </div>

        <div className="admin-panel ai-admin-panel" hidden={activeSection !== 'ai'}>
          <div className="admin-panel-heading">
            <span><Sparkles size={18} /></span>
            <div>
              <strong>AI analysis provider</strong>
              <p>Manage the centrally approved model used for design review synthesis.</p>
            </div>
          </div>
          {aiDraft ? (
            <div className="signin-form">
              <div className="admin-ai-status">
                <span className={aiDraft.enabled && aiDraft.apiKeySet ? 'admin-status-pill active' : 'admin-status-pill disabled'}>
                  {aiStatus}
                </span>
                <small>
                  {aiVerifiedAt
                    ? `Verified ${formatCompactDateTime(aiVerifiedAt)}`
                    : aiDraft.apiKeySet
                      ? 'Key stored, verification pending'
                      : 'No provider key stored'}
                </small>
              </div>

              <label className="toggle-row enterprise-toggle">
                <input
                  type="checkbox"
                  checked={aiDraft.enabled}
                  onChange={(event) => setAIDraft({ ...aiDraft, enabled: event.target.checked })}
                />
                <span>
                  <strong>Use AI for analysis synthesis</strong>
                  <small>Deterministic math always runs first; AI adds judgement, risks, and recommendations.</small>
                </span>
              </label>

              <div className="admin-form-section">
                <div className="admin-section-title">
                  <span>Provider</span>
                  <small>Stored in backend configuration, never browser local storage</small>
                </div>
                <div className="admin-field-grid two">
                  <label className="field">
                    <span>Provider</span>
                    <select
                      value={aiDraft.provider}
                      onChange={(event) => {
                        const provider = event.target.value
                        const defaultModel =
                          provider === 'anthropic' ? 'claude-3-5-sonnet-latest' : provider === 'openai' ? 'gpt-4.1' : aiDraft.model
                        setAIDraft({ ...aiDraft, provider, model: defaultModel })
                      }}
                    >
                      <option value="openai">OpenAI</option>
                      <option value="anthropic">Anthropic</option>
                      <option value="openrouter">OpenRouter</option>
                      <option value="custom">OpenAI-compatible</option>
                    </select>
                  </label>
                  <Field label="Model" value={aiDraft.model} onChange={(model) => setAIDraft({ ...aiDraft, model })} />
                </div>
                <Field label="Base URL" value={aiDraft.baseUrl} onChange={(baseUrl) => setAIDraft({ ...aiDraft, baseUrl })} />
                <Field
                  label={aiDraft.apiKeySet ? 'API key (stored)' : 'API key'}
                  type="password"
                  value={aiDraft.apiKey ?? ''}
                  onChange={(apiKey) => setAIDraft({ ...aiDraft, apiKey })}
                />
              </div>

              <div className="okta-guidance">
                <strong>Analysis workflow</strong>
                <span>
                  Stratum first scores requirements, topology, traffic, consistency, availability, and security with deterministic checks.
                  The configured model then receives that evidence plus the structured design and must return UI-safe JSON.
                </span>
              </div>

              <button className="primary-action full-width" type="button" onClick={() => onSaveAIProvider(aiDraft)}>
                Save and verify AI provider
              </button>
            </div>
          ) : (
            <div className="analysis-empty">
              <strong>Loading AI provider</strong>
              <span>Central analysis settings will appear once the backend responds.</span>
            </div>
          )}
        </div>

        <div className="admin-panel mcp-admin-panel" hidden={activeSection !== 'integrations'}>
          <div className="admin-panel-heading">
            <span><Server size={18} /></span>
            <div>
              <strong>MCP readiness</strong>
              <p>Prepare controlled MCP access for catalog, design, and analysis tools.</p>
            </div>
          </div>
          {mcpDraft ? (
            <div className="signin-form">
              <div className="admin-ai-status">
                <span className={mcpDraft.enabled ? 'admin-status-pill active' : 'admin-status-pill disabled'}>
                  {mcpDraft.enabled ? 'MCP configured' : 'MCP disabled'}
                </span>
                <small>{enabledMCPCapabilities} capabilities selected</small>
              </div>

              <label className="toggle-row enterprise-toggle">
                <input
                  type="checkbox"
                  checked={mcpDraft.enabled}
                  onChange={(event) => setMCPDraft({ ...mcpDraft, enabled: event.target.checked })}
                />
                <span>
                  <strong>Enable MCP configuration</strong>
                  <small>Stores deployment intent. The MCP server remains gated until server runtime support is added.</small>
                </span>
              </label>

              <div className="admin-form-section">
                <div className="admin-section-title">
                  <span>Endpoint</span>
                  <small>Advertised server path for future MCP clients</small>
                </div>
                <Field label="Endpoint path" value={mcpDraft.endpointPath} onChange={(endpointPath) => setMCPDraft({ ...mcpDraft, endpointPath })} />
              </div>

              <div className="admin-form-section">
                <div className="admin-section-title">
                  <span>Capabilities</span>
                  <small>Write actions stay limited until RBAC and audit events are complete</small>
                </div>
                <div className="mcp-capability-grid">
                  {[
                    ['readCatalog', 'Read catalog', 'Expose canonical components and metadata.'],
                    ['readDesigns', 'Read designs', 'Expose structured design documents.'],
                    ['createDraftDesign', 'Create draft design', 'Allow AI tools to propose draft architectures.'],
                    ['runAnalysis', 'Run analysis', 'Trigger deterministic and AI analysis suites.'],
                    ['fetchImpactReport', 'Fetch impact report', 'Reserved for cross-design impact visualization.'],
                  ].map(([key, label, description]) => (
                    <label className="auth-mode-card compact" key={key}>
                      <input
                        type="checkbox"
                        checked={Boolean(mcpDraft[key as keyof BackendMCPConfig])}
                        onChange={(event) => setMCPDraft({ ...mcpDraft, [key]: event.target.checked })}
                      />
                      <strong>{label}</strong>
                      <span>{description}</span>
                    </label>
                  ))}
                </div>
              </div>

              <label className="toggle-row enterprise-toggle">
                <input
                  type="checkbox"
                  checked={mcpDraft.requireAdminConsent}
                  onChange={(event) => setMCPDraft({ ...mcpDraft, requireAdminConsent: event.target.checked })}
                />
                <span>
                  <strong>Require admin consent</strong>
                  <small>External clients must be explicitly approved before accessing configured capabilities.</small>
                </span>
              </label>

              <div className="okta-guidance">
                <strong>Security posture</strong>
                <span>MCP write access is intentionally not active yet. The backend now stores enterprise configuration so runtime support can be added behind RBAC, audit logging, and scoped client credentials.</span>
              </div>

              <button className="primary-action full-width" type="button" onClick={() => onSaveMCP(mcpDraft)}>
                Save MCP settings
              </button>
            </div>
          ) : (
            <div className="analysis-empty">
              <strong>Loading MCP settings</strong>
              <span>MCP readiness settings will appear once the backend responds.</span>
            </div>
          )}
        </div>
          </section>
        </section>
      </div>
    </main>
  )
}
