import {
  lazy,
  Suspense,
  type CSSProperties,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import {
  Activity,
  BookOpen,
  Boxes,
  Braces,
  Bell,
  BadgeCheck,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronsLeftRight,
  Cloud,
  Copy,
  FilePlus2,
  Database,
  Download,
  Eye,
  FolderPlus,
  Globe2,
  Image as ImageIcon,
  KeyRound,
  LayoutDashboard,
  LockKeyhole,
  LogOut,
  MessageSquare,
  MessageSquarePlus,
  Monitor,
  Pencil,
  Plus,
  RotateCcw,
  Route,
  Save,
  Search,
  Server,
  Settings,
  Send,
  ShieldCheck,
  Sparkles,
  SkipBack,
  SkipForward,
  Trash2,
  Upload,
  Users,
  UserCheck,
  UserPlus,
} from 'lucide-react'
import { componentCatalog, getCatalogItem } from './catalog'
import {
  adminSectionCopy,
  adminSectionOrder,
  defaultWorkspace,
  parseRoute,
  routePath,
  type AdminSection,
  type AppRoute,
} from './app/navigation'
import {
  buildPrimaryJourney,
  catalogAssetUsageLabel,
  isBackendOfflineMessage,
  isPotentialCatalogMatch,
  normalizedCatalogLabel,
  stripHtml,
  versionStatusLabel,
} from './app/designUtils'
import { formatCompactDateTime, initialsFor, roleLabel, userDisplayName } from './app/format'
import { AppNavbar } from './components/AppNavbar'
import { FirstAdminOnboarding, LoginScreen } from './components/AuthScreens'
import { CanvasErrorBoundary } from './components/CanvasErrorBoundary'
import { CatalogGlyph, isCatalogComponentType } from './components/CatalogGlyph'
import { ReviewWorkspace, RequestReviewModal, NotificationsModal } from './components/CollaborationPanels'
import { CloneDesignModal, ConfirmDeleteModal, type DeleteTarget } from './components/DesignLifecycleModals'
import { Field, NumberField, SelectField, TextareaField } from './components/FormFields'
import { HomeScreen } from './components/HomeScreen'
import { ComponentInspector, ConnectorInspector, StructuredView } from './components/InspectorPanels'
import { StatelessModeBanner } from './components/StatelessModeBanner'
import {
  analyzeDesign,
  createCatalogAsset,
  createAccessGroup,
  createDesign,
  createDesignComment,
  createDesignDoc,
  createDesignVersion,
  createAdminUser,
  createFirstAdmin,
  createWorkspace,
  deleteAdminUser,
  deleteAccessGroup,
  deleteCatalogAsset,
  deleteDesign as deleteDesignFromBackend,
  deleteDesignDoc as deleteDesignDocFromBackend,
  deleteDesignVersion as deleteDesignVersionFromBackend,
  deleteWorkspace as deleteWorkspaceFromBackend,
  configureStorageDatabase,
  fetchDesignComments,
  fetchDesignDocs,
  fetchDesignReviews,
  fetchDesignVersions,
  fetchNotifications,
  fetchProfile,
  fetchAIProviderConfig,
  fetchAccessGroupMembers,
  fetchAccessGroups,
  fetchAdminStorageStatus,
  fetchMCPConfig,
  fetchCatalogAssets,
  fetchDesignAccess,
  fetchDesignGroupAccess,
  fetchSetupStatus,
  fetchSignInConfig,
  fetchUserAccessSummary,
  fetchUsers,
  fetchWorkspaceAccess,
  fetchWorkspaceGroupAccess,
  fetchWorkspaceDesign,
  fetchWorkspaceDesigns,
  fetchWorkspaces,
  grantDesignAccess,
  grantDesignGroupAccess,
  grantWorkspaceAccess,
  grantWorkspaceGroupAccess,
  login,
  logout,
  markNotificationRead,
  migrateStorageUsers,
  requestDesignReview,
  replaceAccessGroupMembers,
  revokeDesignAccess,
  revokeDesignGroupAccess,
  revokeWorkspaceAccess,
  revokeWorkspaceGroupAccess,
  saveDesignDocument as saveDesignDocumentToBackend,
  setInitialAdminPassword,
  testStorageDatabase,
  updateDesignReview,
  updateDesignVersionStatus,
  updateAccessGroup,
  updateDesignDoc,
  updateDesignMetadata,
  updateAdminUser,
  updateAIProviderConfig,
  updateCatalogAsset,
  updateMCPConfig,
  updateSignInConfig,
  type BackendAccessGroup,
  type BackendAccessGroupMember,
  type BackendAIProviderConfig,
  type BackendCatalogAsset,
  type BackendDesignAccess,
  type BackendDesignComment,
  type BackendDesignDoc,
  type BackendDesignGroupAccess,
  type BackendDesignReviewRequest,
  type BackendMCPConfig,
  type BackendNotification,
  type BackendProfile,
  type BackendSignInConfig,
  type BackendStorageStatus,
  type BackendDesignVersion,
  type BackendDesignVersionStatus,
  type BackendUserAccessSummary,
  type BackendUser,
  type BackendWorkspaceAccess,
  type BackendWorkspaceGroupAccess,
  type DesignAnalysisReport,
} from './backendApi'
import { useBackendDesignSync, type BackendWorkspace } from './backendSync'
import { ReactFlowCanvasProvider } from './canvas/ReactFlowCanvasProvider'
import { createComponent, createEmptyDesign, createEmptyRequirementBrief, toExportableDesign, touchDesign } from './designModel'
import { clearSavedDesign, loadDesignDocument, saveDesignDocument } from './storage'
import type {
  ComponentType,
  ConnectorType,
  DesignComponent,
  DesignConnector,
  DesignDocument,
  DesignJourney,
  RedisMetadata,
  RequirementBrief,
} from './types'
import type { BackendDesign } from './backendSync'

type CanvasMode = 'design' | 'view' | 'comment' | 'journey'
type ContextPanel = 'inspector' | 'requirements' | 'journey' | 'review'

const RichTextDocEditor = lazy(() =>
  import('./components/RichTextDocEditor').then((module) => ({ default: module.RichTextDocEditor })),
)

type AIConnectionMetadata = BackendAIProviderConfig

type PendingConnector = {
  fromComponentId: string
  toComponentId: string
}

type CanvasClipboard = {
  components: DesignComponent[]
  connectors: DesignConnector[]
  copiedAt: string
}

const connectorTypeOptions: Array<{ value: ConnectorType; label: string; description: string }> = [
  { value: 'synchronous', label: 'Synchronous', description: 'Request/response call on the critical path.' },
  { value: 'asynchronous_event', label: 'Async event', description: 'Event or message processed outside the request path.' },
  { value: 'batch_transfer', label: 'Batch transfer', description: 'Scheduled or bulk movement of data.' },
  { value: 'cache_read', label: 'Cache read', description: 'Reads from a cache or derived store.' },
  { value: 'cache_write', label: 'Cache write', description: 'Writes, invalidates, or warms cached data.' },
  { value: 'model_call', label: 'Model call', description: 'Calls an AI model or inference service.' },
  { value: 'observability_signal', label: 'Observability signal', description: 'Metrics, logs, traces, or audit signals.' },
]

export function App() {
  const initialRoute = parseRoute(window.location.pathname)
  const [route, setRoute] = useState<AppRoute>(initialRoute)
  const [view, setView] = useState<'home' | 'canvas' | 'admin'>(initialRoute.screen === 'design' ? 'canvas' : initialRoute.screen === 'admin' ? 'admin' : 'home')
  const [activeWorkspaceId, setActiveWorkspaceId] = useState(
    initialRoute.screen === 'workspace' || initialRoute.screen === 'design' ? initialRoute.workspaceId : 'guest-workspace',
  )
  const [selectedDesignId, setSelectedDesignId] = useState(
    initialRoute.screen === 'design' ? initialRoute.designId : null,
  )
  const [homeProfile, setHomeProfile] = useState<BackendProfile | null>(null)
  const [users, setUsers] = useState<BackendUser[]>([])
  const [activeUserId, setActiveUserId] = useState('')
  const [authenticated, setAuthenticated] = useState(false)
  const [authChecked, setAuthChecked] = useState(false)
  const [setupRequired, setSetupRequired] = useState(false)
  const [passwordSetupRequired, setPasswordSetupRequired] = useState(false)
  const [setupAction, setSetupAction] = useState<'idle' | 'saving' | 'error'>('idle')
  const [authAction, setAuthAction] = useState<'idle' | 'saving' | 'error'>('idle')
  const [authError, setAuthError] = useState<string | null>(null)
  const [adminError, setAdminError] = useState<string | null>(null)
  const [storageStatus, setStorageStatus] = useState<BackendStorageStatus | null>(null)
  const [signInConfig, setSignInConfig] = useState<BackendSignInConfig | null>(null)
  const [mcpConfig, setMCPConfig] = useState<BackendMCPConfig | null>(null)
  const [catalogAssets, setCatalogAssets] = useState<BackendCatalogAsset[]>([])
  const [catalogRailMode, setCatalogRailMode] = useState<'blocks' | 'catalog'>('blocks')
  const [catalogSearch, setCatalogSearch] = useState('')
  const [catalogTypeFilter, setCatalogTypeFilter] = useState('all')
  const [notifications, setNotifications] = useState<BackendNotification[]>([])
  const [notificationsOpen, setNotificationsOpen] = useState(false)
  const [homeWorkspaces, setHomeWorkspaces] = useState<BackendWorkspace[]>([defaultWorkspace])
  const [homeDesigns, setHomeDesigns] = useState<BackendDesign[]>([])
  const [newWorkspaceName, setNewWorkspaceName] = useState('')
  const [homeError, setHomeError] = useState<string | null>(null)
  const [homeAction, setHomeAction] = useState<'workspace' | 'design' | 'delete-workspace' | 'delete-design' | 'delete-version' | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null)
  const [cloneTarget, setCloneTarget] = useState<BackendDesign | null>(null)
  const [cloneVersions, setCloneVersions] = useState<BackendDesignVersion[]>([])
  const [cloneVersionId, setCloneVersionId] = useState('current')
  const [cloneTitle, setCloneTitle] = useState('')
  const [cloneState, setCloneState] = useState<'idle' | 'loading' | 'cloning' | 'error'>('idle')
  const [cloneError, setCloneError] = useState<string | null>(null)
  const [deletedDesignIds, setDeletedDesignIds] = useState<Set<string>>(() => new Set())
  const [newDesignBriefOpen, setNewDesignBriefOpen] = useState(false)
  const [newDesignTitle, setNewDesignTitle] = useState('Untitled system design')
  const [newDesignBrief, setNewDesignBrief] = useState<RequirementBrief>(() => createEmptyRequirementBrief())
  const [saveState, setSaveState] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle')
  const [versionState, setVersionState] = useState<'idle' | 'loading' | 'saving' | 'saved' | 'error'>('idle')
  const [designVersions, setDesignVersions] = useState<BackendDesignVersion[]>([])
  const [saveDialogOpen, setSaveDialogOpen] = useState(false)
  const [saveVersionMode, setSaveVersionMode] = useState<'override' | 'new'>('override')
  const [saveRemarks, setSaveRemarks] = useState('')
  const [design, setDesign] = useState<DesignDocument>(() => touchDesign(loadDesignDocument() ?? createEmptyDesign()))
  const [designName, setDesignName] = useState(() => loadDesignDocument()?.title ?? 'Untitled system design')
  const [selectedComponentId, setSelectedComponentId] = useState<string | null>(null)
  const [selectedConnectorId, setSelectedConnectorId] = useState<string | null>(null)
  const [selectedComponentIds, setSelectedComponentIds] = useState<Set<string>>(() => new Set())
  const [selectedConnectorIds, setSelectedConnectorIds] = useState<Set<string>>(() => new Set())
  const [canvasClipboard, setCanvasClipboard] = useState<CanvasClipboard | null>(null)
  const pasteCountRef = useRef(0)
  const [pendingConnector, setPendingConnector] = useState<PendingConnector | null>(null)
  const [rightRailOpen, setRightRailOpen] = useState(true)
  const [contextPanel, setContextPanel] = useState<ContextPanel>('requirements')
  const [toolbarExpanded, setToolbarExpanded] = useState(false)
  const [canvasMode, setCanvasMode] = useState<CanvasMode>('design')
  const [activeJourneyId, setActiveJourneyId] = useState<string | null>(null)
  const [activeJourneyStepIndex, setActiveJourneyStepIndex] = useState(0)
  const [activeTextDocumentId, setActiveTextDocumentId] = useState<string | null>(null)
  const [designDocs, setDesignDocs] = useState<BackendDesignDoc[]>([])
  const [dirtyDocIds, setDirtyDocIds] = useState<Set<string>>(() => new Set())
  const [docsSaveState, setDocsSaveState] = useState<'idle' | 'loading' | 'saving' | 'saved' | 'error'>('idle')
  const [docsError, setDocsError] = useState<string | null>(null)
  const [analysisState, setAnalysisState] = useState<'idle' | 'running' | 'ready' | 'error'>('idle')
  const [analysisReport, setAnalysisReport] = useState<DesignAnalysisReport | null>(null)
  const [analysisError, setAnalysisError] = useState<string | null>(null)
  const [analysisModalOpen, setAnalysisModalOpen] = useState(false)
  const [docsModalOpen, setDocsModalOpen] = useState(false)
  const [comments, setComments] = useState<BackendDesignComment[]>([])
  const [reviews, setReviews] = useState<BackendDesignReviewRequest[]>([])
  const [commentDraft, setCommentDraft] = useState('')
  const [reviewSummaryDrafts, setReviewSummaryDrafts] = useState<Record<string, string>>({})
  const [requestReviewOpen, setRequestReviewOpen] = useState(false)
  const [requestReviewVersion, setRequestReviewVersion] = useState<BackendDesignVersion | null>(null)
  const [versionPreview, setVersionPreview] = useState<{ version: BackendDesignVersion; document: DesignDocument } | null>(null)
  const [collaborationState, setCollaborationState] = useState<'idle' | 'loading' | 'saving' | 'error'>('idle')
  const [collaborationError, setCollaborationError] = useState<string | null>(null)
  const [collaborationOffline, setCollaborationOffline] = useState(false)
  const [aiConnection, setAIConnection] = useState<AIConnectionMetadata | null>(null)
  const [copilotOpen, setCopilotOpen] = useState(false)
  const [visionOpen, setVisionOpen] = useState(false)
  const designRef = useRef(design)
  const designNameRef = useRef(designName)
  const autosaveTimer = useRef<number | null>(null)
  const hydratedRouteDesignRef = useRef<string | null>(null)
  const canvasDesign = versionPreview?.document ?? design
  const selectedComponent = useMemo(
    () => canvasDesign.components.find((component) => component.id === selectedComponentId) ?? null,
    [canvasDesign.components, selectedComponentId],
  )
  const selectedConnector = useMemo(
    () => canvasDesign.connectors.find((connector) => connector.id === selectedConnectorId) ?? null,
    [canvasDesign.connectors, selectedConnectorId],
  )
  const selectedComponentIdList = useMemo(() => [...selectedComponentIds], [selectedComponentIds])
  const selectedConnectorIdList = useMemo(() => [...selectedConnectorIds], [selectedConnectorIds])
  const activeJourney = useMemo(
    () => canvasDesign.journeys?.find((journey) => journey.id === activeJourneyId) ?? null,
    [activeJourneyId, canvasDesign.journeys],
  )
  const filteredCatalogAssets = useMemo(() => {
    const query = normalizedCatalogLabel(catalogSearch)
    return catalogAssets
      .filter((asset) => isCatalogComponentType(asset.type))
      .filter((asset) => catalogTypeFilter === 'all' || asset.type === catalogTypeFilter)
      .filter((asset) => {
        if (!query) return true
        return [asset.name, asset.normalizedName, asset.type, asset.owner, asset.description, asset.tags.join(' ')]
          .some((value) => normalizedCatalogLabel(value).includes(query))
      })
      .sort((left, right) => {
        if (right.usedInDesignCount !== left.usedInDesignCount) return right.usedInDesignCount - left.usedInDesignCount
        return left.name.localeCompare(right.name)
      })
  }, [catalogAssets, catalogSearch, catalogTypeFilter])
  const catalogTypeOptions = useMemo(
    () =>
      componentCatalog.filter((item) =>
        catalogAssets.some((asset) => asset.type === item.type),
      ),
    [catalogAssets],
  )
  const traversalFocus = useMemo(() => {
    if (!activeJourney || canvasMode !== 'journey') return null
    const componentIds = activeJourney.steps.flatMap((step) => (step.componentId ? [step.componentId] : []))
    const connectorIds = activeJourney.steps.flatMap((step) => (step.connectorId ? [step.connectorId] : []))
    const activeStep = activeJourney.steps[activeJourneyStepIndex]
    return {
      componentIds,
      connectorIds,
      activeComponentId: activeStep?.componentId ?? null,
      activeConnectorId: activeStep?.connectorId ?? null,
    }
  }, [activeJourney, activeJourneyStepIndex, canvasMode])
  const isCanvasReadOnly = canvasMode !== 'design' || Boolean(versionPreview)

  useEffect(() => {
    designRef.current = design
  }, [design])

  useEffect(() => {
    designNameRef.current = designName
  }, [designName])

  useEffect(() => {
    if (activeJourneyId && canvasDesign.journeys.some((journey) => journey.id === activeJourneyId)) return
    setActiveJourneyId(canvasDesign.journeys[0]?.id ?? null)
    setActiveJourneyStepIndex(0)
  }, [activeJourneyId, canvasDesign.journeys])

  useEffect(() => {
    if (!activeJourney) return
    if (activeJourneyStepIndex < activeJourney.steps.length) return
    setActiveJourneyStepIndex(Math.max(0, activeJourney.steps.length - 1))
  }, [activeJourney, activeJourneyStepIndex])

  useEffect(() => {
    if (activeTextDocumentId && designDocs.some((document) => document.id === activeTextDocumentId)) return
    setActiveTextDocumentId(designDocs[0]?.id ?? null)
  }, [activeTextDocumentId, designDocs])

  useEffect(() => {
    let cancelled = false

    async function loadIdentity() {
      try {
        const setup = await fetchSetupStatus()
        if (cancelled) return
        if (setup.storage) setStorageStatus(setup.storage)
        setSetupRequired(setup.requiresSetup)
        setPasswordSetupRequired(setup.requiresPasswordSetup)
        if (setup.requiresSetup || setup.requiresPasswordSetup) {
          setUsers([])
          setHomeProfile(null)
          setNotifications([])
          setAuthenticated(false)
          return
        }
        const [profile, userResponse, notificationResponse, catalogResponse] = await Promise.all([
          fetchProfile(),
          fetchUsers(),
          fetchNotifications(),
          fetchCatalogAssets(),
        ])
        if (cancelled) return
        setHomeProfile(profile)
        if (profile.storage) setStorageStatus(profile.storage)
        setActiveUserId(profile.id)
        setAuthenticated(true)
        setUsers(userResponse.users)
        setNotifications(notificationResponse.notifications)
        setCatalogAssets(catalogResponse.assets)
      } catch (error) {
        if (!cancelled) {
          setAuthenticated(false)
          console.warn('Could not load collaboration identity', error)
        }
      } finally {
        if (!cancelled) setAuthChecked(true)
      }
    }

    void loadIdentity()
    return () => {
      cancelled = true
    }
  }, [authenticated])

  useEffect(() => {
    void refreshCollaboration()
  }, [activeWorkspaceId, selectedDesignId])

  useEffect(() => {
    if (view === 'admin' && !setupRequired) void refreshAdmin()
  }, [view, setupRequired, authenticated])

  useEffect(() => {
    let cancelled = false
    async function loadDesignDocs() {
      if (!selectedDesignId) {
        setDesignDocs([])
        setDirtyDocIds(new Set())
        setActiveTextDocumentId(null)
        return
      }
      setDocsSaveState('loading')
      setDocsError(null)
      try {
        const response = await fetchDesignDocs(activeWorkspaceId, selectedDesignId)
        if (cancelled) return
        setDesignDocs(response.docs)
        setDirtyDocIds(new Set())
        setActiveTextDocumentId(response.docs[0]?.id ?? null)
        setDocsSaveState('idle')
      } catch (error) {
        if (cancelled) return
        setDesignDocs([])
        setDirtyDocIds(new Set())
        setDocsError(error instanceof Error ? error.message : 'Could not load design docs')
        setDocsSaveState('error')
      }
    }

    void loadDesignDocs()
    return () => {
      cancelled = true
    }
  }, [activeWorkspaceId, selectedDesignId])

  useEffect(() => {
    let cancelled = false
    async function loadVersions() {
      if (!selectedDesignId) {
        setDesignVersions([])
        setVersionState('idle')
        return
      }
      setVersionState('loading')
      try {
        const response = await fetchDesignVersions(activeWorkspaceId, selectedDesignId)
        if (cancelled) return
        setDesignVersions(response.versions)
        setVersionState('idle')
      } catch (error) {
        if (cancelled) return
        console.warn('Could not load design versions', error)
        setDesignVersions([])
        setVersionState('error')
      }
    }
    void loadVersions()
    return () => {
      cancelled = true
    }
  }, [activeWorkspaceId, selectedDesignId])

  const backendSync = useBackendDesignSync({
    workspaceId: activeWorkspaceId,
    selectedDesignId,
    onRemoteDesign: (remoteDesign) => {
      loadDesignIntoWorkspace(remoteDesign.document, remoteDesign.name)
    },
  })

  const workspaceDesigns = useMemo(() => {
    const backendDesigns = backendSync.designs.filter((backendDesign) => !deletedDesignIds.has(backendDesign.id))
    const currentDesign = {
      id: design.id,
      workspaceId: activeWorkspaceId,
      name: designName,
      access: 'private',
      title: designName,
      document: design,
      updatedAt: design.updatedAt,
    } satisfies BackendDesign
    const merged = backendDesigns.map((backendDesign) => (backendDesign.id === design.id ? currentDesign : backendDesign))
    if (!merged.length) return [currentDesign]
    if (merged.some((backendDesign) => backendDesign.id === design.id)) return merged
    return [currentDesign, ...merged]
  }, [activeWorkspaceId, backendSync.designs, deletedDesignIds, design, designName])

  useEffect(() => {
    void refreshHome(activeWorkspaceId)
  }, [activeWorkspaceId])

  useEffect(() => {
    function handlePopState() {
      applyRoute(parseRoute(window.location.pathname), { syncHistory: false })
    }

    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  useEffect(() => {
    if (route.screen !== 'design') {
      hydratedRouteDesignRef.current = null
      return
    }

    const { workspaceId, designId } = route
    const routeDesignKey = `${workspaceId}/${designId}`
    if (hydratedRouteDesignRef.current === routeDesignKey) return

    let cancelled = false
    async function loadRoutedDesign() {
      try {
        setHomeError(null)
        const response = await fetchWorkspaceDesign(workspaceId, designId)
        if (cancelled) return
        hydratedRouteDesignRef.current = routeDesignKey
        loadDesignIntoWorkspace(response.design.document, response.design.name)
      } catch (error) {
        if (!cancelled) {
          setHomeError(error instanceof Error ? error.message : 'Could not open design from URL')
        }
      }
    }

    void loadRoutedDesign()
    return () => {
      cancelled = true
    }
  }, [route])

  useEffect(() => {
    if (!selectedDesignId) return
    const name = designName.trim()
    if (!name) return
    const timeout = window.setTimeout(() => {
      void updateDesignMetadata(activeWorkspaceId, selectedDesignId, { name })
        .then((response) => {
          setHomeDesigns((current) =>
            current.map((backendDesign) => (backendDesign.id === response.design.id ? response.design : backendDesign)),
          )
        })
        .catch((error) => {
          console.warn('Could not update design metadata', error)
        })
    }, 500)
    return () => window.clearTimeout(timeout)
  }, [activeWorkspaceId, designName, selectedDesignId])

  useEffect(() => {
    scheduleAutosave()
  }, [design, selectedDesignId])

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      const target = event.target
      if (
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement ||
        (target instanceof HTMLElement && target.isContentEditable)
      ) {
        return
      }
      const isCopy = (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'c'
      const isPaste = (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'v'
      if (isCopy) {
        if (copyCanvasSelection()) event.preventDefault()
        return
      }
      if (isPaste) {
        if (pasteCanvasSelection()) event.preventDefault()
        return
      }
      if (event.key !== 'Delete' && event.key !== 'Backspace') return
      if (isCanvasReadOnly) return
      if (selectedComponentIds.size) {
        event.preventDefault()
        deleteComponents([...selectedComponentIds])
      } else if (selectedConnectorIds.size) {
        event.preventDefault()
        deleteConnectors([...selectedConnectorIds])
      } else if (selectedComponentId) {
        event.preventDefault()
        deleteComponents([selectedComponentId])
      } else if (selectedConnectorId) {
        event.preventDefault()
        deleteConnectors([selectedConnectorId])
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [canvasClipboard, isCanvasReadOnly, selectedComponentId, selectedComponentIds, selectedConnectorId, selectedConnectorIds])

  function updateDesign(updater: (current: DesignDocument) => DesignDocument) {
    setDesign((current) => {
      const nextDesign = touchDesign(updater(current))
      designRef.current = nextDesign
      return nextDesign
    })
  }

  function sameIds(current: Set<string>, next: string[]) {
    if (current.size !== next.length) return false
    return next.every((id) => current.has(id))
  }

  function handleCanvasSelectionChange(componentIds: string[], connectorIds: string[]) {
    const nextComponentId = componentIds.length === 1 && connectorIds.length === 0 ? componentIds[0] : null
    const nextConnectorId = connectorIds.length === 1 && componentIds.length === 0 ? connectorIds[0] : null
    if (
      sameIds(selectedComponentIds, componentIds) &&
      sameIds(selectedConnectorIds, connectorIds) &&
      selectedComponentId === nextComponentId &&
      selectedConnectorId === nextConnectorId
    ) {
      return
    }
    setSelectedComponentIds(new Set(componentIds))
    setSelectedConnectorIds(new Set(connectorIds))
    setSelectedComponentId(nextComponentId)
    setSelectedConnectorId(nextConnectorId)
  }

  function getComponentAbsolutePosition(component: DesignComponent, components = designRef.current.components) {
    const position = component.metadata.position ?? { x: 0, y: 0 }
    const parentFrameId = component.metadata.parentFrameId
    if (!parentFrameId) return position
    const parent = components.find((item) => item.id === parentFrameId)
    const parentPosition = parent?.metadata.position ?? { x: 0, y: 0 }
    return {
      x: parentPosition.x + position.x,
      y: parentPosition.y + position.y,
    }
  }

  function copyCanvasSelection() {
    const selectedIds = new Set(selectedComponentIds)
    if (!selectedIds.size && selectedComponentId) selectedIds.add(selectedComponentId)
    const components = designRef.current.components
      .filter((component) => selectedIds.has(component.id))
      .map((component) => {
        const parentFrameId = component.metadata.parentFrameId
        if (!parentFrameId || selectedIds.has(parentFrameId)) return component
        return {
          ...component,
          metadata: {
            ...component.metadata,
            position: getComponentAbsolutePosition(component),
            parentFrameId: undefined,
          },
        }
      })
    if (!components.length) return false
    const connectorIds = new Set(selectedConnectorIds)
    const connectors = designRef.current.connectors.filter(
      (connector) =>
        (selectedIds.has(connector.fromComponentId) && selectedIds.has(connector.toComponentId)) ||
        connectorIds.has(connector.id),
    )
    setCanvasClipboard({
      components,
      connectors: connectors.filter((connector) => selectedIds.has(connector.fromComponentId) && selectedIds.has(connector.toComponentId)),
      copiedAt: new Date().toISOString(),
    })
    return true
  }

  function cloneComponentForPaste(
    component: DesignComponent,
    idMap: Map<string, string>,
    selectedOriginalIds: Set<string>,
    offset: { x: number; y: number },
    allCopiedComponents: DesignComponent[],
  ): DesignComponent {
    const newId = idMap.get(component.id) ?? `cmp_${crypto.randomUUID()}`
    const originalParentId = component.metadata.parentFrameId
    const mappedParentId = originalParentId ? idMap.get(originalParentId) : undefined
    const originalAbsolutePosition = getComponentAbsolutePosition(component, allCopiedComponents)
    const nextPosition = mappedParentId
      ? component.metadata.position ?? { x: 0, y: 0 }
      : {
          x: originalAbsolutePosition.x + offset.x,
          y: originalAbsolutePosition.y + offset.y,
        }
    return {
      ...component,
      id: newId,
      shapeId: `rf:${crypto.randomUUID()}`,
      metadata: {
        ...component.metadata,
        position: nextPosition,
        parentFrameId: mappedParentId && selectedOriginalIds.has(originalParentId ?? '') ? mappedParentId : undefined,
      },
      notes: component.notes.map((note) => ({ ...note, id: `note_${crypto.randomUUID()}` })),
    }
  }

  function pasteCanvasSelection() {
    if (isCanvasReadOnly || !canvasClipboard?.components.length) return false
    pasteCountRef.current += 1
    const selectedOriginalIds = new Set(canvasClipboard.components.map((component) => component.id))
    const idMap = new Map(canvasClipboard.components.map((component) => [component.id, `cmp_${crypto.randomUUID()}`]))
    const offset = {
      x: 48 + pasteCountRef.current * 18,
      y: 48 + pasteCountRef.current * 18,
    }
    const pastedComponents = canvasClipboard.components.map((component) =>
      cloneComponentForPaste(component, idMap, selectedOriginalIds, offset, canvasClipboard.components),
    )
    const pastedConnectors = canvasClipboard.connectors.flatMap((connector) => {
      const fromComponentId = idMap.get(connector.fromComponentId)
      const toComponentId = idMap.get(connector.toComponentId)
      if (!fromComponentId || !toComponentId) return []
      return [{
        ...connector,
        id: `conn_${crypto.randomUUID()}`,
        shapeId: `rf:${crypto.randomUUID()}`,
        fromComponentId,
        toComponentId,
      }]
    })
    updateDesign((current) => ({
      ...current,
      components: [...current.components, ...pastedComponents],
      connectors: [...current.connectors, ...pastedConnectors],
    }))
    const nextSelectedIds = new Set(pastedComponents.map((component) => component.id))
    setSelectedComponentIds(nextSelectedIds)
    setSelectedConnectorIds(new Set())
    setSelectedComponentId(pastedComponents.length === 1 ? pastedComponents[0].id : null)
    setSelectedConnectorId(null)
    setContextPanel('inspector')
    setRightRailOpen(true)
    return true
  }

  function renameDesign(value: string) {
    setDesignName(value)
    updateDesign((current) => ({ ...current, title: value }))
  }

  function applyRoute(nextRoute: AppRoute, options: { replace?: boolean; syncHistory?: boolean } = {}) {
    setRoute(nextRoute)
    if (nextRoute.screen === 'home') {
      setView('home')
      setSelectedDesignId(null)
    } else if (nextRoute.screen === 'admin') {
      setView('admin')
      setSelectedDesignId(null)
    } else if (nextRoute.screen === 'workspace') {
      setActiveWorkspaceId(nextRoute.workspaceId)
      setSelectedDesignId(null)
      setView('home')
    } else {
      setActiveWorkspaceId(nextRoute.workspaceId)
      setSelectedDesignId(nextRoute.designId)
      setView('canvas')
    }

    if (options.syncHistory === false) return
    const nextPath = routePath(nextRoute)
    if (window.location.pathname === nextPath) return
    const method = options.replace ? 'replaceState' : 'pushState'
    window.history[method](null, '', nextPath)
  }

  function normalizeDesignDocument(nextDesign: DesignDocument): DesignDocument {
    return {
      ...nextDesign,
      requirementBrief: createEmptyRequirementBrief(nextDesign.requirementBrief),
      journeys: nextDesign.journeys ?? [],
    }
  }

  function cloneDesignDocumentForFork(sourceDocument: unknown, nextDesignId: string, nextTitle: string): DesignDocument {
    const source = normalizeDesignDocument(sourceDocument as DesignDocument)
    return touchDesign({
      ...source,
      id: nextDesignId,
      title: nextTitle,
      requirementBrief: createEmptyRequirementBrief(source.requirementBrief),
      components: source.components ?? [],
      connectors: source.connectors ?? [],
      journeys: source.journeys ?? [],
    })
  }

  function loadDesignIntoWorkspace(nextDesign: DesignDocument, metadataName?: string) {
    const normalizedDesign = normalizeDesignDocument(nextDesign)
    setVersionPreview(null)
    setDesign(normalizedDesign)
    setDesignName(metadataName || normalizedDesign.title || 'Untitled system design')
    setSelectedComponentId(null)
    setSelectedConnectorId(null)
    setSelectedComponentIds(new Set())
    setSelectedConnectorIds(new Set())
    setActiveJourneyId(normalizedDesign.journeys[0]?.id ?? null)
    setActiveJourneyStepIndex(0)
    saveDesignDocument(normalizedDesign)
  }

  function persistDesignToRealtime() {
    const currentDesign = designRef.current
    saveDesignDocument(currentDesign)
    const saved = Boolean(selectedDesignId && backendSync.saveDesign(currentDesign))
    if (saved) markCurrentVersionDraft()
    return saved
  }

  async function persistDesignToBackend(versionRemarks = '') {
    const currentDesign = designRef.current
    saveDesignDocument(currentDesign)
    if (!selectedDesignId) return false

    setSaveState('saving')
    try {
      const response = await saveDesignDocumentToBackend(activeWorkspaceId, selectedDesignId, {
        document: currentDesign,
        versionRemarks,
      })
      setHomeDesigns((current) =>
        current.map((backendDesign) => (backendDesign.id === response.design.id ? response.design : backendDesign)),
      )
      markCurrentVersionDraft(response.design.versionNumber, response.design.updatedAt)
      setSaveState('saved')
      window.setTimeout(() => setSaveState('idle'), 1400)
      return true
    } catch (error) {
      console.warn('Could not save design document', error)
      setHomeError(error instanceof Error ? error.message : 'Could not save design')
      setSaveState('error')
      return false
    }
  }

  async function createManualVersion(remarks = '') {
    if (!selectedDesignId) return false
    const saved = await persistDesignToBackend()
    if (!saved) return false
    setVersionState('saving')
    try {
      const response = await createDesignVersion(activeWorkspaceId, selectedDesignId, remarks)
      setDesignVersions((current) => [response.version, ...current.filter((version) => version.id !== response.version.id)])
      setHomeDesigns((current) =>
        current.map((backendDesign) =>
          backendDesign.id === selectedDesignId
            ? { ...backendDesign, versionNumber: response.version.versionNumber, updatedAt: response.version.updatedAt }
            : backendDesign,
        ),
      )
      setVersionState('saved')
      window.setTimeout(() => setVersionState('idle'), 1400)
      return true
    } catch (error) {
      console.warn('Could not create design version', error)
      setHomeError(error instanceof Error ? error.message : 'Could not create version')
      setVersionState('error')
      return false
    }
  }

  async function saveDesignFromDialog() {
    const remarks = saveRemarks.trim()
    const saved = saveVersionMode === 'new'
      ? await createManualVersion(remarks)
      : await persistDesignToBackend(remarks)
    if (!saved) return
    setSaveDialogOpen(false)
    setSaveRemarks('')
  }

  async function changeVersionStatus(versionId: string, status: BackendDesignVersionStatus) {
    if (!selectedDesignId) return
    setVersionState('saving')
    try {
      const response = await updateDesignVersionStatus(activeWorkspaceId, selectedDesignId, versionId, status)
      setDesignVersions((current) =>
        current.map((version) => {
          if (version.id === response.version.id) return response.version
          if (status === 'live' && version.status === 'live') return { ...version, status: 'reviewed' }
          return version
        }),
      )
      setVersionState('saved')
      window.setTimeout(() => setVersionState('idle'), 1400)
    } catch (error) {
      console.warn('Could not update design version', error)
      setHomeError(error instanceof Error ? error.message : 'Could not update version')
      setVersionState('error')
    }
  }

  function markVersionReadyForReview(versionId: string) {
    const targetVersion = designVersions.find((version) => version.id === versionId) ?? null
    if (!targetVersion) {
      setHomeError('Could not find the selected version.')
      return
    }
    setRequestReviewVersion(targetVersion)
    setCanvasMode('comment')
    setContextPanel('review')
    setRightRailOpen(true)
    setRequestReviewOpen(true)
  }

  function requestDeleteVersion(versionId: string) {
    const targetVersion = designVersions.find((version) => version.id === versionId)
    if (!targetVersion) {
      setHomeError('Could not find the selected version.')
      return
    }
    setHomeError(null)
    setDeleteTarget({ kind: 'version', version: targetVersion })
  }

  function viewVersion(version: BackendDesignVersion) {
    const previewDocument = normalizeDesignDocument(version.document as DesignDocument)
    setVersionPreview({ version, document: previewDocument })
    setCanvasMode('view')
    setSelectedComponentId(null)
    setSelectedConnectorId(null)
    setSelectedComponentIds(new Set())
    setSelectedConnectorIds(new Set())
    setActiveJourneyId(previewDocument.journeys[0]?.id ?? null)
    setActiveJourneyStepIndex(0)
  }

  function exitVersionPreview() {
    setVersionPreview(null)
    setActiveJourneyId(designRef.current.journeys[0]?.id ?? null)
    setActiveJourneyStepIndex(0)
  }

  function markCurrentVersionDraft(versionNumber?: number, updatedAt = new Date().toISOString()) {
    const currentVersionNumber =
      versionNumber ?? homeDesigns.find((backendDesign) => backendDesign.id === selectedDesignId)?.versionNumber ?? 0
    if (currentVersionNumber <= 0) return
    setDesignVersions((current) =>
      current.map((version) =>
        version.versionNumber === currentVersionNumber && version.status !== 'live'
          ? { ...version, status: 'draft', updatedAt }
          : version,
      ),
    )
  }

  function scheduleAutosave() {
    if (!selectedDesignId) return
    if (autosaveTimer.current) window.clearTimeout(autosaveTimer.current)
    autosaveTimer.current = window.setTimeout(() => {
      autosaveTimer.current = null
      persistDesignToRealtime()
    }, 700)
  }

  function selectBackendDesign(backendDesign: BackendDesign) {
    loadDesignIntoWorkspace(backendDesign.document, backendDesign.name)
    hydratedRouteDesignRef.current = `${backendDesign.workspaceId}/${backendDesign.id}`
    applyRoute({ screen: 'design', workspaceId: backendDesign.workspaceId, designId: backendDesign.id })
  }

  function openNewDesignBrief() {
    if (homeAction) return
    setHomeError(null)
    setNewDesignTitle('Untitled system design')
    setNewDesignBrief(createEmptyRequirementBrief())
    setNewDesignBriefOpen(true)
  }

  async function createNewDesignFromBrief() {
    if (homeAction) return
    const title = newDesignTitle.trim() || newDesignBrief.useCase.trim() || 'Untitled system design'
    try {
      setHomeAction('design')
      setHomeError(null)
      const workspaceId = activeWorkspaceId || defaultWorkspace.id
      const draft = createEmptyDesign({
        title,
        requirementBrief: {
          ...newDesignBrief,
          problemStatement: newDesignBrief.problemStatement || newDesignBrief.useCase,
        },
      })
      const response = await createDesign(workspaceId, draft.title, null)
      const nextDesign = touchDesign({
        ...response.design.document,
        title,
        requirementBrief: createEmptyRequirementBrief(draft.requirementBrief),
      })
      const savedResponse = await saveDesignDocumentToBackend(response.design.workspaceId, response.design.id, {
        document: nextDesign,
      })
      const savedDesign = savedResponse.design
      setNewDesignBriefOpen(false)
      setHomeDesigns((current) => [savedDesign, ...current.filter((designItem) => designItem.id !== savedDesign.id)])
      loadDesignIntoWorkspace(savedDesign.document, savedDesign.name)
      hydratedRouteDesignRef.current = `${savedDesign.workspaceId}/${savedDesign.id}`
      applyRoute({ screen: 'design', workspaceId: savedDesign.workspaceId, designId: savedDesign.id })
    } catch (error) {
      setHomeError(error instanceof Error ? error.message : 'Could not create design')
    } finally {
      setHomeAction(null)
    }
  }

  async function requestCloneDesign(sourceDesign: BackendDesign) {
    if (homeAction || cloneState === 'cloning') return
    setCloneTarget(sourceDesign)
    setCloneTitle(`${sourceDesign.name || sourceDesign.document.title || 'Untitled design'} Copy`)
    setCloneVersionId('current')
    setCloneVersions([])
    setCloneError(null)
    setCloneState('loading')
    try {
      const response = await fetchDesignVersions(sourceDesign.workspaceId, sourceDesign.id)
      const versions = response.versions
      setCloneVersions(versions)
      const preferredVersion = versions.find((version) => version.status === 'live') ?? versions[0]
      setCloneVersionId(preferredVersion?.id ?? 'current')
      setCloneState('idle')
    } catch (error) {
      setCloneVersions([])
      setCloneVersionId('current')
      setCloneError(error instanceof Error ? error.message : 'Could not load versions for cloning')
      setCloneState('error')
    }
  }

  async function confirmCloneDesign() {
    if (!cloneTarget || cloneState === 'cloning') return
    const title = cloneTitle.trim() || `${cloneTarget.name || cloneTarget.document.title || 'Untitled design'} Copy`
    const selectedVersion = cloneVersions.find((version) => version.id === cloneVersionId)
    const sourceDocument = selectedVersion?.document ?? cloneTarget.document
    try {
      setCloneState('cloning')
      setCloneError(null)
      const created = await createDesign(cloneTarget.workspaceId, title, null)
      const clonedDocument = cloneDesignDocumentForFork(sourceDocument, created.design.id, title)
      const saved = await saveDesignDocumentToBackend(created.design.workspaceId, created.design.id, {
        document: clonedDocument,
        versionRemarks: selectedVersion
          ? `Cloned from ${cloneTarget.name || cloneTarget.document.title || 'design'} v${selectedVersion.versionNumber}`
          : `Cloned from ${cloneTarget.name || cloneTarget.document.title || 'design'}`,
      })
      setHomeDesigns((current) => [saved.design, ...current.filter((designItem) => designItem.id !== saved.design.id)])
      setCloneTarget(null)
      setCloneVersions([])
      setCloneVersionId('current')
      setCloneTitle('')
      setCloneState('idle')
      loadDesignIntoWorkspace(saved.design.document, saved.design.name)
      hydratedRouteDesignRef.current = `${saved.design.workspaceId}/${saved.design.id}`
      applyRoute({ screen: 'design', workspaceId: saved.design.workspaceId, designId: saved.design.id })
    } catch (error) {
      setCloneError(error instanceof Error ? error.message : 'Could not clone design')
      setCloneState('error')
    }
  }

  async function refreshHome(workspaceId: string) {
    try {
      setHomeError(null)
      const [profile, workspaces] = await Promise.all([fetchProfile(), fetchWorkspaces()])
      const nextWorkspaces = workspaces.workspaces.length ? workspaces.workspaces : [defaultWorkspace]
      setHomeProfile(profile)
      if (profile.storage) setStorageStatus(profile.storage)
      setHomeWorkspaces(nextWorkspaces)
      const selectedWorkspace = nextWorkspaces.some((workspace) => workspace.id === workspaceId)
        ? workspaceId
        : nextWorkspaces[0]?.id || defaultWorkspace.id
      if (selectedWorkspace !== workspaceId) {
        applyRoute({ screen: 'workspace', workspaceId: selectedWorkspace }, { replace: true })
        return
      }
      try {
        const designs = await fetchWorkspaceDesigns(selectedWorkspace)
        setHomeDesigns(designs.designs)
      } catch (error) {
        setHomeDesigns([])
        setHomeError(error instanceof Error ? error.message : 'Could not load workspace designs')
      }
    } catch (error) {
      setHomeProfile(null)
      setHomeWorkspaces((current) => (current.length ? current : [defaultWorkspace]))
      setHomeDesigns([])
      setHomeError(error instanceof Error ? error.message : 'Backend unavailable')
    }
  }

  async function completeFirstAdmin(input: { displayName: string; email: string; password: string }) {
    setSetupAction('saving')
    setHomeError(null)
    try {
      const response = await createFirstAdmin(input)
      setAuthenticated(true)
      setActiveUserId(response.user.id)
      setUsers([response.user])
      setHomeProfile(response.user)
      setSetupRequired(false)
      setPasswordSetupRequired(false)
      setSetupAction('idle')
      await refreshHome(activeWorkspaceId)
    } catch (error) {
      setSetupAction('error')
      setHomeError(error instanceof Error ? error.message : 'Could not create admin user')
    }
  }

  async function completePasswordSetup(input: { email: string; password: string }) {
    setAuthAction('saving')
    setAuthError(null)
    try {
      const response = await setInitialAdminPassword(input)
      setAuthenticated(true)
      setActiveUserId(response.user.id)
      setHomeProfile(response.user)
      setPasswordSetupRequired(false)
      setAuthAction('idle')
      await refreshHome(activeWorkspaceId)
    } catch (error) {
      setAuthAction('error')
      setAuthError(error instanceof Error ? error.message : 'Could not set admin password')
    }
  }

  async function signIn(input: { email: string; password: string }) {
    setAuthAction('saving')
    setAuthError(null)
    try {
      const response = await login(input)
      setAuthenticated(true)
      setActiveUserId(response.user.id)
      setHomeProfile(response.user)
      setAuthAction('idle')
      await refreshHome(activeWorkspaceId)
    } catch (error) {
      setAuthAction('error')
      setAuthError(error instanceof Error ? error.message : 'Could not sign in')
    }
  }

  async function signOut() {
    await logout().catch(() => undefined)
    setAuthenticated(false)
    setActiveUserId('')
    setHomeProfile(null)
    setUsers([])
    applyRoute({ screen: 'home' })
  }

  async function refreshAdmin() {
    setAdminError(null)
    const [userResponse, signInResponse, aiProviderResponse, mcpResponse, catalogResponse, storageResponse] = await Promise.allSettled([
      fetchUsers(),
      fetchSignInConfig(),
      fetchAIProviderConfig(),
      fetchMCPConfig(),
      fetchCatalogAssets(),
      fetchAdminStorageStatus(),
    ] as const)
    const criticalFailure = [userResponse, signInResponse, aiProviderResponse, catalogResponse].find((result) => result.status === 'rejected')
    if (criticalFailure?.status === 'rejected') {
      setAdminError(criticalFailure.reason instanceof Error ? criticalFailure.reason.message : 'Could not load admin console')
    }
    if (userResponse.status === 'fulfilled') setUsers(userResponse.value.users)
    if (signInResponse.status === 'fulfilled') setSignInConfig(signInResponse.value.signIn)
    if (aiProviderResponse.status === 'fulfilled') setAIConnection(aiProviderResponse.value.aiProvider)
    if (mcpResponse.status === 'fulfilled') setMCPConfig(mcpResponse.value.mcp)
    if (catalogResponse.status === 'fulfilled') setCatalogAssets(catalogResponse.value.assets)
    if (storageResponse.status === 'fulfilled') setStorageStatus(storageResponse.value.storage)
  }

  async function addCatalogAsset(input: {
    name: string
    type: string
    owner?: string
    description?: string
    criticality?: string
    tags?: string[]
  }) {
    setAdminError(null)
    try {
      const response = await createCatalogAsset(input)
      setCatalogAssets((current) => [response.asset, ...current.filter((asset) => asset.id !== response.asset.id)])
      return response.asset
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not add catalog asset')
      throw error
    }
  }

  async function saveCatalogAsset(asset: BackendCatalogAsset) {
    setAdminError(null)
    try {
      const response = await updateCatalogAsset(asset.id, asset)
      setCatalogAssets((current) => current.map((item) => (item.id === response.asset.id ? response.asset : item)))
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not save catalog asset')
      throw error
    }
  }

  async function removeCatalogAsset(assetId: string) {
    setAdminError(null)
    try {
      await deleteCatalogAsset(assetId)
      setCatalogAssets((current) => current.filter((asset) => asset.id !== assetId))
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not delete catalog asset')
      throw error
    }
  }

  async function addAdminUser(input: { displayName: string; email: string; role: string; password?: string }) {
    setAdminError(null)
    try {
      const response = await createAdminUser(input)
      setUsers((current) => [...current, response.user])
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not add user')
    }
  }

  async function saveAdminUser(user: BackendUser & { password?: string }) {
    setAdminError(null)
    try {
      const response = await updateAdminUser(user.id, {
        displayName: user.displayName,
        email: user.email,
        role: user.role,
        status: user.status,
        password: user.password?.trim() || undefined,
      })
      setUsers((current) => current.map((item) => (item.id === response.user.id ? response.user : item)))
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not save user')
    }
  }

  async function removeAdminUser(userId: string) {
    setAdminError(null)
    try {
      await deleteAdminUser(userId)
      setUsers((current) => current.filter((user) => user.id !== userId))
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not delete user')
    }
  }

  async function saveSignIn(nextConfig: BackendSignInConfig & { clientSecret?: string }) {
    setAdminError(null)
    try {
      const response = await updateSignInConfig(nextConfig)
      setSignInConfig(response.signIn)
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not save sign-in settings')
    }
  }

  async function saveAIProvider(nextConfig: Partial<BackendAIProviderConfig> & { apiKey?: string }) {
    setAdminError(null)
    try {
      const response = await updateAIProviderConfig(nextConfig)
      setAIConnection(response.aiProvider)
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not save AI provider settings')
    }
  }

  async function saveMCP(nextConfig: BackendMCPConfig) {
    setAdminError(null)
    try {
      const response = await updateMCPConfig(nextConfig)
      setMCPConfig(response.mcp)
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not save MCP settings')
    }
  }

  async function refreshStorage() {
    setAdminError(null)
    try {
      const response = await fetchAdminStorageStatus()
      setStorageStatus(response.storage)
      return response.storage
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not load storage settings')
      throw error
    }
  }

  async function testDatabaseStorage(databaseUrl: string) {
    setAdminError(null)
    const response = await testStorageDatabase(databaseUrl)
    setStorageStatus(response.storage)
    return response
  }

  async function configureDatabaseStorage(databaseUrl: string) {
    setAdminError(null)
    const response = await configureStorageDatabase(databaseUrl)
    setStorageStatus(response.storage)
    return response.storage
  }

  async function migrateCachedUsersToDatabase() {
    setAdminError(null)
    const response = await migrateStorageUsers()
    setStorageStatus(response.storage)
    await refreshHome(activeWorkspaceId)
    return response
  }

  async function refreshNotifications() {
    try {
      const response = await fetchNotifications()
      setNotifications(response.notifications)
    } catch (error) {
      console.warn('Could not load notifications', error)
    }
  }

  async function refreshCollaboration() {
    if (!selectedDesignId) {
      setComments([])
      setReviews([])
      setCollaborationState('idle')
      return
    }
    setCollaborationState('loading')
      setCollaborationError(null)
      setCollaborationOffline(false)
    try {
      const [commentResponse, reviewResponse] = await Promise.all([
        fetchDesignComments(activeWorkspaceId, selectedDesignId),
        fetchDesignReviews(activeWorkspaceId, selectedDesignId),
      ])
      setComments(commentResponse.comments)
      setReviews(reviewResponse.reviews)
      setCollaborationState('idle')
    } catch (error) {
      setComments([])
      setReviews([])
      const message = error instanceof Error ? error.message : 'Could not load design review activity'
      const offline = isBackendOfflineMessage(message)
      setCollaborationOffline(offline)
      setCollaborationError(offline ? null : message)
      setCollaborationState(offline ? 'idle' : 'error')
    }
  }

  async function submitDesignComment() {
    const body = commentDraft.trim()
    if (!selectedDesignId || !body) return
      setCollaborationState('saving')
      setCollaborationError(null)
      setCollaborationOffline(false)
    try {
      const response = await createDesignComment(activeWorkspaceId, selectedDesignId, {
        body,
        componentId: selectedComponentId ?? undefined,
        connectorId: selectedConnectorId ?? undefined,
      })
      setComments((current) => [response.comment, ...current])
      setCommentDraft('')
      setCollaborationState('idle')
      await refreshNotifications()
    } catch (error) {
      setCollaborationError(error instanceof Error ? error.message : 'Could not add comment')
      setCollaborationOffline(error instanceof Error && isBackendOfflineMessage(error.message))
      setCollaborationState('error')
    }
  }

  async function submitReviewRequest(reviewerIds: string[], message: string) {
    if (!selectedDesignId || !reviewerIds.length) return
    const reviewVersion = requestReviewVersion ?? versionPreview?.version ?? designVersions[0]
    if (!reviewVersion) {
      setCollaborationError('Create or save a design version before requesting review')
      return
    }
    setCollaborationState('saving')
    setCollaborationError(null)
    setCollaborationOffline(false)
    try {
      const response = await requestDesignReview(activeWorkspaceId, selectedDesignId, {
        versionId: reviewVersion.id,
        reviewerIds,
        message,
      })
      setReviews((current) => [
        ...response.reviews,
        ...current.filter((review) => !response.reviews.some((item) => item.id === review.id)),
      ])
      const versionResponse = await fetchDesignVersions(activeWorkspaceId, selectedDesignId)
      setDesignVersions(versionResponse.versions)
      setRequestReviewOpen(false)
      setRequestReviewVersion(null)
      setCollaborationState('idle')
      await refreshNotifications()
    } catch (error) {
      setCollaborationError(error instanceof Error ? error.message : 'Could not request review')
      setCollaborationOffline(error instanceof Error && isBackendOfflineMessage(error.message))
      setCollaborationState('error')
    }
  }

  async function completeReview(reviewId: string, status: BackendDesignReviewRequest['status']) {
    if (!selectedDesignId) return
    setCollaborationState('saving')
    setCollaborationError(null)
    setCollaborationOffline(false)
    try {
      const response = await updateDesignReview(activeWorkspaceId, selectedDesignId, reviewId, {
        status,
        summary: reviewSummaryDrafts[reviewId] ?? '',
      })
      setReviews((current) => current.map((review) => (review.id === response.review.id ? response.review : review)))
      setReviewSummaryDrafts((current) => ({ ...current, [reviewId]: response.review.summary }))
      if (response.review.versionId) {
        void fetchDesignVersions(activeWorkspaceId, selectedDesignId).then((versionResponse) => setDesignVersions(versionResponse.versions))
      }
      setCollaborationState('idle')
      await refreshNotifications()
    } catch (error) {
      setCollaborationError(error instanceof Error ? error.message : 'Could not update review')
      setCollaborationOffline(error instanceof Error && isBackendOfflineMessage(error.message))
      setCollaborationState('error')
    }
  }

  async function markNotificationAsRead(notificationId: string) {
    try {
      await markNotificationRead(notificationId)
      setNotifications((current) =>
        current.map((notification) => (notification.id === notificationId ? { ...notification, read: true } : notification)),
      )
    } catch (error) {
      console.warn('Could not mark notification read', error)
    }
  }

  async function handleCreateWorkspace() {
    const name = newWorkspaceName.trim()
    if (!name) {
      setHomeError('Workspace name is required')
      return
    }
    if (homeAction) return
    try {
      setHomeAction('workspace')
      setHomeError(null)
      const result = await createWorkspace(name)
      setHomeWorkspaces((current) => [result.workspace, ...current.filter((workspace) => workspace.id !== result.workspace.id)])
      setHomeDesigns([])
      setNewWorkspaceName('')
      applyRoute({ screen: 'workspace', workspaceId: result.workspace.id })
      await refreshHome(result.workspace.id)
    } catch (error) {
      setHomeError(error instanceof Error ? error.message : 'Could not create workspace')
    } finally {
      setHomeAction(null)
    }
  }

  function requestDeleteWorkspace(workspace: BackendWorkspace) {
    if (workspace.id === defaultWorkspace.id) {
      setHomeError('The default guest workspace cannot be deleted.')
      return
    }
    if (workspace.id !== activeWorkspaceId || homeDesigns.length > 0) {
      setHomeError('Workspace deletion is available only after the selected workspace is empty.')
      return
    }
    setHomeError(null)
    setDeleteTarget({ kind: 'workspace', workspace })
  }

  function requestDeleteDesign(design: BackendDesign) {
    setHomeError(null)
    setDeleteTarget({ kind: 'design', design })
  }

  async function confirmDeleteTarget() {
    if (!deleteTarget || homeAction) return
    try {
      setHomeError(null)
      if (deleteTarget.kind === 'workspace') {
        setHomeAction('delete-workspace')
        await deleteWorkspaceFromBackend(deleteTarget.workspace.id)
        const remainingWorkspaces = homeWorkspaces.filter((workspace) => workspace.id !== deleteTarget.workspace.id)
        const nextWorkspaceId =
          activeWorkspaceId === deleteTarget.workspace.id
            ? remainingWorkspaces[0]?.id ?? defaultWorkspace.id
            : activeWorkspaceId
        setHomeWorkspaces(remainingWorkspaces.length ? remainingWorkspaces : [defaultWorkspace])
        setHomeDesigns([])
        setDeleteTarget(null)
        if (activeWorkspaceId === deleteTarget.workspace.id) {
          clearSavedDesign()
          setDesign(createEmptyDesign())
          setDesignName('Untitled system design')
          applyRoute({ screen: 'workspace', workspaceId: nextWorkspaceId }, { replace: true })
        }
        await refreshHome(nextWorkspaceId)
        return
      }

      if (deleteTarget.kind === 'version') {
        if (!selectedDesignId) return
        setHomeAction('delete-version')
        await deleteDesignVersionFromBackend(activeWorkspaceId, selectedDesignId, deleteTarget.version.id)
        const nextVersions = designVersions.filter((version) => version.id !== deleteTarget.version.id)
        setDesignVersions(nextVersions)
        setReviews((current) => current.filter((review) => review.versionId !== deleteTarget.version.id))
        setHomeDesigns((current) =>
          current.map((designItem) =>
            designItem.id === selectedDesignId
              ? { ...designItem, versionNumber: nextVersions[0]?.versionNumber ?? 0, updatedAt: new Date().toISOString() }
              : designItem,
          ),
        )
        if (versionPreview?.version.id === deleteTarget.version.id) {
          exitVersionPreview()
        }
        if (requestReviewVersion?.id === deleteTarget.version.id) {
          setRequestReviewVersion(null)
          setRequestReviewOpen(false)
        }
        setDeleteTarget(null)
        return
      }

      setHomeAction('delete-design')
      await deleteDesignFromBackend(deleteTarget.design.workspaceId, deleteTarget.design.id)
      setDeletedDesignIds((current) => new Set(current).add(deleteTarget.design.id))
      setHomeDesigns((current) => current.filter((designItem) => designItem.id !== deleteTarget.design.id))
      setDeleteTarget(null)
      if (selectedDesignId === deleteTarget.design.id) {
        clearSavedDesign()
        setDesign(createEmptyDesign())
        setDesignName('Untitled system design')
        setSelectedComponentId(null)
        setSelectedConnectorId(null)
        applyRoute({ screen: 'workspace', workspaceId: deleteTarget.design.workspaceId }, { replace: true })
      } else {
        await refreshHome(activeWorkspaceId)
      }
    } catch (error) {
      setHomeError(error instanceof Error ? error.message : 'Could not delete item')
    } finally {
      setHomeAction(null)
    }
  }

  function addComponent(type: ComponentType) {
    if (isCanvasReadOnly) return
    createReactFlowComponent(type, nextReactFlowPosition())
  }

  function addCatalogAssetToCanvas(asset: BackendCatalogAsset) {
    if (isCanvasReadOnly) return
    createReactFlowCatalogAsset(asset, nextReactFlowPosition())
  }

  function nextReactFlowPosition() {
    const index = designRef.current.components.length
    return {
      x: 120 + (index % 3) * 280,
      y: 120 + Math.floor(index / 3) * 170,
    }
  }

  function createReactFlowComponent(type: ComponentType, position: { x: number; y: number }) {
    const item = getCatalogItem(type)
    const shapeId = `rf:${crypto.randomUUID()}`
    const baseComponent = createComponent({ shapeId, type, name: item.label })
    const defaultFrameSize = type === 'frame.cloud' ? { width: 540, height: 340 } : undefined
    const parentFrame = type === 'frame.cloud' ? undefined : findContainingFrame(position)
    const storedPosition = parentFrame?.metadata.position
      ? { x: position.x - parentFrame.metadata.position.x, y: position.y - parentFrame.metadata.position.y }
      : position
    const component = {
      ...baseComponent,
      metadata: {
        ...baseComponent.metadata,
        position: storedPosition,
        ...(parentFrame ? { parentFrameId: parentFrame.id } : {}),
        ...(defaultFrameSize ? { size: defaultFrameSize } : {}),
      },
    }
    updateDesign((current) => ({
      ...current,
      components: [...current.components, component],
    }))
    setSelectedComponentId(component.id)
    setSelectedComponentIds(new Set([component.id]))
    setSelectedConnectorId(null)
    setSelectedConnectorIds(new Set())
  }

  function createReactFlowCatalogAsset(asset: BackendCatalogAsset, position: { x: number; y: number }) {
    if (!isCatalogComponentType(asset.type)) return
    const shapeId = `rf:${crypto.randomUUID()}`
    const baseComponent = createComponent({ shapeId, type: asset.type, name: asset.name })
    const parentFrame = findContainingFrame(position)
    const storedPosition = parentFrame?.metadata.position
      ? { x: position.x - parentFrame.metadata.position.x, y: position.y - parentFrame.metadata.position.y }
      : position
    const component: DesignComponent = {
      ...baseComponent,
      owner: asset.owner,
      purpose: asset.description,
      criticality: (asset.criticality || 'medium') as DesignComponent['criticality'],
      metadata: {
        ...baseComponent.metadata,
        position: storedPosition,
        ...(parentFrame ? { parentFrameId: parentFrame.id } : {}),
        enterpriseAsset: {
          assetId: asset.id,
          name: asset.name,
          type: asset.type,
          owner: asset.owner,
          criticality: asset.criticality,
          linkedAt: new Date().toISOString(),
        },
      },
    }
    updateDesign((current) => ({
      ...current,
      components: [...current.components, component],
    }))
    setSelectedComponentId(component.id)
    setSelectedComponentIds(new Set([component.id]))
    setSelectedConnectorId(null)
    setSelectedConnectorIds(new Set())
    setContextPanel('inspector')
    setRightRailOpen(true)
  }

  function findContainingFrame(position: { x: number; y: number }) {
    return designRef.current.components.find((component) => {
      if (component.type !== 'frame.cloud') return false
      const framePosition = component.metadata.position ?? { x: 0, y: 0 }
      const frameSize = component.metadata.size ?? { width: 540, height: 340 }
      return (
        position.x >= framePosition.x &&
        position.x <= framePosition.x + frameSize.width &&
        position.y >= framePosition.y &&
        position.y <= framePosition.y + frameSize.height
      )
    })
  }

  function moveReactFlowComponent(componentId: string, position: { x: number; y: number }, parentFrameId?: string) {
    updateDesign((current) => ({
      ...current,
      components: current.components.map((component) =>
        component.id === componentId
          ? { ...component, metadata: { ...component.metadata, position, parentFrameId } }
          : component,
      ),
    }))
  }

  function resizeReactFlowComponent(componentId: string, size: { width: number; height: number }) {
    updateDesign((current) => ({
      ...current,
      components: current.components.map((component) =>
        component.id === componentId
          ? { ...component, metadata: { ...component.metadata, size } }
          : component,
      ),
    }))
  }

  function updateComponentFromInspector(component: DesignComponent) {
    updateDesign((current) => ({
      ...current,
      components: current.components.map((item) => (item.id === component.id ? component : item)),
    }))
  }

  function duplicateReactFlowComponent(componentId: string) {
    updateDesign((current) => {
      const component = current.components.find((item) => item.id === componentId)
      if (!component) return current
      const position = component.metadata.position ?? { x: 120, y: 120 }
      const copy = {
        ...component,
        id: `cmp_${crypto.randomUUID()}`,
        shapeId: `rf:${crypto.randomUUID()}`,
        name: `${component.name || getCatalogItem(component.type).label} Copy`,
        metadata: {
          ...component.metadata,
          position: {
            x: position.x + 36,
            y: position.y + 36,
          },
        },
        notes: component.notes.map((note) => ({ ...note, id: `note_${crypto.randomUUID()}` })),
      }
      setSelectedComponentId(copy.id)
      setSelectedComponentIds(new Set([copy.id]))
      setSelectedConnectorId(null)
      setSelectedConnectorIds(new Set())
      return {
        ...current,
        components: [...current.components, copy],
      }
    })
  }

  function createStructuredConnector(
    fromComponentId: string,
    toComponentId: string,
    type: ConnectorType,
  ) {
    if (fromComponentId === toComponentId) return
    updateDesign((current) => {
      if (
        current.connectors.some(
          (connector) =>
            connector.fromComponentId === fromComponentId &&
            connector.toComponentId === toComponentId &&
            connector.type === type,
        )
      ) {
        return current
      }
      return {
        ...current,
        connectors: [
          ...current.connectors,
          {
            id: `conn_${crypto.randomUUID()}`,
            fromComponentId,
            toComponentId,
            type,
            protocol: '',
            timeoutMs: null,
            consistencyExpectation: '',
            notes: '',
            animated: true,
          },
        ],
      }
    })
    setSelectedComponentId(null)
    setSelectedComponentIds(new Set())
    setSelectedConnectorIds(new Set())
  }

  function completePendingConnector(type: ConnectorType) {
    if (!pendingConnector) return
    createStructuredConnector(pendingConnector.fromComponentId, pendingConnector.toComponentId, type)
    setPendingConnector(null)
  }

  function deleteComponents(componentIds: string[]) {
    if (!componentIds.length) return
    const ids = new Set(componentIds)
    updateDesign((current) => {
      const removedConnectorIds = new Set(
        current.connectors
          .filter((connector) => ids.has(connector.fromComponentId) || ids.has(connector.toComponentId))
          .map((connector) => connector.id),
      )
      return {
        ...current,
        components: current.components.filter((component) => !ids.has(component.id)),
        connectors: current.connectors.filter((connector) => !removedConnectorIds.has(connector.id)),
        journeys: (current.journeys ?? []).map((journey) => ({
          ...journey,
          steps: journey.steps.filter(
            (step) =>
              (!step.componentId || !ids.has(step.componentId)) &&
              (!step.connectorId || !removedConnectorIds.has(step.connectorId)),
          ),
        })),
      }
    })
    if (selectedComponentId && ids.has(selectedComponentId)) setSelectedComponentId(null)
    setSelectedConnectorId(null)
    setSelectedComponentIds((current) => new Set([...current].filter((componentId) => !ids.has(componentId))))
    setSelectedConnectorIds(new Set())
  }

  function updateConnectorFromInspector(connector: DesignConnector) {
    updateDesign((current) => ({
      ...current,
      connectors: current.connectors.map((item) => (item.id === connector.id ? connector : item)),
    }))
  }

  function reconnectConnector(connectorId: string, fromComponentId: string, toComponentId: string) {
    updateDesign((current) => ({
      ...current,
      connectors: current.connectors.map((connector) =>
        connector.id === connectorId
          ? { ...connector, fromComponentId, toComponentId }
          : connector,
      ),
    }))
    setSelectedConnectorId(connectorId)
    setSelectedConnectorIds(new Set([connectorId]))
    setSelectedComponentId(null)
    setSelectedComponentIds(new Set())
  }

  function deleteConnectors(connectorIds: string[]) {
    if (!connectorIds.length) return
    const ids = new Set(connectorIds)
    updateDesign((current) => ({
      ...current,
      connectors: current.connectors.filter((connector) => !ids.has(connector.id)),
      journeys: (current.journeys ?? []).map((journey) => ({
        ...journey,
        steps: journey.steps.filter((step) => !step.connectorId || !ids.has(step.connectorId)),
      })),
    }))
    if (selectedConnectorId && ids.has(selectedConnectorId)) setSelectedConnectorId(null)
    setSelectedConnectorIds((current) => new Set([...current].filter((connectorId) => !ids.has(connectorId))))
  }

  function createPrimaryJourney() {
    const journey = buildPrimaryJourney(designRef.current)
    if (!journey) {
      setHomeError('Add at least one non-frame component before creating a journey.')
      return
    }
    updateDesign((current) => ({
      ...current,
      journeys: [journey, ...(current.journeys ?? [])],
    }))
    setActiveJourneyId(journey.id)
    setActiveJourneyStepIndex(0)
    setCanvasMode('journey')
    setSelectedComponentId(null)
    setSelectedConnectorId(null)
    setSelectedComponentIds(new Set())
    setSelectedConnectorIds(new Set())
  }

  function updateJourney(journey: DesignJourney) {
    updateDesign((current) => ({
      ...current,
      journeys: (current.journeys ?? []).map((item) =>
        item.id === journey.id ? { ...journey, updatedAt: new Date().toISOString() } : item,
      ),
    }))
  }

  function deleteJourney(journeyId: string) {
    updateDesign((current) => ({
      ...current,
      journeys: (current.journeys ?? []).filter((journey) => journey.id !== journeyId),
    }))
    if (activeJourneyId === journeyId) {
      setActiveJourneyId(null)
      setActiveJourneyStepIndex(0)
    }
  }

  async function addTextDocument() {
    if (!selectedDesignId) return
    setDocsSaveState('saving')
    setDocsError(null)
    try {
      const response = await createDesignDoc(activeWorkspaceId, selectedDesignId, {
        title: 'Design notes',
        body: '',
        format: 'html',
      })
      setDesignDocs((current) => [response.doc, ...current.filter((document) => document.id !== response.doc.id)])
      setActiveTextDocumentId(response.doc.id)
      setDocsSaveState('saved')
      window.setTimeout(() => setDocsSaveState('idle'), 1400)
    } catch (error) {
      setDocsError(error instanceof Error ? error.message : 'Could not create design doc')
      setDocsSaveState('error')
    }
  }

  function updateTextDocument(documentId: string, patch: Partial<Pick<BackendDesignDoc, 'title' | 'body'>>) {
    const existing = designDocs.find((document) => document.id === documentId)
    if (!existing) return
    const nextTitle = patch.title ?? existing.title
    const nextBody = patch.body ?? existing.body
    if (nextTitle === existing.title && nextBody === existing.body) return

    setDesignDocs((current) =>
      current.map((document) => {
        if (document.id !== documentId) return document
        return { ...document, title: nextTitle, body: nextBody, updatedAt: new Date().toISOString() }
      }),
    )
    setDirtyDocIds((current) => new Set(current).add(documentId))
    setDocsSaveState('idle')
  }

  async function saveTextDocument(documentId: string) {
    if (!selectedDesignId) return
    const document = designDocs.find((item) => item.id === documentId)
    if (!document) return
    setDocsSaveState('saving')
    setDocsError(null)
    try {
      const response = await updateDesignDoc(activeWorkspaceId, selectedDesignId, document.id, {
        title: document.title,
        body: document.body,
        format: document.format,
      })
      setDesignDocs((current) => current.map((item) => (item.id === response.doc.id ? response.doc : item)))
      setDirtyDocIds((current) => {
        const next = new Set(current)
        next.delete(response.doc.id)
        return next
      })
      setDocsSaveState('saved')
      window.setTimeout(() => setDocsSaveState('idle'), 1400)
    } catch (error) {
      setDocsError(error instanceof Error ? error.message : 'Could not save design doc')
      setDocsSaveState('error')
    }
  }

  async function deleteTextDocument(documentId: string) {
    if (!selectedDesignId) return
    setDocsSaveState('saving')
    setDocsError(null)
    try {
      await deleteDesignDocFromBackend(activeWorkspaceId, selectedDesignId, documentId)
      setDesignDocs((current) => current.filter((document) => document.id !== documentId))
      setDirtyDocIds((current) => {
        const next = new Set(current)
        next.delete(documentId)
        return next
      })
      if (activeTextDocumentId === documentId) setActiveTextDocumentId(null)
      setDocsSaveState('saved')
      window.setTimeout(() => setDocsSaveState('idle'), 1400)
    } catch (error) {
      setDocsError(error instanceof Error ? error.message : 'Could not delete design doc')
      setDocsSaveState('error')
    }
  }

  function saveDraft() {
    setSaveVersionMode(designVersions.length ? 'override' : 'new')
    setSaveDialogOpen(true)
  }

  function openAnalysisModal(options: { run?: boolean } = {}) {
    setAnalysisModalOpen(true)
    if (options.run) void runAnalysis()
  }

  async function runAnalysis() {
    if (!selectedDesignId || analysisState === 'running') return
    setAnalysisState('running')
    setAnalysisError(null)
    try {
      await persistDesignToBackend()
      const response = await analyzeDesign(activeWorkspaceId, selectedDesignId)
      setAnalysisReport(response.analysis)
      setAnalysisState('ready')
    } catch (error) {
      setAnalysisError(error instanceof Error ? error.message : 'Could not analyze design')
      setAnalysisState('error')
    }
  }

  function resetDraft() {
    clearSavedDesign()
    setDesign(createEmptyDesign())
    setSelectedComponentId(null)
    setSelectedConnectorId(null)
  }

  function exportDesign() {
    const blob = new Blob([JSON.stringify(toExportableDesign(design), null, 2)], {
      type: 'application/json',
    })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `${designName.trim().replace(/\s+/g, '-').toLowerCase() || 'system-design'}.json`
    anchor.click()
    URL.revokeObjectURL(url)
  }

  if (setupRequired) {
    return (
      <div className="screen-shell">
        <FirstAdminOnboarding
          error={homeError}
          isSaving={setupAction === 'saving'}
          onCreate={(input) => void completeFirstAdmin(input)}
        />
      </div>
    )
  }

  if (!authChecked) {
    return (
      <div className="screen-shell">
        <section className="onboarding-card">
          <div className="home-logo">S</div>
          <p className="eyebrow">Secure session</p>
          <h1>Checking access</h1>
          <p>Verifying your Stratum session.</p>
        </section>
      </div>
    )
  }

  if (passwordSetupRequired || !authenticated) {
    return (
      <div className="screen-shell">
        <LoginScreen
          requiresPasswordSetup={passwordSetupRequired}
          error={authError}
          isSaving={authAction === 'saving'}
          onLogin={(input) => void signIn(input)}
          onSetInitialPassword={(input) => void completePasswordSetup(input)}
        />
      </div>
    )
  }

  if (view === 'admin') {
    if (homeProfile?.role !== 'admin') {
      return (
        <div className="screen-shell">
          <AppNavbar
            profile={homeProfile}
            route={route}
            workspaceName={homeWorkspaces.find((workspace) => workspace.id === activeWorkspaceId)?.name}
            designTitle={designName}
            designs={homeDesigns}
            selectedDesignId={selectedDesignId}
            aiConnection={aiConnection}
            currentUser={homeProfile}
            notifications={notifications}
            onHome={() => applyRoute({ screen: 'home' })}
            onWorkspace={() => applyRoute({ screen: 'workspace', workspaceId: activeWorkspaceId })}
            onSelectDesign={selectBackendDesign}
            onAdmin={() => applyRoute({ screen: 'admin' })}
          onOpenNotifications={() => setNotificationsOpen(true)}
          onLogout={() => void signOut()}
        />
        <StatelessModeBanner storageStatus={storageStatus} />
          <main className="admin-shell">
            <section className="onboarding-card">
              <p className="eyebrow">Access control</p>
              <h1>Admin access required</h1>
              <p>Your current account does not have permission to manage users, sign-in, or provider settings.</p>
              <button className="primary-action full-width" type="button" onClick={() => applyRoute({ screen: 'home' })}>
                Go home
              </button>
            </section>
          </main>
        </div>
      )
    }
    return (
      <div className="screen-shell">
        <AppNavbar
          profile={homeProfile}
          route={route}
          workspaceName={homeWorkspaces.find((workspace) => workspace.id === activeWorkspaceId)?.name}
          designTitle={designName}
          designs={homeDesigns}
          selectedDesignId={selectedDesignId}
          aiConnection={aiConnection}
          currentUser={homeProfile}
          notifications={notifications}
          onHome={() => applyRoute({ screen: 'home' })}
          onWorkspace={() => applyRoute({ screen: 'workspace', workspaceId: activeWorkspaceId })}
          onSelectDesign={selectBackendDesign}
          onAdmin={() => applyRoute({ screen: 'admin' })}
          onOpenNotifications={() => setNotificationsOpen(true)}
          onLogout={() => void signOut()}
        />
        <StatelessModeBanner storageStatus={storageStatus} />
        <AdminConsole
          users={users}
          activeUserId={activeUserId}
          workspaces={homeWorkspaces}
          storageStatus={storageStatus}
          signInConfig={signInConfig}
          aiProviderConfig={aiConnection}
          mcpConfig={mcpConfig}
          catalogAssets={catalogAssets}
          error={adminError}
          onAddUser={addAdminUser}
          onSaveUser={saveAdminUser}
          onDeleteUser={(userId) => void removeAdminUser(userId)}
          onSaveSignIn={(config) => void saveSignIn(config)}
          onSaveAIProvider={(config) => void saveAIProvider(config)}
          onSaveMCP={(config) => void saveMCP(config)}
          onAddCatalogAsset={addCatalogAsset}
          onSaveCatalogAsset={saveCatalogAsset}
          onDeleteCatalogAsset={removeCatalogAsset}
          onRefreshStorage={refreshStorage}
          onTestDatabase={testDatabaseStorage}
          onConfigureDatabase={configureDatabaseStorage}
          onMigrateStorageUsers={migrateCachedUsersToDatabase}
          activeSection={route.screen === 'admin' ? route.section ?? 'overview' : 'overview'}
          onSelectSection={(section) => applyRoute({ screen: 'admin', section })}
        />
      </div>
    )
  }

  if (view === 'home') {
    return (
      <div className="screen-shell">
        <AppNavbar
          profile={homeProfile}
          route={route}
          workspaceName={homeWorkspaces.find((workspace) => workspace.id === activeWorkspaceId)?.name}
          designTitle={designName}
          designs={homeDesigns}
          selectedDesignId={selectedDesignId}
          aiConnection={aiConnection}
          currentUser={homeProfile}
          notifications={notifications}
          onHome={() => applyRoute({ screen: 'home' })}
          onWorkspace={() => applyRoute({ screen: 'workspace', workspaceId: activeWorkspaceId })}
          onSelectDesign={selectBackendDesign}
          onAdmin={() => applyRoute({ screen: 'admin' })}
          onOpenNotifications={() => setNotificationsOpen(true)}
          onLogout={() => void signOut()}
        />
        <StatelessModeBanner storageStatus={storageStatus} />
        <HomeScreen
          workspaces={homeWorkspaces}
          designs={homeDesigns}
          activeWorkspaceId={activeWorkspaceId}
          newWorkspaceName={newWorkspaceName}
          error={homeError}
          action={homeAction}
          onWorkspaceNameChange={setNewWorkspaceName}
          onSelectWorkspace={(workspaceId) => applyRoute({ screen: 'workspace', workspaceId })}
          onCreateWorkspace={handleCreateWorkspace}
          onDeleteWorkspace={requestDeleteWorkspace}
          onOpenDesign={selectBackendDesign}
          onCloneDesign={requestCloneDesign}
          onDeleteDesign={requestDeleteDesign}
          onNewDesign={openNewDesignBrief}
          onRefresh={() => refreshHome(activeWorkspaceId)}
        />
        {newDesignBriefOpen ? (
          <NewDesignBriefModal
            title={newDesignTitle}
            brief={newDesignBrief}
            isCreating={homeAction === 'design'}
            onTitleChange={setNewDesignTitle}
            onBriefChange={setNewDesignBrief}
            onCancel={() => setNewDesignBriefOpen(false)}
            onCreate={() => void createNewDesignFromBrief()}
          />
        ) : null}
        {deleteTarget ? (
          <ConfirmDeleteModal
            target={deleteTarget}
            isDeleting={homeAction === 'delete-workspace' || homeAction === 'delete-design' || homeAction === 'delete-version'}
            onCancel={() => setDeleteTarget(null)}
            onConfirm={() => void confirmDeleteTarget()}
          />
        ) : null}
        {cloneTarget ? (
          <CloneDesignModal
            sourceDesign={cloneTarget}
            versions={cloneVersions}
            selectedVersionId={cloneVersionId}
            title={cloneTitle}
            state={cloneState}
            error={cloneError}
            onTitleChange={setCloneTitle}
            onVersionChange={setCloneVersionId}
            onCancel={() => {
              setCloneTarget(null)
              setCloneError(null)
              setCloneState('idle')
            }}
            onClone={() => void confirmCloneDesign()}
          />
        ) : null}
        {notificationsOpen ? (
          <NotificationsModal
            notifications={notifications}
            users={users}
            onMarkRead={(notificationId) => void markNotificationAsRead(notificationId)}
            onClose={() => setNotificationsOpen(false)}
          />
        ) : null}
      </div>
    )
  }

  return (
    <div className="screen-shell">
      <AppNavbar
        profile={homeProfile}
        route={route}
        workspaceName={backendSync.workspace?.name ?? homeWorkspaces.find((workspace) => workspace.id === activeWorkspaceId)?.name}
        designTitle={designName}
        designs={workspaceDesigns}
        selectedDesignId={selectedDesignId}
        designVersions={designVersions}
        versionState={versionState}
        previewVersion={versionPreview?.version ?? null}
        reviews={reviews}
        aiConnection={aiConnection}
        currentUser={homeProfile}
        notifications={notifications}
        onHome={() => applyRoute({ screen: 'home' })}
        onWorkspace={() => applyRoute({ screen: 'workspace', workspaceId: activeWorkspaceId })}
        onSelectDesign={selectBackendDesign}
        onVersionStatusChange={(versionId, status) => void changeVersionStatus(versionId, status)}
        onVersionReadyForReview={markVersionReadyForReview}
        onDeleteVersion={requestDeleteVersion}
        onViewVersion={viewVersion}
        onAdmin={() => applyRoute({ screen: 'admin' })}
        onOpenNotifications={() => setNotificationsOpen(true)}
        onLogout={() => void signOut()}
      />
      <StatelessModeBanner storageStatus={storageStatus} />
      <main className={`app-shell ${rightRailOpen ? '' : 'right-rail-collapsed'}`}>
        <aside className="left-rail">
          <div className="brand">
            <Boxes size={20} />
            <div>
              <strong>{backendSync.workspace?.name ?? 'Guest Workspace'}</strong>
              <span>{workspaceDesigns.length} design{workspaceDesigns.length === 1 ? '' : 's'}</span>
            </div>
          </div>

          <section className="panel-section">
            <div className="section-title catalog-heading">
              <span>{catalogRailMode === 'blocks' ? 'Components' : 'Enterprise Catalog'}</span>
              <small>{catalogRailMode === 'blocks' ? `${componentCatalog.length} blocks` : `${catalogAssets.length} assets`}</small>
            </div>
            <div className="rail-mode-switch" role="group" aria-label="Component source">
              <button className={catalogRailMode === 'blocks' ? 'active' : ''} type="button" onClick={() => setCatalogRailMode('blocks')}>
                Blocks
              </button>
              <button className={catalogRailMode === 'catalog' ? 'active' : ''} type="button" onClick={() => setCatalogRailMode('catalog')}>
                Catalog
              </button>
            </div>
            {catalogRailMode === 'blocks' ? (
              <div className="catalog-list">
                {componentCatalog.map((item) => (
                  <button
                    className="catalog-item"
                    key={item.type}
                    draggable={!isCanvasReadOnly}
                    disabled={isCanvasReadOnly}
                    onDragStart={(event) => {
                      if (isCanvasReadOnly) return
                      event.dataTransfer.setData('application/x-sde-component-type', item.type)
                      event.dataTransfer.setData('text/plain', item.type)
                      event.dataTransfer.effectAllowed = 'copy'
                    }}
                    onClick={() => addComponent(item.type)}
                    title={isCanvasReadOnly ? 'Switch to edit mode to add components' : item.description}
                  >
                    <span className="catalog-icon-chip" style={{ '--component-color': item.color } as CSSProperties}>
                      <CatalogGlyph type={item.type} />
                    </span>
                    <span>
                      <strong>{item.label}</strong>
                      <small>{item.description}</small>
                    </span>
                    <span className="catalog-add-icon">
                      <Plus size={15} />
                    </span>
                  </button>
                ))}
              </div>
            ) : (
              <div className="enterprise-catalog-browser">
                <label className="rail-search">
                  <Search size={14} />
                  <input
                    value={catalogSearch}
                    onChange={(event) => setCatalogSearch(event.target.value)}
                    placeholder="Search shared services..."
                  />
                </label>
                <select
                  className="rail-select compact"
                  value={catalogTypeFilter}
                  onChange={(event) => setCatalogTypeFilter(event.target.value)}
                  aria-label="Catalog type filter"
                >
                  <option value="all">All asset types</option>
                  {catalogTypeOptions.map((item) => (
                    <option key={item.type} value={item.type}>
                      {item.label}
                    </option>
                  ))}
                </select>
                <div className="enterprise-catalog-list">
                  {filteredCatalogAssets.length ? filteredCatalogAssets.map((asset) => {
                    const item = getCatalogItem(asset.type as ComponentType)
                    return (
                      <button
                        className="enterprise-catalog-item"
                        key={asset.id}
                        draggable={!isCanvasReadOnly}
                        disabled={isCanvasReadOnly}
                        onDragStart={(event) => {
                          if (isCanvasReadOnly) return
                          event.dataTransfer.setData('application/x-stratum-catalog-asset-id', asset.id)
                          event.dataTransfer.effectAllowed = 'copy'
                        }}
                        onClick={() => addCatalogAssetToCanvas(asset)}
                        title={isCanvasReadOnly ? 'Switch to edit mode to add catalog assets' : `Add ${asset.name} as a shared component`}
                      >
                        <span className="catalog-icon-chip" style={{ '--component-color': item.color } as CSSProperties}>
                          <CatalogGlyph type={item.type} />
                        </span>
                        <span>
                          <strong>{asset.name}</strong>
                          <small>{item.label} • {asset.owner || 'No owner'}</small>
                        </span>
                        <em>{catalogAssetUsageLabel(asset.usedInDesignCount)}</em>
                      </button>
                    )
                  }) : (
                    <div className="enterprise-catalog-empty">
                      <strong>No matching assets</strong>
                      <span>Try a broader search or ask an admin to add the service.</span>
                    </div>
                  )}
                </div>
              </div>
            )}
          </section>

          <section className="panel-section draft-actions">
            <button className="command" onClick={exportDesign}>
              <Download size={16} /> Export JSON
            </button>
            <button className="command ghost" onClick={resetDraft}>
              <RotateCcw size={16} /> Reset local draft
            </button>
          </section>
        </aside>

        <section className="workspace">
          <header className="top-bar">
            <input
              className="title-input"
              value={designName}
              disabled={isCanvasReadOnly}
              onChange={(event) => renameDesign(event.target.value)}
            />
            <div className="top-bar-status">
              <span className={`canvas-state-pill ${versionPreview ? 'preview' : canvasMode}`}>
                {versionPreview
                  ? `Viewing v${versionPreview.version.versionNumber} · ${versionStatusLabel(versionPreview.version.status)}`
                  : `${canvasMode === 'design' ? 'Editing' : canvasMode.charAt(0).toUpperCase() + canvasMode.slice(1)} · ${
                      designVersions[0] ? `v${designVersions[0].versionNumber} ${versionStatusLabel(designVersions[0].status)}` : 'unsaved draft'
                    }`}
              </span>
              {saveState === 'error' ? <span className="save-error">Save failed</span> : null}
              {analysisState === 'ready' && analysisReport ? (
                <button className="analysis-pill button-pill" type="button" onClick={() => openAnalysisModal()}>
                  Score {analysisReport.score}
                </button>
              ) : null}
              {analysisState === 'error' ? <span className="save-error">Analysis failed</span> : null}
            </div>
          </header>
        <div className="canvas-host">
          <div className={`floating-canvas-toolbar ${toolbarExpanded ? 'expanded' : 'collapsed'}`} role="toolbar" aria-label="Canvas actions">
            {!toolbarExpanded ? (
              <button className="toolbar-expander" type="button" onClick={() => setToolbarExpanded(true)} title="Open canvas tools">
                <ChevronLeft size={16} />
                <span>Tools</span>
                <strong>{canvasMode === 'design' ? 'Edit' : canvasMode === 'comment' ? 'Comment' : canvasMode === 'journey' ? 'Journey' : 'View'}</strong>
              </button>
            ) : (
              <>
            <button className="toolbar-collapse" type="button" onClick={() => setToolbarExpanded(false)} title="Collapse canvas tools">
              <ChevronRight size={16} />
            </button>
            <div className="mode-switch" role="group" aria-label="Canvas mode">
              <button
                className={canvasMode === 'design' ? 'active' : ''}
                type="button"
                onClick={() => {
                  if (versionPreview) exitVersionPreview()
                  setCanvasMode('design')
                  setContextPanel('inspector')
                }}
                title="Edit design"
              >
                <Pencil size={14} /> <span>Edit</span>
              </button>
              <button
                className={canvasMode === 'view' ? 'active' : ''}
                type="button"
                onClick={() => {
                  setCanvasMode('view')
                  setContextPanel('review')
                  setRightRailOpen(true)
                  setSelectedComponentId(null)
                  setSelectedConnectorId(null)
                }}
                title="View design"
              >
                <Eye size={14} /> <span>View</span>
              </button>
              <button
                className={canvasMode === 'comment' ? 'active' : ''}
                type="button"
                onClick={() => {
                  setCanvasMode('comment')
                  setContextPanel('review')
                  setRightRailOpen(true)
                }}
                title="Comment and review"
              >
                <MessageSquare size={14} /> <span>Comment</span>
              </button>
              <button
                className={canvasMode === 'journey' ? 'active' : ''}
                type="button"
                onClick={() => {
                  setCanvasMode('journey')
                  setContextPanel('journey')
                  setRightRailOpen(true)
                  setSelectedComponentId(null)
                  setSelectedConnectorId(null)
                }}
                title="Journey mode"
              >
                <Route size={14} /> <span>Journey</span>
              </button>
            </div>
            <button className="command compact" onClick={saveDraft} disabled={Boolean(versionPreview)} title={versionPreview ? 'Exit version preview before saving' : 'Save design'}>
              <Save size={16} /> <span>{saveState === 'saving' ? 'Saving' : saveState === 'saved' ? 'Saved' : 'Save'}</span>
            </button>
            <button
              className="command compact primary-compact"
              onClick={() => openAnalysisModal({ run: analysisState !== 'running' })}
              disabled={!selectedDesignId}
              title="Analyse design"
            >
              <Sparkles size={16} /> <span>{analysisState === 'running' ? 'Analysing' : 'Analyse'}</span>
            </button>
            <button className="command compact" onClick={() => setVisionOpen(true)} disabled={!selectedDesignId} title="Import from image">
              <ImageIcon size={16} /> <span>Vision</span>
            </button>
            <button className="command compact" onClick={() => setDocsModalOpen(true)} title="Design docs">
              <BookOpen size={16} /> <span>Docs</span>
            </button>
              </>
            )}
          </div>
          {versionPreview ? (
            <div className="version-preview-banner" role="status">
              <Eye size={16} />
              <span>
                You are viewing saved version v{versionPreview.version.versionNumber}. The canvas is read-only and changes are disabled.
              </span>
              <button type="button" onClick={exitVersionPreview}>Back to editable draft</button>
            </div>
          ) : null}
          <button
            className={`context-rail-toggle ${rightRailOpen ? 'open' : 'closed'}`}
            type="button"
            onClick={() => setRightRailOpen((value) => !value)}
            title={rightRailOpen ? 'Collapse context panel' : 'Open context panel'}
          >
            {rightRailOpen ? <ChevronRight size={17} /> : <ChevronLeft size={17} />}
          </button>
          <CanvasErrorBoundary
            onResetCanvas={() => resetDraft()}
          >
            <ReactFlowCanvasProvider
              design={canvasDesign}
              selectedComponentId={selectedComponentId}
              selectedConnectorId={selectedConnectorId}
              selectedComponentIds={selectedComponentIdList}
              selectedConnectorIds={selectedConnectorIdList}
              readOnly={isCanvasReadOnly}
              traversalFocus={traversalFocus}
              onSelectionChange={handleCanvasSelectionChange}
              onSelectComponent={(componentId) => {
                setSelectedComponentId(componentId || null)
                setSelectedComponentIds(componentId ? new Set([componentId]) : new Set())
                if (componentId) setSelectedConnectorId(null)
                if (componentId) setSelectedConnectorIds(new Set())
                if (componentId) {
                  setContextPanel('inspector')
                  setRightRailOpen(true)
                }
              }}
              onSelectConnector={(connectorId) => {
                setSelectedConnectorId(connectorId || null)
                setSelectedConnectorIds(connectorId ? new Set([connectorId]) : new Set())
                if (connectorId) setSelectedComponentId(null)
                if (connectorId) setSelectedComponentIds(new Set())
                if (connectorId) {
                  setContextPanel('inspector')
                  setRightRailOpen(true)
                }
              }}
              onMoveComponent={moveReactFlowComponent}
              onResizeComponent={resizeReactFlowComponent}
              onConnectComponents={(fromComponentId, toComponentId) => setPendingConnector({ fromComponentId, toComponentId })}
              onReconnectConnector={reconnectConnector}
              onDropComponent={createReactFlowComponent}
              onDropCatalogAsset={(assetId, position) => {
                const asset = catalogAssets.find((item) => item.id === assetId)
                if (asset) createReactFlowCatalogAsset(asset, position)
              }}
              onDuplicateComponent={duplicateReactFlowComponent}
              onDeleteComponents={deleteComponents}
              onDeleteConnectors={deleteConnectors}
            />
          </CanvasErrorBoundary>
        </div>
      </section>

        <aside className="right-rail">
          <div className="context-drawer-header">
            <div>
              <strong>Workspace context</strong>
              <span>Inspect, explain, and review this system</span>
            </div>
          </div>
          <div className="context-tabs" role="tablist" aria-label="Canvas context">
            <button className={contextPanel === 'inspector' ? 'active' : ''} type="button" onClick={() => setContextPanel('inspector')}>
              Inspector
            </button>
            <button className={contextPanel === 'requirements' ? 'active' : ''} type="button" onClick={() => setContextPanel('requirements')}>
              Requirements
            </button>
            <button className={contextPanel === 'journey' ? 'active' : ''} type="button" onClick={() => setContextPanel('journey')}>
              Journey
            </button>
            <button className={contextPanel === 'review' ? 'active' : ''} type="button" onClick={() => setContextPanel('review')}>
              Review
            </button>
          </div>
          {contextPanel === 'review' ? (
            <ReviewWorkspace
              mode={canvasMode === 'comment' ? 'comment' : 'view'}
              users={users}
              activeUserId={activeUserId}
              comments={comments}
              reviews={reviews}
              activeVersion={versionPreview?.version ?? designVersions[0] ?? null}
              selectedComponent={selectedComponent}
              selectedConnector={selectedConnector}
              commentDraft={commentDraft}
              reviewSummaryDrafts={reviewSummaryDrafts}
              isSaving={collaborationState === 'saving'}
              error={collaborationError}
              offline={collaborationOffline}
              onCommentDraftChange={setCommentDraft}
              onSubmitComment={() => void submitDesignComment()}
              onRequestReview={() => {
                const targetVersion = versionPreview?.version ?? designVersions[0] ?? null
                setRequestReviewVersion(targetVersion)
                setRequestReviewOpen(true)
              }}
              onReviewSummaryChange={(reviewId, summary) =>
                setReviewSummaryDrafts((current) => ({ ...current, [reviewId]: summary }))
              }
              onCompleteReview={(reviewId, status) => void completeReview(reviewId, status)}
              onRefresh={() => void refreshCollaboration()}
            />
          ) : contextPanel === 'inspector' ? (
            !isCanvasReadOnly && selectedConnector ? (
              <ConnectorInspector
                connector={selectedConnector}
                onChange={updateConnectorFromInspector}
                onDelete={() => deleteConnectors([selectedConnector.id])}
              />
            ) : !isCanvasReadOnly && selectedComponent ? (
              <ComponentInspector
                component={selectedComponent}
                currentDesignId={selectedDesignId ?? design.id}
                workspaceDesigns={workspaceDesigns}
                catalogAssets={catalogAssets}
                onChange={updateComponentFromInspector}
                onDelete={() => deleteComponents([selectedComponent.id])}
                onOpenLinkedDesign={selectBackendDesign}
                onCreateCatalogAsset={async (input) => {
                  const asset = await addCatalogAsset(input)
                  return asset
                }}
              />
            ) : (
              <div className="context-empty">
                <Monitor size={24} />
                <strong>Select a component</strong>
                <span>Pick a node or connector to edit details. Use Requirements, Journey, or Review when you need design context.</span>
              </div>
            )
          ) : contextPanel === 'requirements' ? (
            <section className="analysis-result-panel">
              <div className="inspector-heading">
                <Monitor size={18} />
                <div>
                  <strong>Requirement Brief</strong>
                  <span>Use case, scale, consistency, and SLA context</span>
                </div>
              </div>
              <RequirementPanel design={canvasDesign} onChange={updateDesign} compact readOnly={Boolean(versionPreview)} />
            </section>
          ) : (
            <JourneyPanel
              design={canvasDesign}
              activeJourneyId={activeJourneyId}
              activeStepIndex={activeJourneyStepIndex}
              readOnly={Boolean(versionPreview)}
              onCreatePrimaryJourney={() => {
                createPrimaryJourney()
                setCanvasMode('journey')
              }}
              onSelectJourney={(journeyId) => {
                setActiveJourneyId(journeyId)
                setActiveJourneyStepIndex(0)
                setCanvasMode('journey')
                setSelectedComponentId(null)
                setSelectedConnectorId(null)
              }}
              onChangeJourney={updateJourney}
              onDeleteJourney={deleteJourney}
              onSetStep={setActiveJourneyStepIndex}
            />
          )}
        </aside>
      </main>
      {newDesignBriefOpen ? (
        <NewDesignBriefModal
          title={newDesignTitle}
          brief={newDesignBrief}
          isCreating={homeAction === 'design'}
          onTitleChange={setNewDesignTitle}
          onBriefChange={setNewDesignBrief}
          onCancel={() => setNewDesignBriefOpen(false)}
          onCreate={() => void createNewDesignFromBrief()}
        />
      ) : null}
      {deleteTarget ? (
        <ConfirmDeleteModal
          target={deleteTarget}
          isDeleting={homeAction === 'delete-workspace' || homeAction === 'delete-design' || homeAction === 'delete-version'}
          onCancel={() => setDeleteTarget(null)}
          onConfirm={() => void confirmDeleteTarget()}
        />
      ) : null}
      {analysisModalOpen ? (
        <AnalysisModal
          design={design}
          docs={designDocs}
          analysisState={analysisState}
          analysisReport={analysisReport}
          analysisError={analysisError}
          onClose={() => setAnalysisModalOpen(false)}
          onRunAnalysis={() => void runAnalysis()}
        />
      ) : null}
      {saveDialogOpen ? (
        <SaveDesignModal
          mode={saveVersionMode}
          remarks={saveRemarks}
          hasVersions={designVersions.length > 0}
          isSaving={saveState === 'saving' || versionState === 'saving'}
          onModeChange={setSaveVersionMode}
          onRemarksChange={setSaveRemarks}
          onCancel={() => setSaveDialogOpen(false)}
          onSave={() => void saveDesignFromDialog()}
        />
      ) : null}
      {pendingConnector ? (
        <ConnectorTypeModal
          fromComponent={design.components.find((component) => component.id === pendingConnector.fromComponentId) ?? null}
          toComponent={design.components.find((component) => component.id === pendingConnector.toComponentId) ?? null}
          onCancel={() => setPendingConnector(null)}
          onSelect={completePendingConnector}
        />
      ) : null}
      {docsModalOpen ? (
        <DesignDocsModal
          documents={designDocs}
          activeDocumentId={activeTextDocumentId}
          saveState={docsSaveState}
          dirtyDocIds={dirtyDocIds}
          error={docsError}
          onAdd={() => void addTextDocument()}
          onSelect={setActiveTextDocumentId}
          onChange={updateTextDocument}
          onDelete={(documentId) => void deleteTextDocument(documentId)}
          onSave={(documentId) => void saveTextDocument(documentId)}
          onClose={() => setDocsModalOpen(false)}
        />
      ) : null}
      {copilotOpen ? (
        <CopilotDraftModal
          aiConnection={aiConnection?.enabled && aiConnection.apiKeySet ? aiConnection : null}
          onClose={() => setCopilotOpen(false)}
        />
      ) : null}
      {visionOpen ? (
        <VisionImportModal
          aiConnection={aiConnection?.enabled && aiConnection.apiKeySet ? aiConnection : null}
          onClose={() => setVisionOpen(false)}
        />
      ) : null}
      {requestReviewOpen ? (
        <RequestReviewModal
          users={users}
          activeUserId={activeUserId}
          version={requestReviewVersion ?? versionPreview?.version ?? designVersions[0] ?? null}
          isSaving={collaborationState === 'saving'}
          onCancel={() => {
            setRequestReviewOpen(false)
            setRequestReviewVersion(null)
          }}
          onRequest={(reviewerIds, message) => void submitReviewRequest(reviewerIds, message)}
        />
      ) : null}
      {notificationsOpen ? (
        <NotificationsModal
          notifications={notifications}
          users={users}
          onMarkRead={(notificationId) => void markNotificationAsRead(notificationId)}
          onClose={() => setNotificationsOpen(false)}
        />
      ) : null}
    </div>
  )
}

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

function AdminConsole({
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
  onAddUser: (input: { displayName: string; email: string; role: string; password?: string }) => Promise<void>
  onSaveUser: (user: BackendUser & { password?: string }) => Promise<void>
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
  const [newUser, setNewUser] = useState({ displayName: '', email: '', role: 'member', password: '' })
  const [newAsset, setNewAsset] = useState({ name: '', type: 'compute.service', owner: '', description: '', criticality: 'medium' })
  const [draftUsers, setDraftUsers] = useState<Array<BackendUser & { password?: string }>>(users)
  const [draftAssets, setDraftAssets] = useState<BackendCatalogAsset[]>(catalogAssets)
  const [editingUserId, setEditingUserId] = useState<string | null>(null)
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
          fetchWorkspaceDesigns(accessWorkspaceId),
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

  function updateDraftUser(user: BackendUser, patch: Partial<BackendUser & { password?: string }>) {
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
    await onAddUser({ ...newUser, email, password: newUser.password.trim() || undefined })
    setNewUser({ displayName: '', email: '', role: 'member', password: '' })
  }

  async function submitUserSave(user: BackendUser & { password?: string }) {
    const email = user.email.trim().toLowerCase()
    if (draftUsers.some((item) => item.id !== user.id && item.email.trim().toLowerCase() === email)) {
      setUserError('A user with this email already exists.')
      return
    }
    setUserError(null)
    await onSaveUser({ ...user, email, password: user.password?.trim() || undefined })
    setEditingUserId(null)
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
            <Field label="Temporary password" type="password" value={newUser.password} onChange={(password) => setNewUser((current) => ({ ...current, password }))} />
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
                      <Field label={user.passwordSet ? 'Reset password' : 'Set password'} type="password" value={user.password ?? ''} onChange={(password) => updateDraftUser(user, { password })} />
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
                    <button className="secondary-action compact-action" type="button" onClick={() => setEditingUserId(user.id)}>
                      Edit
                    </button>
                    <button className="text-button danger" type="button" disabled={user.id === activeUserId} onClick={() => onDeleteUser(user.id)}>
                      Delete
                    </button>
                  </div>
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

function CopilotDraftModal({
  aiConnection,
  onClose,
}: {
  aiConnection: AIConnectionMetadata | null
  onClose: () => void
}) {
  const [prompt, setPrompt] = useState('')
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="ai-settings-modal" role="dialog" aria-modal="true" aria-labelledby="copilot-title">
        <div className="modal-heading">
          <div>
            <p className="eyebrow">Copilot</p>
            <h2 id="copilot-title">Generate a first draft</h2>
            <span>{aiConnection ? `${aiConnection.provider} / ${aiConnection.model}` : 'Connect an AI provider to enable draft generation.'}</span>
          </div>
        </div>
        <TextareaField
          label="Design prompt"
          value={prompt}
          onChange={setPrompt}
        />
        <div className="analysis-empty">
          <strong>Coming next</strong>
          <span>The backend contract will generate structured components, connectors, and requirement assumptions from this prompt.</span>
        </div>
        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onClose}>
            Close
          </button>
          <button className="primary-action" type="button" disabled>
            Generate draft
          </button>
        </div>
      </section>
    </div>
  )
}

function VisionImportModal({
  aiConnection,
  onClose,
}: {
  aiConnection: AIConnectionMetadata | null
  onClose: () => void
}) {
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const canGenerate = Boolean(aiConnection && selectedFile)

  useEffect(() => {
    if (!selectedFile) {
      setPreviewUrl(null)
      return
    }
    const objectUrl = URL.createObjectURL(selectedFile)
    setPreviewUrl(objectUrl)
    return () => URL.revokeObjectURL(objectUrl)
  }, [selectedFile])

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="vision-import-modal" role="dialog" aria-modal="true" aria-labelledby="vision-import-title">
        <div className="modal-heading">
          <div>
            <p className="eyebrow">AI Vision</p>
            <h2 id="vision-import-title">Import a whiteboard sketch</h2>
            <span>
              Upload a whiteboard image so Stratum can later extract components, flows, and assumptions into a structured design.
            </span>
          </div>
        </div>

        <label className={`vision-dropzone ${previewUrl ? 'has-preview' : ''}`}>
          <input
            type="file"
            accept="image/png,image/jpeg,image/webp,image/gif"
            onChange={(event) => setSelectedFile(event.target.files?.[0] ?? null)}
          />
          {previewUrl ? (
            <img src={previewUrl} alt={selectedFile?.name ?? 'Selected whiteboard'} />
          ) : (
            <div>
              <Upload size={24} />
              <strong>Choose whiteboard image</strong>
              <span>PNG, JPG, WebP, or GIF</span>
            </div>
          )}
        </label>

        {selectedFile ? (
          <div className="vision-file-summary">
            <ImageIcon size={18} />
            <div>
              <strong>{selectedFile.name}</strong>
              <span>{Math.max(1, Math.round(selectedFile.size / 1024))} KB</span>
            </div>
          </div>
        ) : null}

        <div className="analysis-empty">
          <strong>{aiConnection ? 'Vision pipeline placeholder' : 'AI provider required'}</strong>
          <span>
            {aiConnection
              ? 'Next backend step: send this image to the configured vision model and return structured components and connectors for review.'
              : 'Configure an AI provider before generating architecture from images.'}
          </span>
        </div>

        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onClose}>
            Close
          </button>
          <button className="primary-action" type="button" disabled={!canGenerate}>
            Generate architecture
          </button>
        </div>
      </section>
    </div>
  )
}

function AnalysisModal({
  design,
  docs,
  analysisState,
  analysisReport,
  analysisError,
  onClose,
  onRunAnalysis,
}: {
  design: DesignDocument
  docs: BackendDesignDoc[]
  analysisState: 'idle' | 'running' | 'ready' | 'error'
  analysisReport: DesignAnalysisReport | null
  analysisError: string | null
  onClose: () => void
  onRunAnalysis: () => void
}) {
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="analysis-modal" role="dialog" aria-modal="true" aria-labelledby="analysis-title">
        <div className="analysis-modal-header">
          <div className="modal-heading">
            <p className="eyebrow">Design review</p>
            <h2 id="analysis-title">Analysis workspace</h2>
            <span>Run deterministic suites and review findings away from the canvas authoring panel.</span>
          </div>
          <div className="analysis-modal-actions">
            <button className="secondary-action" type="button" onClick={onClose}>
              Close
            </button>
            <button className="primary-action" type="button" onClick={onRunAnalysis} disabled={analysisState === 'running'}>
              <Sparkles size={17} /> {analysisState === 'running' ? 'Analysing...' : 'Run analysis'}
            </button>
          </div>
        </div>

        <div className="analysis-modal-body">
          <DesignAnalysisBase design={design} docs={docs} />

          <section className="analysis-result-panel analysis-report-panel">
            <div className="inspector-heading">
              <Sparkles size={18} />
              <div>
                <strong>Findings</strong>
                <span>Requirements, graph integrity, traffic, consistency, availability, and security</span>
              </div>
            </div>

            {analysisState === 'error' ? <div className="analysis-error">{analysisError}</div> : null}
            {analysisReport ? (
              <div className="analysis-report">
                <div className="analysis-score-card">
                  <strong>{analysisReport.score}</strong>
                  <span>Readiness score</span>
                </div>
                <p>{analysisReport.summary}</p>
                {analysisReport.workflow?.length ? (
                  <div className="analysis-workflow">
                    {analysisReport.workflow.map((step) => (
                      <article className={`workflow-step ${step.status}`} key={step.id}>
                        <span>{step.status}</span>
                        <strong>{step.label}</strong>
                        <small>{step.detail}</small>
                      </article>
                    ))}
                  </div>
                ) : null}
                {analysisReport.aiReview ? (
                  <div className="ai-review-block">
                    <div className="admin-section-title">
                      <span>AI synthesis</span>
                      <small>
                        {analysisReport.aiReview.provider
                          ? `${analysisReport.aiReview.provider} / ${analysisReport.aiReview.model ?? 'model'}`
                          : analysisReport.aiReview.status}
                      </small>
                    </div>
                    {analysisReport.aiReview.error ? <div className="analysis-error">{analysisReport.aiReview.error}</div> : null}
                    {analysisReport.aiReview.strengths?.length ? (
                      <div className="ai-review-list strengths">
                        <strong>Strengths</strong>
                        {analysisReport.aiReview.strengths.map((strength) => (
                          <span key={strength}>{strength}</span>
                        ))}
                      </div>
                    ) : null}
                    {analysisReport.aiReview.risks?.length ? (
                      <div className="analysis-finding-list">
                        <strong>AI risks</strong>
                        {analysisReport.aiReview.risks.map((risk) => (
                          <article className={`analysis-finding ${risk.severity}`} key={`risk-${risk.suite}-${risk.title}`}>
                            <span>{risk.severity} · {risk.suite}</span>
                            <strong>{risk.title}</strong>
                            <p>{risk.detail}</p>
                            <small>{risk.impact}</small>
                            <em>{risk.recommendation}</em>
                          </article>
                        ))}
                      </div>
                    ) : null}
                    {analysisReport.aiReview.recommendations?.length ? (
                      <div className="analysis-finding-list">
                        <strong>AI recommendations</strong>
                        {analysisReport.aiReview.recommendations.map((recommendation) => (
                          <article className={`analysis-finding ${recommendation.severity}`} key={`rec-${recommendation.suite}-${recommendation.title}`}>
                            <span>{recommendation.severity} · {recommendation.suite}</span>
                            <strong>{recommendation.title}</strong>
                            <p>{recommendation.detail}</p>
                            <small>{recommendation.impact}</small>
                            <em>{recommendation.recommendation}</em>
                          </article>
                        ))}
                      </div>
                    ) : null}
                    {analysisReport.aiReview.openQuestions?.length ? (
                      <div className="ai-review-list questions">
                        <strong>Open questions</strong>
                        {analysisReport.aiReview.openQuestions.map((question) => (
                          <span key={question}>{question}</span>
                        ))}
                      </div>
                    ) : null}
                  </div>
                ) : null}
                <div className="analysis-finding-list">
                  {analysisReport.findings.length ? (
                    analysisReport.findings.map((finding) => (
                      <article className={`analysis-finding ${finding.severity}`} key={`${finding.suite}-${finding.title}`}>
                        <span>{finding.severity} · {finding.suite}</span>
                        <strong>{finding.title}</strong>
                        <p>{finding.detail}</p>
                        {finding.impact ? <small>{finding.impact}</small> : null}
                        {finding.recommendation ? <em>{finding.recommendation}</em> : null}
                      </article>
                    ))
                  ) : (
                    <div className="analysis-empty">
                      <strong>No major gaps found</strong>
                      <span>The structured design has enough base signals for deeper evaluation.</span>
                    </div>
                  )}
                </div>
              </div>
            ) : (
              <div className="analysis-empty">
                <strong>No analysis yet</strong>
                <span>Run analysis after sketching the first meaningful design path.</span>
              </div>
            )}
          </section>
        </div>
      </section>
    </div>
  )
}

function SaveDesignModal({
  mode,
  remarks,
  hasVersions,
  isSaving,
  onModeChange,
  onRemarksChange,
  onCancel,
  onSave,
}: {
  mode: 'override' | 'new'
  remarks: string
  hasVersions: boolean
  isSaving: boolean
  onModeChange: (mode: 'override' | 'new') => void
  onRemarksChange: (value: string) => void
  onCancel: () => void
  onSave: () => void
}) {
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="save-design-modal" role="dialog" aria-modal="true" aria-labelledby="save-design-title">
        <div className="modal-heading">
          <p className="eyebrow">Versioned save</p>
          <h2 id="save-design-title">Save design</h2>
          <span>Choose whether this save updates the current editable version or starts a new draft version.</span>
        </div>
        <div className="save-mode-options" role="radiogroup" aria-label="Save target">
          <label className={mode === 'override' ? 'active' : ''}>
            <input
              type="radio"
              name="save-version-mode"
              value="override"
              checked={mode === 'override'}
              disabled={!hasVersions}
              onChange={() => onModeChange('override')}
            />
            <span>
              <strong>Override current version</strong>
              <small>{hasVersions ? 'Save changes into the current draft/review version.' : 'Create a first version before overrides are available.'}</small>
            </span>
          </label>
          <label className={mode === 'new' ? 'active' : ''}>
            <input
              type="radio"
              name="save-version-mode"
              value="new"
              checked={mode === 'new'}
              onChange={() => onModeChange('new')}
            />
            <span>
              <strong>Create new version</strong>
              <small>Snapshot the current design as a new draft version.</small>
            </span>
          </label>
        </div>
        <label className="field">
          <span>Remarks optional</span>
          <textarea
            value={remarks}
            onChange={(event) => onRemarksChange(event.target.value)}
            placeholder="What changed in this save?"
            rows={4}
          />
        </label>
        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onCancel} disabled={isSaving}>
            Cancel
          </button>
          <button className="primary-action" type="button" onClick={onSave} disabled={isSaving}>
            <Save size={16} /> {isSaving ? 'Saving...' : 'Save design'}
          </button>
        </div>
      </section>
    </div>
  )
}

