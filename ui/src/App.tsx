import {
  Component,
  lazy,
  Suspense,
  type ErrorInfo,
  type CSSProperties,
  type ReactNode,
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
  UserCheck,
  UserPlus,
} from 'lucide-react'
import { componentCatalog, getCatalogItem } from './catalog'
import {
  analyzeDesign,
  createCatalogAsset,
  createDesign,
  createDesignComment,
  createDesignDoc,
  createAdminUser,
  createFirstAdmin,
  createWorkspace,
  deleteAdminUser,
  deleteCatalogAsset,
  deleteDesign as deleteDesignFromBackend,
  deleteDesignDoc as deleteDesignDocFromBackend,
  deleteWorkspace as deleteWorkspaceFromBackend,
  fetchDesignComments,
  fetchDesignDocs,
  fetchDesignReviews,
  fetchNotifications,
  fetchProfile,
  fetchAIProviderConfig,
  fetchMCPConfig,
  fetchCatalogAssets,
  fetchDesignAccess,
  fetchSetupStatus,
  fetchSignInConfig,
  fetchUsers,
  fetchWorkspaceAccess,
  fetchWorkspaceDesign,
  fetchWorkspaceDesigns,
  fetchWorkspaces,
  grantDesignAccess,
  grantWorkspaceAccess,
  login,
  logout,
  markNotificationRead,
  requestDesignReview,
  revokeDesignAccess,
  revokeWorkspaceAccess,
  saveDesignDocument as saveDesignDocumentToBackend,
  setInitialAdminPassword,
  updateDesignReview,
  updateDesignDoc,
  updateDesignMetadata,
  updateAdminUser,
  updateAIProviderConfig,
  updateCatalogAsset,
  updateMCPConfig,
  updateSignInConfig,
  type BackendAIProviderConfig,
  type BackendCatalogAsset,
  type BackendDesignAccess,
  type BackendDesignComment,
  type BackendDesignDoc,
  type BackendDesignReviewRequest,
  type BackendMCPConfig,
  type BackendNotification,
  type BackendProfile,
  type BackendSignInConfig,
  type BackendUser,
  type BackendWorkspaceAccess,
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

type AppRoute =
  | { screen: 'home' }
  | { screen: 'admin'; section?: AdminSection }
  | { screen: 'workspace'; workspaceId: string }
  | { screen: 'design'; workspaceId: string; designId: string }

type AdminSection =
  | 'overview'
  | 'users'
  | 'access'
  | 'workspaces'
  | 'catalog'
  | 'identity'
  | 'ai'
  | 'integrations'
  | 'security'
  | 'settings'
type CanvasMode = 'design' | 'view' | 'comment' | 'journey'
type ContextPanel = 'inspector' | 'requirements' | 'journey' | 'review'

const adminSectionOrder: AdminSection[] = [
  'overview',
  'users',
  'access',
  'workspaces',
  'catalog',
  'identity',
  'ai',
  'integrations',
  'security',
  'settings',
]

const adminSectionCopy: Record<AdminSection, { label: string; description: string; group: string }> = {
  overview: {
    label: 'Dashboard',
    description: 'Deployment health, configuration state, and setup shortcuts.',
    group: 'Command',
  },
  users: {
    label: 'Users and roles',
    description: 'Manage people, passwords, roles, and account state.',
    group: 'Identity',
  },
  access: {
    label: 'Access center',
    description: 'Workspace and design ACL policy surface.',
    group: 'Identity',
  },
  workspaces: {
    label: 'Workspaces',
    description: 'Govern workspace ownership, deletion, and design creation.',
    group: 'Content',
  },
  catalog: {
    label: 'Enterprise catalog',
    description: 'Canonical services and infrastructure reused across designs.',
    group: 'Content',
  },
  identity: {
    label: 'Sign-in and SSO',
    description: 'Local sign-in, Okta OIDC, claims, and provisioning.',
    group: 'Security',
  },
  ai: {
    label: 'AI suite',
    description: 'Central provider for analysis, vision, and future copilots.',
    group: 'Intelligence',
  },
  integrations: {
    label: 'Integrations',
    description: 'MCP readiness and future enterprise integration endpoints.',
    group: 'Platform',
  },
  security: {
    label: 'Security and audit',
    description: 'Policy posture, audit readiness, and security controls.',
    group: 'Security',
  },
  settings: {
    label: 'System settings',
    description: 'Deployment defaults and platform-level configuration.',
    group: 'Platform',
  },
}

const defaultWorkspace: BackendWorkspace = {
  id: 'guest-workspace',
  name: 'Guest Workspace',
}

const RichTextDocEditor = lazy(() =>
  import('./components/RichTextDocEditor').then((module) => ({ default: module.RichTextDocEditor })),
)

type AIConnectionMetadata = BackendAIProviderConfig

type DeleteTarget =
  | { kind: 'workspace'; workspace: BackendWorkspace }
  | { kind: 'design'; design: BackendDesign }

type PendingConnector = {
  fromComponentId: string
  toComponentId: string
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

function parseRoute(pathname: string): AppRoute {
  const segments = pathname.split('/').filter(Boolean).map(decodeURIComponent)
  if (segments[0] === 'workspaces' && segments[1] && segments[2] === 'designs' && segments[3]) {
    return { screen: 'design', workspaceId: segments[1], designId: segments[3] }
  }
  if (segments[0] === 'workspaces' && segments[1]) {
    return { screen: 'workspace', workspaceId: segments[1] }
  }
  if (segments[0] === 'admin') {
    return { screen: 'admin', section: parseAdminSection(segments[1]) }
  }
  return { screen: 'home' }
}

function routePath(route: AppRoute) {
  if (route.screen === 'admin') return route.section && route.section !== 'overview' ? `/admin/${route.section}` : '/admin'
  if (route.screen === 'workspace') return `/workspaces/${encodeURIComponent(route.workspaceId)}`
  if (route.screen === 'design') {
    return `/workspaces/${encodeURIComponent(route.workspaceId)}/designs/${encodeURIComponent(route.designId)}`
  }
  return '/'
}

function parseAdminSection(value: string | undefined): AdminSection {
  if (
    value === 'users' ||
    value === 'access' ||
    value === 'workspaces' ||
    value === 'catalog' ||
    value === 'identity' ||
    value === 'ai' ||
    value === 'integrations' ||
    value === 'security' ||
    value === 'settings'
  ) {
    return value
  }
  return 'overview'
}

function isJourneyComponent(component: DesignComponent) {
  return component.type !== 'frame.cloud' && component.type !== 'note.sticky'
}

function buildPrimaryJourney(design: DesignDocument): DesignJourney | null {
  const components = design.components.filter(isJourneyComponent)
  if (!components.length) return null

  const componentById = new Map(design.components.map((component) => [component.id, component]))
  const incoming = new Set(design.connectors.map((connector) => connector.toComponentId))
  const preferredStartTypes: ComponentType[] = ['client.web', 'external.api', 'edge.api_gateway']
  const start =
    components.find((component) => !incoming.has(component.id) && preferredStartTypes.includes(component.type)) ??
    components.find((component) => !incoming.has(component.id)) ??
    components[0]
  const now = new Date().toISOString()
  const steps: DesignJourney['steps'] = []
  const visitedComponents = new Set<string>()
  const visitedConnectors = new Set<string>()

  function pushComponentStep(component: DesignComponent, prefix = 'Start') {
    if (visitedComponents.has(component.id) || steps.length >= 18) return
    visitedComponents.add(component.id)
    steps.push({
      id: `step_${crypto.randomUUID()}`,
      componentId: component.id,
      title: component.name || getCatalogItem(component.type).label,
      description: `${prefix} at ${component.name || getCatalogItem(component.type).label}.`,
    })
  }

  function walk(component: DesignComponent) {
    if (steps.length >= 18) return
    const outgoing = design.connectors.filter(
      (connector) => connector.fromComponentId === component.id && !visitedConnectors.has(connector.id),
    )
    for (const connector of outgoing) {
      if (steps.length >= 18) return
      visitedConnectors.add(connector.id)
      const target = componentById.get(connector.toComponentId)
      const sourceName = component.name || getCatalogItem(component.type).label
      const targetName = target ? target.name || getCatalogItem(target.type).label : 'unknown target'
      steps.push({
        id: `step_${crypto.randomUUID()}`,
        componentId: target?.id,
        connectorId: connector.id,
        title: `${sourceName} to ${targetName}`,
        description: `${sourceName} sends ${connector.type.replaceAll('_', ' ')} traffic to ${targetName}${
          connector.protocol ? ` over ${connector.protocol}` : ''
        }.`,
      })
      if (target && isJourneyComponent(target) && !visitedComponents.has(target.id)) {
        visitedComponents.add(target.id)
        walk(target)
      }
    }
  }

  pushComponentStep(start)
  walk(start)

  if (steps.length === 1 && design.connectors.length) {
    const firstConnector = design.connectors[0]
    const source = componentById.get(firstConnector.fromComponentId)
    const target = componentById.get(firstConnector.toComponentId)
    steps.push({
      id: `step_${crypto.randomUUID()}`,
      componentId: target?.id,
      connectorId: firstConnector.id,
      title: `${source?.name ?? 'Source'} to ${target?.name ?? 'Target'}`,
      description: `${source?.name ?? 'Source'} sends ${firstConnector.type.replaceAll('_', ' ')} traffic to ${
        target?.name ?? 'target'
      }.`,
    })
  }

  return {
    id: `journey_${crypto.randomUUID()}`,
    title: design.requirementBrief.useCase.trim() ? `${design.requirementBrief.useCase.trim()} journey` : 'Primary request journey',
    description: 'Ordered walkthrough generated from the current component graph. Refine the step text as the design matures.',
    entryComponentId: start.id,
    steps,
    createdAt: now,
    updatedAt: now,
  }
}

function stripHtml(value: string) {
  return value
    .replace(/<style[\s\S]*?<\/style>/gi, '')
    .replace(/<script[\s\S]*?<\/script>/gi, '')
    .replace(/<[^>]+>/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function isBackendOfflineMessage(message: string) {
  const normalized = message.toLowerCase()
  return normalized.includes('backend unavailable') || normalized.includes('load failed') || normalized.includes('failed to fetch')
}

function normalizedCatalogLabel(value: string) {
  return value.trim().toLowerCase().replace(/\s+/g, ' ')
}

function isPotentialCatalogMatch(left: string, right: string) {
  const normalizedLeft = normalizedCatalogLabel(left)
  const normalizedRight = normalizedCatalogLabel(right)
  return Boolean(
    normalizedLeft &&
      normalizedRight &&
      (normalizedLeft === normalizedRight || normalizedLeft.includes(normalizedRight) || normalizedRight.includes(normalizedLeft)),
  )
}

function CatalogGlyph({ type }: { type: ComponentType }) {
  if (type === 'client.web') return <Monitor size={17} />
  if (type === 'edge.api_gateway') return <Route size={17} />
  if (type === 'data.sql_database' || type === 'data.redis' || type === 'data.object_store') return <Database size={17} />
  if (type === 'messaging.queue') return <ChevronsLeftRight size={17} />
  if (type === 'ai.llm') return <Sparkles size={17} />
  if (type === 'external.api') return <Globe2 size={17} />
  if (type === 'observability.telemetry') return <Activity size={17} />
  if (type === 'security.control') return <ShieldCheck size={17} />
  if (type === 'design.link') return <FilePlus2 size={17} />
  if (type === 'frame.cloud') return <Cloud size={17} />
  if (type === 'note.sticky') return <MessageSquare size={17} />
  return <Server size={17} />
}

function isCatalogComponentType(type: string): type is ComponentType {
  return componentCatalog.some((item) => item.type === type)
}

function catalogAssetUsageLabel(count: number) {
  return `${count} design${count === 1 ? '' : 's'}`
}

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
  const [homeAction, setHomeAction] = useState<'workspace' | 'design' | 'delete-workspace' | 'delete-design' | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null)
  const [deletedDesignIds, setDeletedDesignIds] = useState<Set<string>>(() => new Set())
  const [newDesignBriefOpen, setNewDesignBriefOpen] = useState(false)
  const [newDesignTitle, setNewDesignTitle] = useState('Untitled system design')
  const [newDesignBrief, setNewDesignBrief] = useState<RequirementBrief>(() => createEmptyRequirementBrief())
  const [saveState, setSaveState] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle')
  const [design, setDesign] = useState<DesignDocument>(() => touchDesign(loadDesignDocument() ?? createEmptyDesign()))
  const [designName, setDesignName] = useState(() => loadDesignDocument()?.title ?? 'Untitled system design')
  const [selectedComponentId, setSelectedComponentId] = useState<string | null>(null)
  const [selectedConnectorId, setSelectedConnectorId] = useState<string | null>(null)
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
  const selectedComponent = useMemo(
    () => design.components.find((component) => component.id === selectedComponentId) ?? null,
    [design.components, selectedComponentId],
  )
  const selectedConnector = useMemo(
    () => design.connectors.find((connector) => connector.id === selectedConnectorId) ?? null,
    [design.connectors, selectedConnectorId],
  )
  const activeJourney = useMemo(
    () => design.journeys?.find((journey) => journey.id === activeJourneyId) ?? null,
    [activeJourneyId, design.journeys],
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
  const isCanvasReadOnly = canvasMode !== 'design'

  useEffect(() => {
    designRef.current = design
  }, [design])

  useEffect(() => {
    designNameRef.current = designName
  }, [designName])

  useEffect(() => {
    if (activeJourneyId && design.journeys.some((journey) => journey.id === activeJourneyId)) return
    setActiveJourneyId(design.journeys[0]?.id ?? null)
    setActiveJourneyStepIndex(0)
  }, [activeJourneyId, design.journeys])

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
      if (event.key !== 'Delete' && event.key !== 'Backspace') return
      if (isCanvasReadOnly) return
      if (selectedComponentId) {
        event.preventDefault()
        deleteComponents([selectedComponentId])
      } else if (selectedConnectorId) {
        event.preventDefault()
        deleteConnectors([selectedConnectorId])
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isCanvasReadOnly, selectedComponentId, selectedConnectorId])

  function updateDesign(updater: (current: DesignDocument) => DesignDocument) {
    setDesign((current) => {
      const nextDesign = touchDesign(updater(current))
      designRef.current = nextDesign
      return nextDesign
    })
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

  function loadDesignIntoWorkspace(nextDesign: DesignDocument, metadataName?: string) {
    const normalizedDesign = normalizeDesignDocument(nextDesign)
    setDesign(normalizedDesign)
    setDesignName(metadataName || normalizedDesign.title || 'Untitled system design')
    setSelectedComponentId(null)
    setSelectedConnectorId(null)
    setActiveJourneyId(normalizedDesign.journeys[0]?.id ?? null)
    setActiveJourneyStepIndex(0)
    saveDesignDocument(normalizedDesign)
  }

  function persistDesignToRealtime() {
    const currentDesign = designRef.current
    saveDesignDocument(currentDesign)
    return Boolean(selectedDesignId && backendSync.saveDesign(currentDesign))
  }

  async function persistDesignToBackend() {
    const currentDesign = designRef.current
    saveDesignDocument(currentDesign)
    if (!selectedDesignId) return false

    setSaveState('saving')
    try {
      const response = await saveDesignDocumentToBackend(activeWorkspaceId, selectedDesignId, {
        document: currentDesign,
      })
      setHomeDesigns((current) =>
        current.map((backendDesign) => (backendDesign.id === response.design.id ? response.design : backendDesign)),
      )
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

  async function refreshHome(workspaceId: string) {
    try {
      setHomeError(null)
      const [profile, workspaces] = await Promise.all([fetchProfile(), fetchWorkspaces()])
      const nextWorkspaces = workspaces.workspaces.length ? workspaces.workspaces : [defaultWorkspace]
      setHomeProfile(profile)
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
    const [userResponse, signInResponse, aiProviderResponse, mcpResponse, catalogResponse] = await Promise.allSettled([
      fetchUsers(),
      fetchSignInConfig(),
      fetchAIProviderConfig(),
      fetchMCPConfig(),
      fetchCatalogAssets(),
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

  async function submitReviewRequest(reviewerId: string, message: string) {
    if (!selectedDesignId) return
    setCollaborationState('saving')
    setCollaborationError(null)
    setCollaborationOffline(false)
    try {
      const response = await requestDesignReview(activeWorkspaceId, selectedDesignId, { reviewerId, message })
      setReviews((current) => [response.review, ...current])
      setRequestReviewOpen(false)
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
    setSelectedConnectorId(null)
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
    setSelectedConnectorId(null)
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
      setSelectedConnectorId(null)
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
    setSelectedComponentId(null)
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
    void persistDesignToBackend()
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
        <AdminConsole
          users={users}
          activeUserId={activeUserId}
          workspaces={homeWorkspaces}
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
            isDeleting={homeAction === 'delete-workspace' || homeAction === 'delete-design'}
            onCancel={() => setDeleteTarget(null)}
            onConfirm={() => void confirmDeleteTarget()}
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
            <div className="section-title">Draft</div>
            <button className="command" onClick={saveDraft}>
              <Save size={16} /> Save design
            </button>
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
            <button className="command compact" onClick={saveDraft} title="Save design">
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
          <button
            className={`context-rail-toggle ${rightRailOpen ? 'open' : 'closed'}`}
            type="button"
            onClick={() => setRightRailOpen((value) => !value)}
            title={rightRailOpen ? 'Collapse context panel' : 'Open context panel'}
          >
            {rightRailOpen ? <ChevronRight size={17} /> : <ChevronLeft size={17} />}
          </button>
          <StratumCanvasBoundary
            onResetCanvas={() => resetDraft()}
          >
            <ReactFlowCanvasProvider
              design={design}
              selectedComponentId={selectedComponentId}
              selectedConnectorId={selectedConnectorId}
              readOnly={isCanvasReadOnly}
              traversalFocus={traversalFocus}
              onSelectComponent={(componentId) => {
                setSelectedComponentId(componentId || null)
                if (componentId) setSelectedConnectorId(null)
                if (componentId) {
                  setContextPanel('inspector')
                  setRightRailOpen(true)
                }
              }}
              onSelectConnector={(connectorId) => {
                setSelectedConnectorId(connectorId || null)
                if (connectorId) setSelectedComponentId(null)
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
          </StratumCanvasBoundary>
        </div>
      </section>

        <aside className="right-rail">
          <div className="context-drawer-header">
            <div>
              <strong>Workspace context</strong>
              <span>Inspect, explain, and review this system</span>
            </div>
            <button className="icon-command" type="button" onClick={() => setRightRailOpen(false)} title="Collapse context drawer">
              <ChevronRight size={16} />
            </button>
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
              selectedComponent={selectedComponent}
              selectedConnector={selectedConnector}
              commentDraft={commentDraft}
              reviewSummaryDrafts={reviewSummaryDrafts}
              isSaving={collaborationState === 'saving'}
              error={collaborationError}
              offline={collaborationOffline}
              onCommentDraftChange={setCommentDraft}
              onSubmitComment={() => void submitDesignComment()}
              onRequestReview={() => setRequestReviewOpen(true)}
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
              <RequirementPanel design={design} onChange={updateDesign} compact />
            </section>
          ) : (
            <JourneyPanel
              design={design}
              activeJourneyId={activeJourneyId}
              activeStepIndex={activeJourneyStepIndex}
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
          isDeleting={homeAction === 'delete-workspace' || homeAction === 'delete-design'}
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
          isSaving={collaborationState === 'saving'}
          onCancel={() => setRequestReviewOpen(false)}
          onRequest={(reviewerId, message) => void submitReviewRequest(reviewerId, message)}
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

function AppNavbar({
  profile,
  route,
  workspaceName,
  designTitle,
  designs = [],
  selectedDesignId,
  aiConnection,
  currentUser,
  notifications,
  onHome,
  onWorkspace,
  onSelectDesign,
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
  aiConnection?: AIConnectionMetadata | null
  currentUser: BackendProfile | null
  notifications: BackendNotification[]
  onHome: () => void
  onWorkspace: () => void
  onSelectDesign?: (design: BackendDesign) => void
  onAdmin: () => void
  onOpenNotifications: () => void
  onLogout: () => void
}) {
  const unreadCount = notifications.filter((notification) => !notification.read).length
  const activeUser = currentUser ?? profile
  return (
    <header className="app-navbar">
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

class StratumCanvasBoundary extends Component<
  { children: ReactNode; onResetCanvas: () => void },
  { error: Error | null }
> {
  state: { error: Error | null } = { error: null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Stratum canvas error', error, info)
  }

  render() {
    if (!this.state.error) return this.props.children

    return (
      <div className="canvas-error-panel">
        <div>
          <strong>Canvas could not be opened</strong>
          <span>{this.state.error.message || 'The saved canvas data is invalid or from an unsupported version.'}</span>
        </div>
        <div className="canvas-error-actions">
          <button
            onClick={() => {
              this.props.onResetCanvas()
              this.setState({ error: null })
            }}
          >
            Reset canvas data
          </button>
          <button onClick={() => window.location.reload()}>Refresh</button>
        </div>
      </div>
    )
  }
}

function userDisplayName(users: BackendUser[], userId: string) {
  return users.find((user) => user.id === userId)?.displayName ?? userId.replaceAll('-', ' ')
}

function initialsFor(value: string) {
  const words = value.trim().split(/[\s@._-]+/).filter(Boolean)
  return words
    .slice(0, 2)
    .map((word) => word[0]?.toUpperCase())
    .join('') || 'U'
}

function formatCompactDateTime(value?: string) {
  if (!value) return 'Never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Never'
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

function roleLabel(role: string) {
  if (role === 'admin') return 'Admin'
  if (role === 'architect') return 'Architect'
  if (role === 'reviewer') return 'Reviewer'
  return 'Member'
}

function FirstAdminOnboarding({
  error,
  isSaving,
  onCreate,
}: {
  error: string | null
  isSaving: boolean
  onCreate: (input: { displayName: string; email: string; password: string }) => void
}) {
  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  return (
    <main className="onboarding-shell">
      <section className="onboarding-card">
        <div className="home-logo">S</div>
        <p className="eyebrow">First run setup</p>
        <h1>Create the first admin</h1>
        <p>Stratum has no users yet. The first account becomes the organization admin and can configure SSO and user access.</p>
        {error ? <div className="home-error inline-error"><span>{error}</span></div> : null}
        <Field label="Name" value={displayName} onChange={setDisplayName} />
        <Field label="Work email" value={email} onChange={setEmail} />
        <Field label="Password" type="password" value={password} onChange={setPassword} />
        <button
          className="primary-action full-width"
          type="button"
          disabled={isSaving || !displayName.trim() || !email.trim() || password.length < 8}
          onClick={() => onCreate({ displayName, email, password })}
        >
          <ShieldCheck size={17} /> {isSaving ? 'Creating admin...' : 'Create admin'}
        </button>
      </section>
    </main>
  )
}

function LoginScreen({
  requiresPasswordSetup,
  error,
  isSaving,
  onLogin,
  onSetInitialPassword,
}: {
  requiresPasswordSetup: boolean
  error: string | null
  isSaving: boolean
  onLogin: (input: { email: string; password: string }) => void
  onSetInitialPassword: (input: { email: string; password: string }) => void
}) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const title = requiresPasswordSetup ? 'Set admin password' : 'Sign in to Stratum'
  const description = requiresPasswordSetup
    ? 'Existing users were created before local passwords existed. Set the first admin password to continue.'
    : 'Use your workspace account to access designs, reviews, and admin settings.'
  return (
    <main className="onboarding-shell">
      <section className="onboarding-card">
        <div className="home-logo">S</div>
        <p className="eyebrow">Enterprise access</p>
        <h1>{title}</h1>
        <p>{description}</p>
        {error ? <div className="home-error inline-error"><span>{error}</span></div> : null}
        <Field label="Work email" value={email} onChange={setEmail} />
        <Field label="Password" type="password" value={password} onChange={setPassword} />
        <button
          className="primary-action full-width"
          type="button"
          disabled={isSaving || !email.trim() || password.length < 8}
          onClick={() => (requiresPasswordSetup ? onSetInitialPassword({ email, password }) : onLogin({ email, password }))}
        >
          <ShieldCheck size={17} /> {isSaving ? 'Checking...' : requiresPasswordSetup ? 'Set password and continue' : 'Sign in'}
        </button>
      </section>
    </main>
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
  activeSection,
  onSelectSection,
}: {
  users: BackendUser[]
  activeUserId: string
  workspaces: BackendWorkspace[]
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
  const [adminSearch, setAdminSearch] = useState('')
  const [catalogAdminSearch, setCatalogAdminSearch] = useState('')
  const [accessScope, setAccessScope] = useState<'workspace' | 'design'>('workspace')
  const [accessWorkspaceId, setAccessWorkspaceId] = useState(workspaces[0]?.id ?? '')
  const [accessDesignId, setAccessDesignId] = useState('')
  const [accessDesigns, setAccessDesigns] = useState<BackendDesign[]>([])
  const [workspaceAccess, setWorkspaceAccess] = useState<BackendWorkspaceAccess[]>([])
  const [designAccess, setDesignAccess] = useState<BackendDesignAccess[]>([])
  const [accessUserId, setAccessUserId] = useState('')
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
  const accessRows = accessScope === 'workspace' ? workspaceAccess : designAccess
  const grantableUsers = useMemo(() => users.filter((user) => user.status !== 'disabled'), [users])
  const selectedAccessUser = grantableUsers.find((user) => user.id === accessUserId)
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
    let cancelled = false
    async function loadAccessCenter() {
      if (activeSection !== 'access' || !accessWorkspaceId) return
      setAccessLoading(true)
      setAccessError(null)
      try {
        const [designResponse, workspaceAccessResponse] = await Promise.all([
          fetchWorkspaceDesigns(accessWorkspaceId),
          fetchWorkspaceAccess(accessWorkspaceId),
        ])
        if (cancelled) return
        setAccessDesigns(designResponse.designs)
        setWorkspaceAccess(workspaceAccessResponse.access)
        const nextDesignId = accessDesignId || designResponse.designs[0]?.id || ''
        if (!accessDesignId && nextDesignId) setAccessDesignId(nextDesignId)
        if (nextDesignId) {
          const designAccessResponse = await fetchDesignAccess(accessWorkspaceId, nextDesignId)
          if (!cancelled) setDesignAccess(designAccessResponse.access)
        } else {
          setDesignAccess([])
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

  function updateDraftUser(user: BackendUser, patch: Partial<BackendUser & { password?: string }>) {
    setDraftUsers((current) => current.map((item) => (item.id === user.id ? { ...item, ...patch } : item)))
  }

  function updateDraftAsset(asset: BackendCatalogAsset, patch: Partial<BackendCatalogAsset>) {
    setDraftAssets((current) => current.map((item) => (item.id === asset.id ? { ...item, ...patch } : item)))
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

  async function submitAccessGrant() {
    if (!accessWorkspaceId || !accessUserId) return
    setAccessError(null)
    setAccessLoading(true)
    try {
      if (accessScope === 'workspace') {
        const response = await grantWorkspaceAccess(accessWorkspaceId, { userId: accessUserId, ...workspaceAccessDraft })
        setWorkspaceAccess((current) => {
          const existing = current.filter((item) => item.userId !== response.access.userId)
          return [...existing, response.access]
        })
      } else if (accessDesignId) {
        const response = await grantDesignAccess(accessWorkspaceId, accessDesignId, { userId: accessUserId, ...designAccessDraft })
        setDesignAccess((current) => {
          const existing = current.filter((item) => item.userId !== response.access.userId)
          return [...existing, response.access]
        })
      }
    } catch (error) {
      setAccessError(error instanceof Error ? error.message : 'Could not save access grant.')
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
    } catch (error) {
      setAccessError(error instanceof Error ? error.message : 'Could not revoke access grant.')
    } finally {
      setAccessLoading(false)
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
                    <small>{selectedAccessUser ? selectedAccessUser.email : 'Choose a user'}</small>
                  </div>
                  <label className="field">
                    <span>User</span>
                    <select value={accessUserId} onChange={(event) => setAccessUserId(event.target.value)}>
                      {grantableUsers.map((user) => (
                        <option key={user.id} value={user.id}>
                          {user.displayName} · {user.email}
                        </option>
                      ))}
                    </select>
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
                  <button className="primary-action full-width" type="button" disabled={accessLoading || !accessUserId || (accessScope === 'design' && !accessDesignId)} onClick={() => void submitAccessGrant()}>
                    <ShieldCheck size={16} /> Save grant
                  </button>
                </section>
              </div>

              {accessError ? <div className="admin-inline-error">{accessError}</div> : null}

              <div className="access-grant-list">
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
            <div className="admin-panel">
              <div className="admin-panel-heading">
                <span><Settings size={18} /></span>
                <div>
                  <strong>System settings</strong>
                  <p>Reserved for deployment defaults, retention policy, export policy, and workspace templates.</p>
                </div>
              </div>
              <div className="analysis-empty">
                <strong>Settings modules will land here</strong>
                <span>This keeps future configuration out of the canvas and out of crowded one-page admin forms.</span>
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

function reviewStatusLabel(status: BackendDesignReviewRequest['status']) {
  if (status === 'changes_requested') return 'Changes requested'
  if (status === 'approved') return 'Approved'
  return 'Requested'
}

function ReviewWorkspace({
  mode,
  users,
  activeUserId,
  comments,
  reviews,
  selectedComponent,
  selectedConnector,
  commentDraft,
  reviewSummaryDrafts,
  isSaving,
  error,
  offline,
  onCommentDraftChange,
  onSubmitComment,
  onRequestReview,
  onReviewSummaryChange,
  onCompleteReview,
  onRefresh,
}: {
  mode: 'view' | 'comment'
  users: BackendUser[]
  activeUserId: string
  comments: BackendDesignComment[]
  reviews: BackendDesignReviewRequest[]
  selectedComponent: DesignComponent | null
  selectedConnector: DesignConnector | null
  commentDraft: string
  reviewSummaryDrafts: Record<string, string>
  isSaving: boolean
  error: string | null
  offline: boolean
  onCommentDraftChange: (value: string) => void
  onSubmitComment: () => void
  onRequestReview: () => void
  onReviewSummaryChange: (reviewId: string, summary: string) => void
  onCompleteReview: (reviewId: string, status: BackendDesignReviewRequest['status']) => void
  onRefresh: () => void
}) {
  const selectedTarget = selectedComponent?.name || (selectedConnector ? 'selected connection' : 'whole design')

  return (
    <div className="review-workspace">
      <div className="inspector-heading">
        {mode === 'comment' ? <MessageSquare size={18} /> : <Eye size={18} />}
        <div>
          <strong>{mode === 'comment' ? 'Comment mode' : 'View mode'}</strong>
          <span>Canvas is locked while reviews are captured</span>
        </div>
      </div>

      {offline ? (
        <div className="offline-notice">
          <strong>Collaboration backend offline</strong>
          <span>View mode still works. Start the backend to load or capture comments and review requests.</span>
        </div>
      ) : null}
      {error ? <div className="analysis-error">{error}</div> : null}

      <div className="review-actions">
        <button className="primary-action full-width" type="button" onClick={onRequestReview} disabled={isSaving || offline}>
          <UserPlus size={16} /> Request review
        </button>
        <button className="secondary-action compact-action" type="button" onClick={onRefresh} disabled={isSaving}>
          Refresh
        </button>
      </div>

      {mode === 'comment' ? (
        <section className="review-card">
          <div className="section-title">Add comment</div>
          <span className="comment-target">Target: {selectedTarget}</span>
          <textarea
            value={commentDraft}
            rows={4}
            placeholder="Capture a question, decision, or review note..."
            onChange={(event) => onCommentDraftChange(event.target.value)}
          />
          <button className="primary-action full-width" type="button" onClick={onSubmitComment} disabled={isSaving || offline || !commentDraft.trim()}>
            <Send size={16} /> Add comment
          </button>
        </section>
      ) : null}

      <section className="review-card">
        <div className="section-title">Review history</div>
        {reviews.length ? (
          <div className="activity-list">
            {reviews.map((review) => {
              const isAssignedToActiveUser = review.reviewerId === activeUserId && review.status === 'requested'
              return (
                <article className="activity-item" key={review.id}>
                  <div>
                    <strong>{userDisplayName(users, review.reviewerId)}</strong>
                    <span className={`status-pill ${review.status}`}>{reviewStatusLabel(review.status)}</span>
                  </div>
                  {review.message ? <p>{review.message}</p> : null}
                  {review.summary ? <p>{review.summary}</p> : null}
                  <small>
                    Requested by {userDisplayName(users, review.requestedBy)} · {new Date(review.updatedAt).toLocaleString()}
                  </small>
                  {isAssignedToActiveUser ? (
                    <div className="review-response">
                      <textarea
                        rows={3}
                        value={reviewSummaryDrafts[review.id] ?? review.summary}
                        placeholder="Summarize your review..."
                        onChange={(event) => onReviewSummaryChange(review.id, event.target.value)}
                      />
                      <div className="review-response-actions">
                        <button className="secondary-action compact-action" type="button" onClick={() => onCompleteReview(review.id, 'changes_requested')} disabled={isSaving}>
                          Request changes
                        </button>
                        <button className="primary-action" type="button" onClick={() => onCompleteReview(review.id, 'approved')} disabled={isSaving}>
                          <CheckCircle2 size={16} /> Approve
                        </button>
                      </div>
                    </div>
                  ) : null}
                </article>
              )
            })}
          </div>
        ) : (
          <div className="analysis-empty">
            <strong>No reviews yet</strong>
            <span>Ask a teammate to review the current design when it is ready.</span>
          </div>
        )}
      </section>

      <section className="review-card">
        <div className="section-title">Comments</div>
        {comments.length ? (
          <div className="activity-list">
            {comments.map((comment) => (
              <article className="activity-item" key={comment.id}>
                <div>
                  <strong>{userDisplayName(users, comment.authorId)}</strong>
                  <span>{new Date(comment.createdAt).toLocaleString()}</span>
                </div>
                <p>{comment.body}</p>
                {comment.componentId ? <small>Component: {comment.componentId}</small> : null}
                {comment.connectorId ? <small>Connector: {comment.connectorId}</small> : null}
              </article>
            ))}
          </div>
        ) : (
          <div className="analysis-empty">
            <strong>No comments yet</strong>
            <span>Switch to comment mode and capture feedback without changing the canvas.</span>
          </div>
        )}
      </section>
    </div>
  )
}

function RequestReviewModal({
  users,
  activeUserId,
  isSaving,
  onCancel,
  onRequest,
}: {
  users: BackendUser[]
  activeUserId: string
  isSaving: boolean
  onCancel: () => void
  onRequest: (reviewerId: string, message: string) => void
}) {
  const reviewers = users.filter((user) => user.id !== activeUserId)
  const [reviewerId, setReviewerId] = useState(reviewers[0]?.id ?? '')
  const [message, setMessage] = useState('')

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="request-review-modal" role="dialog" aria-modal="true" aria-labelledby="request-review-title">
        <div className="modal-heading">
          <p className="eyebrow">Request review</p>
          <h2 id="request-review-title">Add a reviewer</h2>
          <span>The reviewer gets a notification and can approve or request changes from view mode.</span>
        </div>
        <label className="field">
          <span>Reviewer</span>
          <select value={reviewerId} onChange={(event) => setReviewerId(event.target.value)}>
            {reviewers.map((user) => (
              <option key={user.id} value={user.id}>
                {user.displayName} · {user.role}
              </option>
            ))}
          </select>
        </label>
        <TextareaField label="Message" value={message} onChange={setMessage} />
        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onCancel} disabled={isSaving}>
            Cancel
          </button>
          <button className="primary-action" type="button" onClick={() => onRequest(reviewerId, message)} disabled={isSaving || !reviewerId}>
            <UserPlus size={16} /> Request review
          </button>
        </div>
      </section>
    </div>
  )
}

function NotificationsModal({
  notifications,
  users,
  onMarkRead,
  onClose,
}: {
  notifications: BackendNotification[]
  users: BackendUser[]
  onMarkRead: (notificationId: string) => void
  onClose: () => void
}) {
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="notifications-modal" role="dialog" aria-modal="true" aria-labelledby="notifications-title">
        <div className="analysis-modal-header">
          <div className="modal-heading">
            <p className="eyebrow">Notifications</p>
            <h2 id="notifications-title">Review activity</h2>
            <span>Comments and review updates for the active dummy user.</span>
          </div>
          <button className="secondary-action" type="button" onClick={onClose}>
            Close
          </button>
        </div>
        <div className="activity-list notification-list">
          {notifications.length ? (
            notifications.map((notification) => (
              <article className={`activity-item ${notification.read ? '' : 'unread'}`} key={notification.id}>
                <div>
                  <strong>{notification.title}</strong>
                  <span>{new Date(notification.createdAt).toLocaleString()}</span>
                </div>
                <p>{notification.body}</p>
                <small>For {userDisplayName(users, notification.userId)}</small>
                {!notification.read ? (
                  <button className="text-button" type="button" onClick={() => onMarkRead(notification.id)}>
                    Mark read
                  </button>
                ) : null}
              </article>
            ))
          ) : (
            <div className="analysis-empty">
              <strong>No notifications</strong>
              <span>Review requests and comments will appear here.</span>
            </div>
          )}
        </div>
      </section>
    </div>
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
  onCreatePrimaryJourney,
  onSelectJourney,
  onChangeJourney,
  onDeleteJourney,
  onSetStep,
}: {
  design: DesignDocument
  activeJourneyId: string | null
  activeStepIndex: number
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
        <button className="primary-action full-width" type="button" onClick={onCreatePrimaryJourney}>
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
              <Field label="Journey title" value={activeJourney.title} onChange={(title) => updateActiveJourney({ title })} />
              <TextareaField
                label="Journey summary"
                value={activeJourney.description}
                onChange={(description) => updateActiveJourney({ description })}
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

              <button className="text-button danger" type="button" onClick={() => onDeleteJourney(activeJourney.id)}>
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
}: {
  design: DesignDocument
  onChange: (updater: (current: DesignDocument) => DesignDocument) => void
  compact?: boolean
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
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, useCase: value } }))
        }
      />
      <TextareaField
        label="Functional requirements"
        value={brief.functionalRequirements}
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
        onChange={(targetRps) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, targetRps } }))
        }
      />
      <Field
        label="Availability"
        value={brief.availabilityRequirement}
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
        onChange={(sla) => onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, sla } }))}
      />
      <TextareaField
        label="Consistency requirements"
        value={brief.consistencyNotes}
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, consistencyNotes: value } }))
        }
      />
      <TextareaField
        label="Non-functional requirements"
        value={brief.nonFunctionalRequirements}
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
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, trafficNotes: value } }))
        }
      />
      <TextareaField
        label="Primary actors"
        value={brief.primaryActors}
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, primaryActors: value } }))
        }
      />
      <TextareaField
        label="Problem statement"
        value={brief.problemStatement}
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, problemStatement: value } }))
        }
      />
      <TextareaField
        label="Open questions"
        value={brief.openQuestions}
        onChange={(value) =>
          onChange((current) => ({ ...current, requirementBrief: { ...current.requirementBrief, openQuestions: value } }))
        }
      />
    </div>
  )
}

