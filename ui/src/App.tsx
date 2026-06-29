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
  ArrowDown,
  ArrowUp,
  BookOpen,
  Boxes,
  Braces,
  Bell,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronsLeftRight,
  Copy,
  FilePlus2,
  Download,
  Eye,
  FolderPlus,
  Image as ImageIcon,
  MessageSquare,
  MessageSquarePlus,
  Monitor,
  Pencil,
  Plus,
  RotateCcw,
  Route,
  Save,
  Search,
  Send,
  Sparkles,
  SkipBack,
  SkipForward,
  Trash2,
} from 'lucide-react'
import { componentCatalog, getCatalogItem } from './catalog'
import {
  defaultWorkspace,
  parseRoute,
  routePath,
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
import { journeyStepKindLabel, journeyStepSummary } from './app/journeyModel'
import { formatCompactDateTime } from './app/format'
import { AppNavbar } from './components/AppNavbar'
import { CatalogGlyph, isCatalogComponentType } from './components/CatalogGlyph'
import type { DeleteTarget } from './components/DesignLifecycleModals'
import { Field, NumberField, SelectField, TextareaField } from './components/FormFields'
import { StatelessModeBanner } from './components/StatelessModeBanner'
import {
  analyzeDesign,
  createCatalogAsset,
  createDesign,
  createDesignComment,
  createDesignDoc,
  createDesignVersion,
  createAdminUser,
  createFirstAdmin,
  createPasswordResetLink,
  createWorkspace,
  deleteAdminUser,
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
  fetchAdminStorageStatus,
  fetchMCPConfig,
  fetchTelemetryIntegrationConfig,
  fetchCatalogAssets,
  fetchSetupStatus,
  fetchSignInConfig,
  fetchUsers,
  fetchWorkspaceDesign,
  fetchWorkspaceDesigns,
  fetchWorkspaces,
  login,
  logout,
  markNotificationRead,
  migrateStorageUsers,
  requestDesignReview,
  resetPassword,
  saveDesignDocument as saveDesignDocumentToBackend,
  setInitialAdminPassword,
  testStorageDatabase,
  updateDesignReview,
  updateDesignVersionStatus,
  updateDesignDoc,
  updateDesignMetadata,
  updateAdminUser,
  updateAIProviderConfig,
  updateCatalogAsset,
  updateMCPConfig,
  updateTelemetryIntegrationConfig,
  updateSignInConfig,
  type BackendAIProviderConfig,
  type BackendCatalogAsset,
  type BackendDesignComment,
  type BackendDesignDoc,
  type BackendDesignReviewRequest,
  type BackendMCPConfig,
  type BackendTelemetryIntegrationConfig,
  type BackendNotification,
  type BackendPageInfo,
  type BackendProfile,
  type BackendSignInConfig,
  type BackendStorageStatus,
  type BackendDesignVersion,
  type BackendDesignVersionStatus,
  type BackendUser,
  type DesignAnalysisReport,
} from './backendApi'
import { useBackendDesignSync, type BackendWorkspace } from './backendSync'
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
const FirstAdminOnboarding = lazy(() =>
  import('./components/AuthScreens').then((module) => ({ default: module.FirstAdminOnboarding })),
)
const LoginScreen = lazy(() =>
  import('./components/AuthScreens').then((module) => ({ default: module.LoginScreen })),
)
const ResetPasswordScreen = lazy(() =>
  import('./components/AuthScreens').then((module) => ({ default: module.ResetPasswordScreen })),
)
const CanvasErrorBoundary = lazy(() =>
  import('./components/CanvasErrorBoundary').then((module) => ({ default: module.CanvasErrorBoundary })),
)
const ReviewWorkspace = lazy(() =>
  import('./components/CollaborationPanels').then((module) => ({ default: module.ReviewWorkspace })),
)
const RequestReviewModal = lazy(() =>
  import('./components/CollaborationPanels').then((module) => ({ default: module.RequestReviewModal })),
)
const NotificationsModal = lazy(() =>
  import('./components/CollaborationPanels').then((module) => ({ default: module.NotificationsModal })),
)
const CopilotDraftModal = lazy(() =>
  import('./components/AIImportModals').then((module) => ({ default: module.CopilotDraftModal })),
)
const VisionImportModal = lazy(() =>
  import('./components/AIImportModals').then((module) => ({ default: module.VisionImportModal })),
)
const CloneDesignModal = lazy(() =>
  import('./components/DesignLifecycleModals').then((module) => ({ default: module.CloneDesignModal })),
)
const ConfirmDeleteModal = lazy(() =>
  import('./components/DesignLifecycleModals').then((module) => ({ default: module.ConfirmDeleteModal })),
)
const HomeScreen = lazy(() =>
  import('./components/HomeScreen').then((module) => ({ default: module.HomeScreen })),
)
const AdminConsole = lazy(() =>
  import('./components/AdminConsole').then((module) => ({ default: module.AdminConsole })),
)
const ComponentInspector = lazy(() =>
  import('./components/InspectorPanels').then((module) => ({ default: module.ComponentInspector })),
)
const ConnectorInspector = lazy(() =>
  import('./components/InspectorPanels').then((module) => ({ default: module.ConnectorInspector })),
)
const StructuredView = lazy(() =>
  import('./components/InspectorPanels').then((module) => ({ default: module.StructuredView })),
)
const ReactFlowCanvasProvider = lazy(() =>
  import('./canvas/ReactFlowCanvasProvider').then((module) => ({ default: module.ReactFlowCanvasProvider })),
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

function RouteLoading({ label = 'Loading workspace' }: { label?: string }) {
  return (
    <section className="route-loading">
      <div className="route-loading-mark">S</div>
      <span>{label}</span>
    </section>
  )
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

const HOME_PAGE_LIMIT = 24

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
  const [resetAction, setResetAction] = useState<'idle' | 'saving' | 'error' | 'complete'>('idle')
  const [resetError, setResetError] = useState<string | null>(null)
  const [adminError, setAdminError] = useState<string | null>(null)
  const [storageStatus, setStorageStatus] = useState<BackendStorageStatus | null>(null)
  const [signInConfig, setSignInConfig] = useState<BackendSignInConfig | null>(null)
  const [mcpConfig, setMCPConfig] = useState<BackendMCPConfig | null>(null)
  const [telemetryConfig, setTelemetryConfig] = useState<BackendTelemetryIntegrationConfig | null>(null)
  const [catalogAssets, setCatalogAssets] = useState<BackendCatalogAsset[]>([])
  const [catalogRailMode, setCatalogRailMode] = useState<'blocks' | 'catalog'>('blocks')
  const [catalogSearch, setCatalogSearch] = useState('')
  const [catalogTypeFilter, setCatalogTypeFilter] = useState('all')
  const [notifications, setNotifications] = useState<BackendNotification[]>([])
  const [notificationsOpen, setNotificationsOpen] = useState(false)
  const [homeWorkspaces, setHomeWorkspaces] = useState<BackendWorkspace[]>([defaultWorkspace])
  const [homeDesigns, setHomeDesigns] = useState<BackendDesign[]>([])
  const [homeWorkspaceSearch, setHomeWorkspaceSearch] = useState('')
  const [homeDesignSearch, setHomeDesignSearch] = useState('')
  const [homeWorkspacePage, setHomeWorkspacePage] = useState<BackendPageInfo | null>(null)
  const [homeDesignPage, setHomeDesignPage] = useState<BackendPageInfo | null>(null)
  const [homeLoadingMore, setHomeLoadingMore] = useState<'workspaces' | 'designs' | null>(null)
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
      activeStepKind: activeStep?.kind ?? null,
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
          fetchUsers({ limit: 100 }),
          fetchNotifications(),
          fetchCatalogAssets('', { limit: 100 }),
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
        const response = await fetchDesignVersions(activeWorkspaceId, selectedDesignId, { limit: 100 })
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
    enabled: view === 'canvas' && Boolean(selectedDesignId),
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
    const timer = window.setTimeout(() => {
      void refreshHome(activeWorkspaceId)
    }, 200)
    return () => window.clearTimeout(timer)
  }, [activeWorkspaceId, homeDesignSearch, homeWorkspaceSearch])

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
      const updated = updater(current)
      if (updated === current) return current
      const nextDesign = touchDesign(updated)
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

  function selectCanvasComponent(componentId: string) {
    if (!componentId) {
      handleCanvasSelectionChange([], [])
      return
    }
    handleCanvasSelectionChange([componentId], [])
    setContextPanel('inspector')
    setRightRailOpen(true)
  }

  function selectCanvasConnector(connectorId: string) {
    if (!connectorId) {
      handleCanvasSelectionChange([], [])
      return
    }
    handleCanvasSelectionChange([], [connectorId])
    setContextPanel('inspector')
    setRightRailOpen(true)
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
    } else if (nextRoute.screen === 'reset-password') {
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
      const response = await fetchDesignVersions(sourceDesign.workspaceId, sourceDesign.id, { limit: 100 })
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
      const [profile, workspaces] = await Promise.all([
        fetchProfile(),
        fetchWorkspaces({ query: homeWorkspaceSearch, limit: HOME_PAGE_LIMIT }),
      ])
      const nextWorkspaces = workspaces.workspaces.length ? workspaces.workspaces : [defaultWorkspace]
      setHomeProfile(profile)
      if (profile.storage) setStorageStatus(profile.storage)
      setHomeWorkspaces(nextWorkspaces)
      setHomeWorkspacePage(workspaces.page ?? null)
      const keepSearchedWorkspace = homeWorkspaceSearch.trim() !== ''
      const selectedWorkspace = nextWorkspaces.some((workspace) => workspace.id === workspaceId) || keepSearchedWorkspace
        ? workspaceId
        : nextWorkspaces[0]?.id || defaultWorkspace.id
      if (selectedWorkspace !== workspaceId) {
        applyRoute({ screen: 'workspace', workspaceId: selectedWorkspace }, { replace: true })
        return
      }
      try {
        const designs = await fetchWorkspaceDesigns(selectedWorkspace, { query: homeDesignSearch, limit: HOME_PAGE_LIMIT })
        setHomeDesigns(designs.designs)
        setHomeDesignPage(designs.page ?? null)
      } catch (error) {
        setHomeDesigns([])
        setHomeDesignPage(null)
        setHomeError(error instanceof Error ? error.message : 'Could not load workspace designs')
      }
    } catch (error) {
      setHomeProfile(null)
      setHomeWorkspaces((current) => (current.length ? current : [defaultWorkspace]))
      setHomeDesigns([])
      setHomeWorkspacePage(null)
      setHomeDesignPage(null)
      setHomeError(error instanceof Error ? error.message : 'Backend unavailable')
    }
  }

  async function loadMoreHomeWorkspaces() {
    if (!homeWorkspacePage?.hasMore || !homeWorkspacePage.nextCursor || homeLoadingMore) return
    try {
      setHomeLoadingMore('workspaces')
      const response = await fetchWorkspaces({
        query: homeWorkspaceSearch,
        cursor: homeWorkspacePage.nextCursor,
        limit: HOME_PAGE_LIMIT,
      })
      setHomeWorkspaces((current) => {
        const seen = new Set(current.map((workspace) => workspace.id))
        return [...current, ...response.workspaces.filter((workspace) => !seen.has(workspace.id))]
      })
      setHomeWorkspacePage(response.page ?? null)
    } catch (error) {
      setHomeError(error instanceof Error ? error.message : 'Could not load more workspaces')
    } finally {
      setHomeLoadingMore(null)
    }
  }

  async function loadMoreHomeDesigns() {
    if (!homeDesignPage?.hasMore || !homeDesignPage.nextCursor || homeLoadingMore) return
    try {
      setHomeLoadingMore('designs')
      const response = await fetchWorkspaceDesigns(activeWorkspaceId, {
        query: homeDesignSearch,
        cursor: homeDesignPage.nextCursor,
        limit: HOME_PAGE_LIMIT,
      })
      setHomeDesigns((current) => {
        const seen = new Set(current.map((designItem) => designItem.id))
        return [...current, ...response.designs.filter((designItem) => !seen.has(designItem.id))]
      })
      setHomeDesignPage(response.page ?? null)
    } catch (error) {
      setHomeError(error instanceof Error ? error.message : 'Could not load more designs')
    } finally {
      setHomeLoadingMore(null)
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

  async function completePasswordReset(input: { token: string; password: string }) {
    setResetAction('saving')
    setResetError(null)
    try {
      await resetPassword(input)
      setResetAction('complete')
    } catch (error) {
      setResetAction('error')
      setResetError(error instanceof Error ? error.message : 'Could not reset password')
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
    const [userResponse, signInResponse, aiProviderResponse, mcpResponse, telemetryResponse, catalogResponse, storageResponse] = await Promise.allSettled([
      fetchUsers({ limit: 100 }),
      fetchSignInConfig(),
      fetchAIProviderConfig(),
      fetchMCPConfig(),
      fetchTelemetryIntegrationConfig(),
      fetchCatalogAssets('', { limit: 100 }),
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
    if (telemetryResponse.status === 'fulfilled') setTelemetryConfig(telemetryResponse.value.telemetry)
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

  async function addAdminUser(input: { displayName: string; email: string; role: string }) {
    setAdminError(null)
    try {
      const response = await createAdminUser(input)
      setUsers((current) => [...current, response.user])
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not add user')
    }
  }

  async function saveAdminUser(user: BackendUser) {
    setAdminError(null)
    try {
      const response = await updateAdminUser(user.id, {
        displayName: user.displayName,
        email: user.email,
        role: user.role,
        status: user.status,
      })
      setUsers((current) => current.map((item) => (item.id === response.user.id ? response.user : item)))
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not save user')
    }
  }

  async function generatePasswordResetLink(userId: string) {
    setAdminError(null)
    try {
      return await createPasswordResetLink(userId)
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not generate reset link')
      throw error
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

  async function saveTelemetryIntegration(nextConfig: Partial<BackendTelemetryIntegrationConfig> & { secret?: string }) {
    setAdminError(null)
    try {
      const response = await updateTelemetryIntegrationConfig(nextConfig)
      setTelemetryConfig(response.telemetry)
    } catch (error) {
      setAdminError(error instanceof Error ? error.message : 'Could not save telemetry integration settings')
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
        fetchDesignComments(activeWorkspaceId, selectedDesignId, { limit: 100 }),
        fetchDesignReviews(activeWorkspaceId, selectedDesignId, { limit: 100 }),
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
      const versionResponse = await fetchDesignVersions(activeWorkspaceId, selectedDesignId, { limit: 100 })
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
        void fetchDesignVersions(activeWorkspaceId, selectedDesignId, { limit: 100 }).then((versionResponse) => setDesignVersions(versionResponse.versions))
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
    updateDesign((current) => {
      const component = current.components.find((item) => item.id === componentId)
      if (!component) return current
      const currentPosition = component?.metadata.position
      if (
        currentPosition?.x === position.x &&
        currentPosition?.y === position.y &&
        component.metadata.parentFrameId === parentFrameId
      ) {
        return current
      }
      return {
        ...current,
        components: current.components.map((item) =>
          item.id === componentId
            ? { ...item, metadata: { ...item.metadata, position, parentFrameId } }
            : item,
        ),
      }
    })
  }

  function resizeReactFlowComponent(componentId: string, size: { width: number; height: number }) {
    updateDesign((current) => {
      const component = current.components.find((item) => item.id === componentId)
      if (!component) return current
      const currentSize = component?.metadata.size
      if (currentSize?.width === size.width && currentSize?.height === size.height) {
        return current
      }
      return {
        ...current,
        components: current.components.map((item) =>
          item.id === componentId
            ? { ...item, metadata: { ...item.metadata, size } }
            : item,
        ),
      }
    })
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
        <Suspense fallback={<RouteLoading label="Loading setup" />}>
          <FirstAdminOnboarding
            error={homeError}
            isSaving={setupAction === 'saving'}
            onCreate={(input) => void completeFirstAdmin(input)}
          />
        </Suspense>
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

  if (route.screen === 'reset-password') {
    const resetToken = new URLSearchParams(window.location.search).get('token') ?? ''
    return (
      <div className="screen-shell">
        <Suspense fallback={<RouteLoading label="Loading password reset" />}>
          <ResetPasswordScreen
            token={resetToken}
            error={resetError}
            isSaving={resetAction === 'saving'}
            isComplete={resetAction === 'complete'}
            onReset={(input) => void completePasswordReset(input)}
            onSignIn={() => {
              setResetAction('idle')
              setResetError(null)
              applyRoute({ screen: 'home' })
            }}
          />
        </Suspense>
      </div>
    )
  }

  if (passwordSetupRequired || !authenticated) {
    return (
      <div className="screen-shell">
        <Suspense fallback={<RouteLoading label="Loading sign in" />}>
          <LoginScreen
            requiresPasswordSetup={passwordSetupRequired}
            error={authError}
            isSaving={authAction === 'saving'}
            onLogin={(input) => void signIn(input)}
            onSetInitialPassword={(input) => void completePasswordSetup(input)}
          />
        </Suspense>
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
        <Suspense fallback={<RouteLoading label="Loading admin console" />}>
          <AdminConsole
            users={users}
            activeUserId={activeUserId}
            workspaces={homeWorkspaces}
            storageStatus={storageStatus}
            signInConfig={signInConfig}
            aiProviderConfig={aiConnection}
            mcpConfig={mcpConfig}
            telemetryConfig={telemetryConfig}
            catalogAssets={catalogAssets}
            error={adminError}
            onAddUser={addAdminUser}
            onSaveUser={saveAdminUser}
            onGeneratePasswordResetLink={generatePasswordResetLink}
            onDeleteUser={(userId) => void removeAdminUser(userId)}
            onSaveSignIn={(config) => void saveSignIn(config)}
            onSaveAIProvider={(config) => void saveAIProvider(config)}
            onSaveMCP={(config) => void saveMCP(config)}
            onSaveTelemetry={(config) => void saveTelemetryIntegration(config)}
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
        </Suspense>
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
        <Suspense fallback={<RouteLoading label="Loading home" />}>
          <HomeScreen
            workspaces={homeWorkspaces}
            designs={homeDesigns}
            activeWorkspaceId={activeWorkspaceId}
            newWorkspaceName={newWorkspaceName}
            workspaceSearch={homeWorkspaceSearch}
            designSearch={homeDesignSearch}
            workspacePage={homeWorkspacePage}
            designPage={homeDesignPage}
            loadingMore={homeLoadingMore}
            error={homeError}
            action={homeAction}
            onWorkspaceNameChange={setNewWorkspaceName}
            onWorkspaceSearchChange={setHomeWorkspaceSearch}
            onDesignSearchChange={setHomeDesignSearch}
            onSelectWorkspace={(workspaceId) => applyRoute({ screen: 'workspace', workspaceId })}
            onCreateWorkspace={handleCreateWorkspace}
            onDeleteWorkspace={requestDeleteWorkspace}
            onOpenDesign={selectBackendDesign}
            onCloneDesign={requestCloneDesign}
            onDeleteDesign={requestDeleteDesign}
            onNewDesign={openNewDesignBrief}
            onLoadMoreWorkspaces={loadMoreHomeWorkspaces}
            onLoadMoreDesigns={loadMoreHomeDesigns}
            onRefresh={() => refreshHome(activeWorkspaceId)}
          />
        </Suspense>
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
          <Suspense fallback={null}>
            <ConfirmDeleteModal
              target={deleteTarget}
              isDeleting={homeAction === 'delete-workspace' || homeAction === 'delete-design' || homeAction === 'delete-version'}
              onCancel={() => setDeleteTarget(null)}
              onConfirm={() => void confirmDeleteTarget()}
            />
          </Suspense>
        ) : null}
        {cloneTarget ? (
          <Suspense fallback={null}>
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
          </Suspense>
        ) : null}
        {notificationsOpen ? (
          <Suspense fallback={null}>
            <NotificationsModal
              notifications={notifications}
              users={users}
              onMarkRead={(notificationId) => void markNotificationAsRead(notificationId)}
              onClose={() => setNotificationsOpen(false)}
            />
          </Suspense>
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
      <Suspense fallback={<RouteLoading label="Loading canvas" />}>
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
              onSelectComponent={selectCanvasComponent}
              onSelectConnector={selectCanvasConnector}
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
      </Suspense>
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
        <Suspense fallback={null}>
          <ConfirmDeleteModal
            target={deleteTarget}
            isDeleting={homeAction === 'delete-workspace' || homeAction === 'delete-design' || homeAction === 'delete-version'}
            onCancel={() => setDeleteTarget(null)}
            onConfirm={() => void confirmDeleteTarget()}
          />
        </Suspense>
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
        <Suspense fallback={null}>
          <CopilotDraftModal
            aiConnection={aiConnection?.enabled && aiConnection.apiKeySet ? aiConnection : null}
            onClose={() => setCopilotOpen(false)}
          />
        </Suspense>
      ) : null}
      {visionOpen ? (
        <Suspense fallback={null}>
          <VisionImportModal
            aiConnection={aiConnection?.enabled && aiConnection.apiKeySet ? aiConnection : null}
            onClose={() => setVisionOpen(false)}
          />
        </Suspense>
      ) : null}
      {requestReviewOpen ? (
        <Suspense fallback={null}>
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
        </Suspense>
      ) : null}
      {notificationsOpen ? (
        <Suspense fallback={null}>
          <NotificationsModal
            notifications={notifications}
            users={users}
            onMarkRead={(notificationId) => void markNotificationAsRead(notificationId)}
            onClose={() => setNotificationsOpen(false)}
          />
        </Suspense>
      ) : null}
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

  function updateStep(
    stepId: string,
    patch: Partial<Pick<DesignJourney['steps'][number], 'title' | 'description' | 'kind'>>,
  ) {
    if (!activeJourney) return
    onChangeJourney({
      ...activeJourney,
      steps: activeJourney.steps.map((step) => (step.id === stepId ? { ...step, ...patch } : step)),
    })
  }

  function addStep() {
    if (!activeJourney || readOnly) return
    const nextStep = {
      id: `step_${crypto.randomUUID()}`,
      kind: 'request' as const,
      title: 'New step',
      description: '',
    }
    const insertAt = Math.min(activeStepIndex + 1, activeJourney.steps.length)
    onChangeJourney({
      ...activeJourney,
      steps: [
        ...activeJourney.steps.slice(0, insertAt),
        nextStep,
        ...activeJourney.steps.slice(insertAt),
      ],
      updatedAt: new Date().toISOString(),
    })
    onSetStep(insertAt)
  }

  function deleteStep(stepId: string) {
    if (!activeJourney || readOnly) return
    const nextSteps = activeJourney.steps.filter((step) => step.id !== stepId)
    onChangeJourney({ ...activeJourney, steps: nextSteps, updatedAt: new Date().toISOString() })
    onSetStep(Math.max(0, Math.min(activeStepIndex, nextSteps.length - 1)))
  }

  function moveStep(stepId: string, direction: -1 | 1) {
    if (!activeJourney || readOnly) return
    const currentIndex = activeJourney.steps.findIndex((step) => step.id === stepId)
    const nextIndex = currentIndex + direction
    if (currentIndex < 0 || nextIndex < 0 || nextIndex >= activeJourney.steps.length) return
    const nextSteps = [...activeJourney.steps]
    const [step] = nextSteps.splice(currentIndex, 1)
    nextSteps.splice(nextIndex, 0, step)
    onChangeJourney({ ...activeJourney, steps: nextSteps, updatedAt: new Date().toISOString() })
    onSetStep(nextIndex)
  }

  const connectorById = useMemo(() => new Map(design.connectors.map((connector) => [connector.id, connector])), [design.connectors])
  const activeConnector = activeStep?.connectorId ? connectorById.get(activeStep.connectorId) : null

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
        <button className="command" type="button" onClick={addStep} disabled={readOnly || !activeJourney}>
          <Plus size={16} /> Add step
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
                  <div className="journey-step-kind-row">
                    {(['request', 'async', 'callback', 'batch', 'signal', 'decision'] as const).map((kind) => (
                      <button
                        className={activeStep.kind === kind ? 'active' : ''}
                        type="button"
                        key={kind}
                        disabled={readOnly}
                        onClick={() => updateStep(activeStep.id, { kind })}
                      >
                        {journeyStepKindLabel(kind)}
                      </button>
                    ))}
                  </div>
                  <div className={`journey-step-meaning kind-${activeStep.kind ?? 'request'}`}>
                    {journeyStepSummary(activeStep.kind, activeConnector?.type)}
                  </div>
                  <Field
                    label="Step title"
                    value={activeStep.title}
                    onChange={(title) => updateStep(activeStep.id, { title })}
                    disabled={readOnly}
                  />
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
                  <div className={index === activeStepIndex ? 'journey-step-card active' : 'journey-step-card'} key={step.id}>
                    <button className="journey-step-select" type="button" onClick={() => onSetStep(index)}>
                      <span>{index + 1}</span>
                      <div>
                        <strong>{step.title || 'Untitled step'}</strong>
                        <small>{journeyStepKindLabel(step.kind, connectorById.get(step.connectorId ?? '')?.type)}</small>
                      </div>
                    </button>
                    <div className="journey-step-actions">
                      <button type="button" title="Move up" disabled={readOnly || index === 0} onClick={() => moveStep(step.id, -1)}>
                        <ArrowUp size={14} />
                      </button>
                      <button
                        type="button"
                        title="Move down"
                        disabled={readOnly || index === activeJourney.steps.length - 1}
                        onClick={() => moveStep(step.id, 1)}
                      >
                        <ArrowDown size={14} />
                      </button>
                      <button type="button" title="Delete step" disabled={readOnly} onClick={() => deleteStep(step.id)}>
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </div>
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