function ConnectorTypeModal({
  fromComponent,
  toComponent,
  onCancel,
  onSelect,
}: {
  fromComponent: DesignComponent | null
  toComponent: DesignComponent | null
  onCancel: () => void
  onSelect: (type: ConnectorType) => void
}) {
  return (
    <div className="modal-backdrop connector-modal-backdrop" role="presentation">
      <section className="connector-type-modal" role="dialog" aria-modal="true" aria-labelledby="connector-type-title">
        <div className="modal-heading">
          <p className="eyebrow">New connection</p>
          <h2 id="connector-type-title">Choose relationship type</h2>
          <span>
            {(fromComponent?.name || 'Source')} to {(toComponent?.name || 'Target')}
          </span>
        </div>
        <div className="connector-type-options">
          {connectorTypeOptions.map((option) => (
            <button type="button" key={option.value} onClick={() => onSelect(option.value)}>
              <strong>{option.label}</strong>
              <span>{option.description}</span>
            </button>
          ))}
        </div>
        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onCancel}>
            Cancel
          </button>
        </div>
      </section>
    </div>
  )
}

function JourneyPanel({
  design,
  activeJourneyId,
  activeStepIndex,
  readOnly = false,
  onCreatePrimaryJourney,
  onSelectJourney,
  onChangeJourney,
  onDeleteJourney,
  onSetStep,
}: {
  design: DesignDocument
  activeJourneyId: string | null
  activeStepIndex: number
  readOnly?: boolean
  onCreatePrimaryJourney: () => void
  onSelectJourney: (journeyId: string) => void
  onChangeJourney: (journey: DesignJourney) => void
  onDeleteJourney: (journeyId: string) => void
  onSetStep: (stepIndex: number) => void
}) {
  const journeys = design.journeys ?? []
  const activeJourney = journeys.find((journey) => journey.id === activeJourneyId) ?? journeys[0] ?? null
  const activeStep = activeJourney?.steps[activeStepIndex] ?? null

  function updateActiveJourney(patch: Partial<Pick<DesignJourney, 'title' | 'description'>>) {
    if (!activeJourney) return
    onChangeJourney({ ...activeJourney, ...patch })
  }

  function updateStep(stepId: string, patch: Partial<Pick<DesignJourney['steps'][number], 'title' | 'description'>>) {
    if (!activeJourney) return
    onChangeJourney({
      ...activeJourney,
      steps: activeJourney.steps.map((step) => (step.id === stepId ? { ...step, ...patch } : step)),
    })
  }

  return (
    <section className="analysis-result-panel journey-panel">
      <div className="inspector-heading">
        <Route size={18} />
        <div>
          <strong>Traversal</strong>
          <span>Explain request direction and user journeys</span>
        </div>
      </div>

      <div className="journey-toolbar">
        <button className="primary-action full-width" type="button" onClick={onCreatePrimaryJourney} disabled={readOnly}>
          <Route size={16} /> Generate primary journey
        </button>
      </div>

      {journeys.length ? (
        <>
          <label className="field">
            <span>Active journey</span>
            <select value={activeJourney?.id ?? ''} onChange={(event) => onSelectJourney(event.target.value)}>
              {journeys.map((journey) => (
                <option key={journey.id} value={journey.id}>
                  {journey.title || 'Untitled journey'}
                </option>
              ))}
            </select>
          </label>

          {activeJourney ? (
            <div className="journey-editor">
              <Field label="Journey title" value={activeJourney.title} onChange={(title) => updateActiveJourney({ title })} disabled={readOnly} />
              <TextareaField
                label="Journey summary"
                value={activeJourney.description}
                onChange={(description) => updateActiveJourney({ description })}
                disabled={readOnly}
              />

              <div className="journey-stepper">
                <button
                  className="icon-command"
                  type="button"
                  title="Previous step"
                  disabled={activeStepIndex <= 0}
                  onClick={() => onSetStep(Math.max(0, activeStepIndex - 1))}
                >
                  <SkipBack size={16} />
                </button>
                <span>
                  Step {activeJourney.steps.length ? activeStepIndex + 1 : 0} of {activeJourney.steps.length}
                </span>
                <button
                  className="icon-command"
                  type="button"
                  title="Next step"
                  disabled={activeStepIndex >= activeJourney.steps.length - 1}
                  onClick={() => onSetStep(Math.min(activeJourney.steps.length - 1, activeStepIndex + 1))}
                >
                  <SkipForward size={16} />
                </button>
              </div>

              {activeStep ? (
                <div className="journey-active-step">
                  <Field label="Step title" value={activeStep.title} onChange={(title) => updateStep(activeStep.id, { title })} />
                  <TextareaField
                    label="Step narration"
                    value={activeStep.description}
                    onChange={(description) => updateStep(activeStep.id, { description })}
                    disabled={readOnly}
                  />
                </div>
              ) : (
                <div className="analysis-empty">
                  <strong>No steps yet</strong>
                  <span>Add connectors between components, then generate a journey again.</span>
                </div>
              )}

              <div className="journey-step-list">
                {activeJourney.steps.map((step, index) => (
                  <button
                    className={index === activeStepIndex ? 'active' : ''}
                    type="button"
                    key={step.id}
                    onClick={() => onSetStep(index)}
                  >
                    <span>{index + 1}</span>
                    <strong>{step.title || 'Untitled step'}</strong>
                  </button>
                ))}
              </div>

              <button className="text-button danger" type="button" onClick={() => onDeleteJourney(activeJourney.id)} disabled={readOnly}>
                Delete journey
              </button>
            </div>
          ) : null}
        </>
      ) : (
        <div className="analysis-empty">
          <strong>No traversal yet</strong>
          <span>Create components and connectors, then generate a path for reviewers to follow.</span>
        </div>
      )}
    </section>
  )
}