function ConfirmDeleteModal({
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
  const name = isWorkspace
    ? target.workspace.name
    : target.design.name || target.design.document.title || 'Untitled design'
  const title = isWorkspace ? 'Delete workspace?' : 'Delete design?'
  const detail = isWorkspace
    ? 'This workspace is empty. Deleting it cannot be undone.'
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
          <span>{isWorkspace ? 'Workspace' : 'Design'}</span>
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

function HomeScreen({
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
  onDeleteDesign,
  onNewDesign,
  onRefresh,
}: {
  workspaces: BackendWorkspace[]
  designs: BackendDesign[]
  activeWorkspaceId: string
  newWorkspaceName: string
  error: string | null
  action: 'workspace' | 'design' | 'delete-workspace' | 'delete-design' | null
  onWorkspaceNameChange: (value: string) => void
  onSelectWorkspace: (workspaceId: string) => void
  onCreateWorkspace: () => void | Promise<void>
  onDeleteWorkspace: (workspace: BackendWorkspace) => void
  onOpenDesign: (design: BackendDesign) => void
  onDeleteDesign: (design: BackendDesign) => void
  onNewDesign: () => void | Promise<void>
  onRefresh: () => void
}) {
  const activeWorkspace = workspaces.find((workspace) => workspace.id === activeWorkspaceId)
  const isCreatingWorkspace = action === 'workspace'
  const isCreatingDesign = action === 'design'
  const canDeleteActiveWorkspace = activeWorkspaceId !== defaultWorkspace.id && designs.length === 0

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
          <div className="workspace-list">
            {workspaces.map((workspace) => (
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

          {designs.length ? (
            <div className="home-design-grid">
              {designs.map((backendDesign) => (
                <article className="home-design-card" key={backendDesign.id}>
                  <button type="button" className="home-design-open" onClick={() => onOpenDesign(backendDesign)}>
                    <div className="design-card-preview">
                      <span>{backendDesign.document.components.length}</span>
                      <small>components</small>
                    </div>
                    <strong>{backendDesign.name || backendDesign.document.title || 'Untitled design'}</strong>
                    <span>
                      v{backendDesign.versionNumber ?? 1} • {new Date(backendDesign.updatedAt).toLocaleString()}
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
                </article>
              ))}
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

function ComponentInspector({
  component,
  currentDesignId,
  workspaceDesigns,
  catalogAssets,
  onChange,
  onDelete,
  onOpenLinkedDesign,
  onCreateCatalogAsset,
}: {
  component: DesignComponent
  currentDesignId: string
  workspaceDesigns: BackendDesign[]
  catalogAssets: BackendCatalogAsset[]
  onChange: (component: DesignComponent) => void
  onDelete: () => void
  onOpenLinkedDesign: (design: BackendDesign) => void
  onCreateCatalogAsset: (input: {
    name: string
    type: string
    owner?: string
    description?: string
    criticality?: string
    tags?: string[]
  }) => Promise<BackendCatalogAsset>
}) {
  const [catalogError, setCatalogError] = useState<string | null>(null)

  if (component.type === 'design.link') {
    return (
      <LinkedDesignInspector
        component={component}
        currentDesignId={currentDesignId}
        workspaceDesigns={workspaceDesigns}
        onChange={onChange}
        onDelete={onDelete}
        onOpenLinkedDesign={onOpenLinkedDesign}
      />
    )
  }

  const linkedAsset = component.metadata.enterpriseAsset
    ? catalogAssets.find((asset) => asset.id === component.metadata.enterpriseAsset?.assetId)
    : null
  const exactDuplicateCandidates = catalogAssets.filter(
    (asset) => normalizedCatalogLabel(asset.name) === normalizedCatalogLabel(component.name),
  )
  const duplicateCandidates = catalogAssets.filter((asset) => isPotentialCatalogMatch(component.name, asset.name))

  async function promoteToCatalog() {
    setCatalogError(null)
    try {
      const created = await onCreateCatalogAsset({
        name: component.name,
        type: component.type,
        owner: component.owner,
        description: component.purpose,
        criticality: component.criticality,
      })
      linkCatalogAsset(created)
    } catch (error) {
      setCatalogError(error instanceof Error ? error.message : 'Could not add component to catalog.')
    }
  }

  function linkCatalogAsset(asset: BackendCatalogAsset) {
    onChange({
      ...component,
      name: component.name.trim() ? component.name : asset.name,
      owner: component.owner.trim() ? component.owner : asset.owner,
      criticality: (asset.criticality || component.criticality) as DesignComponent['criticality'],
      metadata: {
        ...component.metadata,
        enterpriseAsset: {
          assetId: asset.id,
          name: asset.name,
          type: asset.type,
          owner: asset.owner,
          criticality: asset.criticality,
          linkedAt: new Date().toISOString(),
        },
      },
    })
  }

  function unlinkCatalogAsset() {
    const metadata = { ...component.metadata }
    delete metadata.enterpriseAsset
    onChange({ ...component, metadata })
  }

  return (
    <div className="inspector">
      <div className="inspector-heading">
        {component.type.startsWith('data.') ? <Database size={18} /> : <Server size={18} />}
        <div>
          <strong>{component.name}</strong>
          <span>{component.type}</span>
        </div>
        <button className="text-button danger" type="button" onClick={onDelete}>
          Delete
        </button>
      </div>

      <Field label="Name" value={component.name} onChange={(name) => onChange({ ...component, name })} />
      <div className="inspector-group enterprise-link-card">
        <div className="section-title">Enterprise Catalog</div>
        {component.metadata.enterpriseAsset ? (
          <div className="catalog-link-summary">
            <span className="shared-entity-badge">Shared entity</span>
            <strong>{linkedAsset?.name ?? component.metadata.enterpriseAsset.name}</strong>
            <small>
              {linkedAsset
                ? `${linkedAsset.usedInDesignCount} linked design${linkedAsset.usedInDesignCount === 1 ? '' : 's'}`
                : 'Catalog asset not visible or deleted'}
            </small>
            <button className="text-button" type="button" onClick={unlinkCatalogAsset}>
              Unlink from catalog
            </button>
          </div>
        ) : (
          <>
            {duplicateCandidates.length ? (
              <div className="catalog-suggestions">
                <strong>Possible existing asset</strong>
                {duplicateCandidates.map((asset) => (
                  <button className="catalog-suggestion" type="button" key={asset.id} onClick={() => linkCatalogAsset(asset)}>
                    <span>{asset.name}</span>
                    <small>{asset.type} • {asset.usedInDesignCount} use{asset.usedInDesignCount === 1 ? '' : 's'}</small>
                  </button>
                ))}
              </div>
            ) : null}
            <label className="field">
              <span>Link existing asset</span>
              <select
                value=""
                onChange={(event) => {
                  const asset = catalogAssets.find((item) => item.id === event.target.value)
                  if (asset) linkCatalogAsset(asset)
                }}
              >
                <option value="">Choose from catalog...</option>
                {catalogAssets.map((asset) => (
                  <option key={asset.id} value={asset.id}>
                    {asset.name} ({asset.type})
                  </option>
                ))}
              </select>
            </label>
            {catalogError ? <div className="admin-inline-error">{catalogError}</div> : null}
            <button
              className="command"
              type="button"
              disabled={!component.name.trim() || exactDuplicateCandidates.length > 0}
              onClick={() => void promoteToCatalog()}
              title={exactDuplicateCandidates.length ? 'Link the existing catalog asset instead of creating a duplicate.' : 'Create catalog asset'}
            >
              <FilePlus2 size={16} /> Add to catalog
            </button>
          </>
        )}
      </div>
      <Field label="Owner" value={component.owner} onChange={(owner) => onChange({ ...component, owner })} />
      <TextareaField
        label="Purpose"
        value={component.purpose}
        onChange={(purpose) => onChange({ ...component, purpose })}
      />
      <div className="inspector-group">
        <div className="section-title">Notes</div>
        <button
          className="command"
          onClick={() =>
            onChange({
              ...component,
              notes: [
                ...component.notes,
                {
                  id: `note_${crypto.randomUUID()}`,
                  body: '',
                  tone: 'neutral',
                },
              ],
            })
          }
        >
          <MessageSquarePlus size={16} /> Add note
        </button>
        {component.notes.map((note) => (
          <div className={`note-editor ${note.tone}`} key={note.id}>
            <select
              value={note.tone}
              onChange={(event) =>
                onChange({
                  ...component,
                  notes: component.notes.map((item) =>
                    item.id === note.id ? { ...item, tone: event.target.value as typeof note.tone } : item,
                  ),
                })
              }
            >
              <option value="neutral">Neutral</option>
              <option value="risk">Risk</option>
              <option value="decision">Decision</option>
              <option value="question">Question</option>
            </select>
            <textarea
              value={note.body}
              placeholder="Add context, risk, decision, or reviewer question..."
              onChange={(event) =>
                onChange({
                  ...component,
                  notes: component.notes.map((item) => (item.id === note.id ? { ...item, body: event.target.value } : item)),
                })
              }
              rows={3}
            />
            <button
              className="text-button danger"
              onClick={() => onChange({ ...component, notes: component.notes.filter((item) => item.id !== note.id) })}
            >
              Remove
            </button>
          </div>
        ))}
      </div>
      <label className="field">
        <span>Criticality</span>
        <select
          value={component.criticality}
          onChange={(event) => onChange({ ...component, criticality: event.target.value as DesignComponent['criticality'] })}
        >
          <option value="low">Low</option>
          <option value="medium">Medium</option>
          <option value="high">High</option>
          <option value="critical">Critical</option>
        </select>
      </label>

      <div className="inspector-group">
        <div className="section-title">Evaluation</div>
        <NumberField
          label="Expected QPS"
          value={component.metadata.expectedQps ?? null}
          onChange={(expectedQps) => onChange({ ...component, metadata: { ...component.metadata, expectedQps } })}
        />
        <NumberField
          label="Latency budget ms"
          value={component.metadata.latencyBudgetMs ?? null}
          onChange={(latencyBudgetMs) => onChange({ ...component, metadata: { ...component.metadata, latencyBudgetMs } })}
        />
      </div>

      {component.type === 'data.redis' && <RedisInspector component={component} onChange={onChange} />}
    </div>
  )
}

function LinkedDesignInspector({
  component,
  currentDesignId,
  workspaceDesigns,
  onChange,
  onDelete,
  onOpenLinkedDesign,
}: {
  component: DesignComponent
  currentDesignId: string
  workspaceDesigns: BackendDesign[]
  onChange: (component: DesignComponent) => void
  onDelete: () => void
  onOpenLinkedDesign: (design: BackendDesign) => void
}) {
  const linkableDesigns = workspaceDesigns.filter((design) => design.id !== currentDesignId)
  const linkedDesign = component.metadata.linkedDesign
  const linkedBackendDesign = linkedDesign
    ? workspaceDesigns.find((design) => design.id === linkedDesign.designId && design.workspaceId === linkedDesign.workspaceId)
    : null
  const currentLinkedTitle = linkedBackendDesign
    ? linkedBackendDesign.name || linkedBackendDesign.document.title || 'Untitled design'
    : linkedDesign?.title

  useEffect(() => {
    if (!linkedDesign || !linkedBackendDesign) return
    const title = linkedBackendDesign.name || linkedBackendDesign.document.title || 'Untitled design'
    if (
      linkedDesign.title === title &&
      linkedDesign.access === linkedBackendDesign.access &&
      linkedDesign.updatedAt === linkedBackendDesign.updatedAt
    ) {
      return
    }
    onChange({
      ...component,
      metadata: {
        ...component.metadata,
        linkedDesign: {
          workspaceId: linkedBackendDesign.workspaceId,
          designId: linkedBackendDesign.id,
          title,
          access: linkedBackendDesign.access,
          updatedAt: linkedBackendDesign.updatedAt,
        },
      },
    })
  }, [component, linkedBackendDesign, linkedDesign, onChange])

  function updateLinkedDesign(designId: string) {
    if (!designId) {
      const metadata = { ...component.metadata }
      delete metadata.linkedDesign
      onChange({ ...component, metadata })
      return
    }
    const target = workspaceDesigns.find((design) => design.id === designId)
    if (!target) return
    const title = target.name || target.document.title || 'Untitled design'
    onChange({
      ...component,
      name: component.name === 'Linked Design' || component.name.trim() === '' ? title : component.name,
      metadata: {
        ...component.metadata,
        linkedDesign: {
          workspaceId: target.workspaceId,
          designId: target.id,
          title,
          access: target.access,
          updatedAt: target.updatedAt,
        },
      },
    })
  }

  return (
    <div className="inspector linked-design-inspector">
      <div className="inspector-heading">
        <LayoutDashboard size={18} />
        <div>
          <strong>{component.name || 'Linked Design'}</strong>
          <span>Reference to another design</span>
        </div>
        <button className="text-button danger" type="button" onClick={onDelete}>
          Delete
        </button>
      </div>

      <Field label="Label" value={component.name} onChange={(name) => onChange({ ...component, name })} />

      <div className="inspector-group">
        <div className="section-title">Target</div>
        <label className="field">
          <span>Target design</span>
          <select value={linkedDesign?.designId ?? ''} onChange={(event) => updateLinkedDesign(event.target.value)}>
            <option value="">Choose a design...</option>
            {linkableDesigns.map((design) => (
              <option key={design.id} value={design.id}>
                {design.name || design.document.title || 'Untitled design'}
              </option>
            ))}
          </select>
        </label>
        {linkedDesign ? (
          <>
            <Field label="Title" value={currentLinkedTitle ?? 'Unavailable design'} onChange={() => undefined} disabled />
            <Field
              label="Access"
              value={linkedBackendDesign?.access ?? `${linkedDesign.access} (not visible)`}
              onChange={() => undefined}
              disabled
            />
            <Field
              label="Last synced"
              value={new Date(linkedBackendDesign?.updatedAt ?? linkedDesign.updatedAt).toLocaleString()}
              onChange={() => undefined}
              disabled
            />
            <button
              className="command"
              type="button"
              disabled={!linkedBackendDesign}
              onClick={() => linkedBackendDesign && onOpenLinkedDesign(linkedBackendDesign)}
            >
              <LayoutDashboard size={16} /> Open linked design
            </button>
          </>
        ) : (
          <div className="analysis-empty compact-empty">
            <strong>No target selected</strong>
            <span>Links are limited to designs visible in this workspace.</span>
          </div>
        )}
      </div>
    </div>
  )
}

function ConnectorInspector({
  connector,
  onChange,
  onDelete,
}: {
  connector: DesignConnector
  onChange: (connector: DesignConnector) => void
  onDelete: () => void
}) {
  return (
    <div className="inspector">
      <div className="inspector-heading">
        <ChevronsLeftRight size={18} />
        <div>
          <strong>{connector.type.replaceAll('_', ' ')}</strong>
          <span>Connector</span>
        </div>
        <button className="text-button danger" type="button" onClick={onDelete}>
          Delete
        </button>
      </div>

      <SelectField
        label="Type"
        value={connector.type}
        values={['synchronous', 'asynchronous_event', 'batch_transfer', 'cache_read', 'cache_write', 'model_call', 'observability_signal']}
        onChange={(type) => onChange({ ...connector, type: type as ConnectorType })}
      />
      <Field label="Protocol" value={connector.protocol} onChange={(protocol) => onChange({ ...connector, protocol })} />
      <NumberField
        label="Timeout ms"
        value={connector.timeoutMs}
        onChange={(timeoutMs) => onChange({ ...connector, timeoutMs })}
      />
      <Field
        label="Consistency"
        value={connector.consistencyExpectation}
        onChange={(consistencyExpectation) => onChange({ ...connector, consistencyExpectation })}
      />
      <TextareaField label="Notes" value={connector.notes} onChange={(notes) => onChange({ ...connector, notes })} />
      <label className="field checkbox-field">
        <span>Animated flow</span>
        <input
          type="checkbox"
          checked={connector.animated}
          onChange={(event) => onChange({ ...connector, animated: event.target.checked })}
        />
      </label>
    </div>
  )
}

function RedisInspector({
  component,
  onChange,
}: {
  component: DesignComponent
  onChange: (component: DesignComponent) => void
}) {
  const redis = component.metadata.redis
  if (!redis) return null
  const currentRedis: RedisMetadata = redis

  function updateRedis(patch: Partial<RedisMetadata>) {
    onChange({ ...component, metadata: { ...component.metadata, redis: { ...currentRedis, ...patch } } })
  }

  return (
    <div className="inspector-group">
      <div className="section-title">Redis Details</div>
      <SelectField
        label="Usage"
        value={redis.usage}
        values={['unknown', 'cache', 'session_store', 'rate_limiter', 'pub_sub', 'lock_manager']}
        onChange={(usage) => updateRedis({ usage: usage as typeof redis.usage })}
      />
      <SelectField
        label="Cluster mode"
        value={redis.clusterMode}
        values={['unknown', 'enabled', 'disabled']}
        onChange={(clusterMode) => updateRedis({ clusterMode: clusterMode as typeof redis.clusterMode })}
      />
      <SelectField
        label="Replication"
        value={redis.replication}
        values={['unknown', 'none', 'primary_replica', 'multi_az', 'cross_region']}
        onChange={(replication) => updateRedis({ replication: replication as typeof redis.replication })}
      />
      <SelectField
        label="Persistence"
        value={redis.persistence}
        values={['unknown', 'none', 'rdb', 'aof', 'rdb_aof', 'managed_default']}
        onChange={(persistence) => updateRedis({ persistence: persistence as typeof redis.persistence })}
      />
      <SelectField
        label="Eviction policy"
        value={redis.evictionPolicy}
        values={['unknown', 'noeviction', 'allkeys_lru', 'volatile_lru', 'allkeys_lfu', 'volatile_ttl']}
        onChange={(evictionPolicy) => updateRedis({ evictionPolicy: evictionPolicy as typeof redis.evictionPolicy })}
      />
      <SelectField
        label="Consistency"
        value={redis.consistencyExpectation}
        values={['unknown', 'best_effort_cache', 'read_after_write', 'strong_for_locking', 'eventual']}
        onChange={(consistencyExpectation) =>
          updateRedis({ consistencyExpectation: consistencyExpectation as typeof redis.consistencyExpectation })
        }
      />
      <SelectField
        label="Failover"
        value={redis.failoverBehavior}
        values={['unknown', 'automatic', 'manual', 'data_loss_possible']}
        onChange={(failoverBehavior) => updateRedis({ failoverBehavior: failoverBehavior as typeof redis.failoverBehavior })}
      />
      <SelectField
        label="Backup/restore"
        value={redis.backupRestore}
        values={['unknown', 'configured', 'not_required', 'missing']}
        onChange={(backupRestore) => updateRedis({ backupRestore: backupRestore as typeof redis.backupRestore })}
      />
      <NumberField
        label="Memory limit GB"
        value={redis.memoryLimitGb}
        onChange={(memoryLimitGb) => updateRedis({ memoryLimitGb })}
      />
      <NumberField label="Redis QPS" value={redis.expectedQps} onChange={(expectedQps) => updateRedis({ expectedQps })} />
      <SelectField
        label="Hot key risk"
        value={redis.hotKeyRisk}
        values={['unknown', 'low', 'medium', 'high']}
        onChange={(hotKeyRisk) => updateRedis({ hotKeyRisk: hotKeyRisk as typeof redis.hotKeyRisk })}
      />
    </div>
  )
}

function StructuredView({ design }: { design: DesignDocument }) {
  return (
    <div className="inspector structured">
      <div className="inspector-heading">
        <Braces size={18} />
        <div>
          <strong>Structured JSON</strong>
          <span>Semantic model plus React Flow layout</span>
        </div>
      </div>
      <pre>{JSON.stringify(toExportableDesign(design), null, 2)}</pre>
    </div>
  )
}

function Field({
  label,
  value,
  onChange,
  disabled = false,
  type = 'text',
}: {
  label: string
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  type?: string
}) {
  return (
    <label className="field">
      <span>{label}</span>
      <input type={type} value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)} />
    </label>
  )
}

function TextareaField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="field">
      <span>{label}</span>
      <textarea value={value} onChange={(event) => onChange(event.target.value)} rows={4} />
    </label>
  )
}

function NumberField({
  label,
  value,
  onChange,
}: {
  label: string
  value: number | null
  onChange: (value: number | null) => void
}) {
  return (
    <label className="field">
      <span>{label}</span>
      <input
        type="number"
        min="0"
        value={value ?? ''}
        onChange={(event) => onChange(event.target.value === '' ? null : Number(event.target.value))}
      />
    </label>
  )
}

function SelectField({
  label,
  value,
  values,
  onChange,
}: {
  label: string
  value: string
  values: string[]
  onChange: (value: string) => void
}) {
  return (
    <label className="field">
      <span>{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {values.map((item) => (
          <option key={item} value={item}>
            {item.replaceAll('_', ' ')}
          </option>
        ))}
      </select>
    </label>
  )
}