function DesignDocsModal({
  documents,
  activeDocumentId,
  saveState,
  dirtyDocIds,
  error,
  onAdd,
  onSelect,
  onChange,
  onDelete,
  onSave,
  onClose,
}: {
  documents: BackendDesignDoc[]
  activeDocumentId: string | null
  saveState: 'idle' | 'loading' | 'saving' | 'saved' | 'error'
  dirtyDocIds: Set<string>
  error: string | null
  onAdd: () => void
  onSelect: (documentId: string) => void
  onChange: (documentId: string, patch: Partial<Pick<BackendDesignDoc, 'title' | 'body'>>) => void
  onDelete: (documentId: string) => void
  onSave: (documentId: string) => void
  onClose: () => void
}) {
  const activeDocument = documents.find((document) => document.id === activeDocumentId) ?? documents[0] ?? null
  const activeDocumentIsDirty = activeDocument ? dirtyDocIds.has(activeDocument.id) : false

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="design-docs-modal" role="dialog" aria-modal="true" aria-labelledby="design-docs-title">
        <div className="analysis-modal-header">
          <div className="modal-heading">
            <p className="eyebrow">Design docs</p>
            <h2 id="design-docs-title">Attached context</h2>
            <span>Capture decisions, assumptions, rollout notes, links, and review narrative for this design.</span>
          </div>
          <div className="analysis-modal-actions">
            <button className="secondary-action" type="button" onClick={onClose}>
              Close
            </button>
            <button
              className="secondary-action"
              type="button"
              onClick={() => activeDocument && onSave(activeDocument.id)}
              disabled={!activeDocument || saveState === 'saving' || saveState === 'loading' || !activeDocumentIsDirty}
            >
              <Save size={16} /> {saveState === 'saving' ? 'Saving...' : saveState === 'saved' ? 'Saved' : 'Save'}
            </button>
            <button className="primary-action" type="button" onClick={onAdd} disabled={saveState === 'saving' || saveState === 'loading'}>
              <FilePlus2 size={16} /> {saveState === 'loading' ? 'Loading...' : 'Add doc'}
            </button>
          </div>
        </div>

        <div className="design-docs-modal-body">
          <aside className="design-doc-list">
            {error ? <div className="analysis-error">{error}</div> : null}
            {documents.length ? (
              documents.map((document) => (
                <button
                  className={document.id === activeDocument?.id ? 'active' : ''}
                  type="button"
                  key={document.id}
                  onClick={() => onSelect(document.id)}
                >
                  <strong>{document.title || 'Untitled doc'}{dirtyDocIds.has(document.id) ? ' (unsaved)' : ''}</strong>
                  <span>{stripHtml(document.body).length ? `${stripHtml(document.body).length} characters` : 'Empty document'}</span>
                </button>
              ))
            ) : (
              <div className="analysis-empty">
                <strong>No docs yet</strong>
                <span>Add a document for decisions, links, or review context.</span>
              </div>
            )}
          </aside>

          <section className="design-doc-main">
            {activeDocument ? (
              <>
                <Field
                  label="Title"
                  value={activeDocument.title}
                  onChange={(title) => onChange(activeDocument.id, { title })}
                />
                <label className="field design-doc-body-field">
                  <span>Body</span>
                  <Suspense fallback={<div className="rich-doc-editor-shell loading" />}>
                    <RichTextDocEditor
                      key={activeDocument.id}
                      value={activeDocument.body}
                      onChange={(body) => onChange(activeDocument.id, { body })}
                    />
                  </Suspense>
                </label>
                <div className="design-doc-actions">
                  <button className="text-button danger" type="button" onClick={() => onDelete(activeDocument.id)}>
                    Delete doc
                  </button>
                </div>
              </>
            ) : (
              <div className="design-doc-empty-editor">
                <BookOpen size={34} />
                <strong>No document selected</strong>
                <span>Create a doc to start writing design context.</span>
                <button className="primary-action" type="button" onClick={onAdd}>
                  <FilePlus2 size={16} /> Add doc
                </button>
              </div>
            )}
          </section>
        </div>
      </section>
    </div>
  )
}

function RequirementPanel({
  design,
  onChange,
  compact = false,
  readOnly = false,
}: {
  design: DesignDocument
  onChange: (updater: (current: DesignDocument) => DesignDocument) => void
  compact?: boolean
  readOnly?: boolean
}) {
  const brief = createEmptyRequirementBrief(design.requirementBrief)
  return (
    <div className={`inspector ${compact ? 'compact-requirements' : ''}`}>
      {!compact ? <div className="inspector-heading">
        <Monitor size={18} />
        <div>
          <strong>Requirement Brief</strong>
          <span>Lightweight MVP capture</span>
        </div>
      </div> : null}
      <TextareaField
        label="Use case"
        value={brief.useCase}
        disabled={readOnly}
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, useCase: value } }))
        }
      />
      <TextareaField
        label="Functional requirements"
        value={brief.functionalRequirements}
        disabled={readOnly}
        onChange={(value) =>
          onChange((current) => ({
            ...current,
            requirementBrief: { ...current.requirementBrief, functionalRequirements: value },
          }))
        }
      />
      <NumberField
        label="Target RPS"
        value={brief.targetRps}
        disabled={readOnly}
        onChange={(targetRps) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, targetRps } }))
        }
      />
      <Field
        label="Availability"
        value={brief.availabilityRequirement}
        disabled={readOnly}
        onChange={(availabilityRequirement) =>
          onChange((current) => ({
            ...current,
            requirementBrief: { ...current.requirementBrief, availabilityRequirement },
          }))
        }
      />
      <Field
        label="SLA"
        value={brief.sla}
        disabled={readOnly}
        onChange={(sla) => onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, sla } }))}
      />
      <TextareaField
        label="Consistency requirements"
        value={brief.consistencyNotes}
        disabled={readOnly}
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, consistencyNotes: value } }))
        }
      />
      <TextareaField
        label="Non-functional requirements"
        value={brief.nonFunctionalRequirements}
        disabled={readOnly}
        onChange={(value) =>
          onChange((current) => ({
            ...current,
            requirementBrief: { ...current.requirementBrief, nonFunctionalRequirements: value },
          }))
        }
      />
      <TextareaField
        label="Traffic notes"
        value={brief.trafficNotes}
        disabled={readOnly}
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, trafficNotes: value } }))
        }
      />
      <TextareaField
        label="Primary actors"
        value={brief.primaryActors}
        disabled={readOnly}
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, primaryActors: value } }))
        }
      />
      <TextareaField
        label="Problem statement"
        value={brief.problemStatement}
        disabled={readOnly}
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, problemStatement: value } }))
        }
      />
      <TextareaField
        label="Open questions"
        value={brief.openQuestions}
        disabled={readOnly}
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, openQuestions: value } }))
        }
      />
    </div>
  )
}

function NewDesignBriefModal({
  title,
  brief,
  isCreating,
  onTitleChange,
  onBriefChange,
  onCancel,
  onCreate,
}: {
  title: string
  brief: RequirementBrief
  isCreating: boolean
  onTitleChange: (value: string) => void
  onBriefChange: (brief: RequirementBrief) => void
  onCancel: () => void
  onCreate: () => void
}) {
  function update(patch: Partial<RequirementBrief>) {
    onBriefChange(createEmptyRequirementBrief({ ...brief, ...patch }))
  }

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="design-brief-modal" role="dialog" aria-modal="true" aria-labelledby="new-design-title">
        <div className="modal-heading">
          <div>
            <p className="eyebrow">New design</p>
            <h2 id="new-design-title">Capture project context first</h2>
            <span>These requirements become the base for structured analysis.</span>
          </div>
        </div>

        <div className="brief-form-grid">
          <Field label="Design name" value={title} onChange={onTitleChange} />
          <NumberField label="Target RPS" value={brief.targetRps} onChange={(targetRps) => update({ targetRps })} />
          <TextareaField label="Use case" value={brief.useCase} onChange={(useCase) => update({ useCase })} />
          <TextareaField
            label="Functional requirements"
            value={brief.functionalRequirements}
            onChange={(functionalRequirements) => update({ functionalRequirements })}
          />
          <TextareaField
            label="Consistency requirements"
            value={brief.consistencyNotes}
            onChange={(consistencyNotes) => update({ consistencyNotes })}
          />
          <Field
            label="Availability requirements"
            value={brief.availabilityRequirement}
            onChange={(availabilityRequirement) => update({ availabilityRequirement })}
          />
          <Field label="SLA" value={brief.sla} onChange={(sla) => update({ sla })} />
          <TextareaField
            label="Non-functional requirements"
            value={brief.nonFunctionalRequirements}
            onChange={(nonFunctionalRequirements) => update({ nonFunctionalRequirements })}
          />
          <TextareaField label="Open questions" value={brief.openQuestions} onChange={(openQuestions) => update({ openQuestions })} />
        </div>

        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onCancel} disabled={isCreating}>
            Cancel
          </button>
          <button className="primary-action" type="button" onClick={onCreate} disabled={isCreating}>
            {isCreating ? 'Creating...' : 'Create design'}
          </button>
        </div>
      </section>
    </div>
  )
}

function DesignAnalysisBase({ design, docs }: { design: DesignDocument; docs: BackendDesignDoc[] }) {
  const brief = createEmptyRequirementBrief(design.requirementBrief)
  const checks = [
    {
      label: 'Use case captured',
      ok: Boolean(brief.useCase.trim() || brief.problemStatement.trim()),
    },
    {
      label: 'Traffic estimate present',
      ok: Boolean(brief.targetRps || brief.trafficNotes.trim()),
    },
    {
      label: 'Consistency target present',
      ok: Boolean(brief.consistencyNotes.trim()),
    },
    {
      label: 'Availability/SLA present',
      ok: Boolean(brief.availabilityRequirement.trim() || brief.sla.trim()),
    },
    {
      label: 'Architecture has components',
      ok: design.components.length > 0,
    },
    {
      label: 'Architecture has data flow',
      ok: design.connectors.length > 0,
    },
    {
      label: 'Traversal path documented',
      ok: Boolean((design.journeys ?? []).some((journey) => journey.steps.length > 0)),
    },
    {
      label: 'Design notes attached',
      ok: Boolean(docs.some((document) => stripHtml(document.body).trim())),
    },
  ]
  const score = checks.filter((check) => check.ok).length

  return (
    <div className="analysis-base-panel">
      <div className="inspector-heading">
        <Braces size={18} />
        <div>
          <strong>Analysis base</strong>
          <span>{score}/{checks.length} signals ready</span>
        </div>
      </div>
      <div className="analysis-score">
        <strong>{Math.round((score / checks.length) * 100)}%</strong>
        <span>Analysis readiness</span>
      </div>
      <div className="analysis-checklist">
        {checks.map((check) => (
          <div className={check.ok ? 'ready' : 'missing'} key={check.label}>
            <span>{check.ok ? 'Ready' : 'Missing'}</span>
            <strong>{check.label}</strong>
          </div>
        ))}
      </div>
    </div>
  )
}
