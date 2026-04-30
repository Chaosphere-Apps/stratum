import {
  Component,
  lazy,
  Suspense,
  type DragEvent,
  type ErrorInfo,
  type ReactNode,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import {
  BookOpen,
  Boxes,
  Braces,
  ChevronDown,
  ChevronRight,
  ChevronsLeftRight,
  FilePlus2,
  Database,
  Download,
  FolderPlus,
  LayoutDashboard,
  MessageSquarePlus,
  Monitor,
  Plus,
  RotateCcw,
  Route,
  Save,
  Server,
  Sparkles,
  SkipBack,
  SkipForward,
  Trash2,
} from 'lucide-react'
import { componentCatalog, getCatalogItem } from './catalog'
import {
  analyzeDesign,
  createDesign,
  createDesignDoc,
  createWorkspace,
  deleteDesign as deleteDesignFromBackend,
  deleteDesignDoc as deleteDesignDocFromBackend,
  deleteWorkspace as deleteWorkspaceFromBackend,
  fetchDesignDocs,
  fetchProfile,
  fetchWorkspaceDesign,
  fetchWorkspaceDesigns,
  fetchWorkspaces,
  saveDesignDocument as saveDesignDocumentToBackend,
  updateDesignDoc,
  updateDesignMetadata,
  verifyAIProvider,
  type BackendDesignDoc,
  type BackendProfile,
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
  | { screen: 'workspace'; workspaceId: string }
  | { screen: 'design'; workspaceId: string; designId: string }

type CanvasMode = 'design' | 'journey'

const defaultWorkspace: BackendWorkspace = {
  id: 'guest-workspace',
  name: 'Guest Workspace',
}

const RichTextDocEditor = lazy(() =>
  import('./components/RichTextDocEditor').then((module) => ({ default: module.RichTextDocEditor })),
)

interface AIConnectionMetadata {
  provider: string
  model: string
  baseUrl: string
  verifiedAt: string
}

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

const AI_CONNECTION_KEY = 'stratum.ai.connection'

function loadAIConnection(): AIConnectionMetadata | null {
  try {
    const raw = window.localStorage.getItem(AI_CONNECTION_KEY)
    return raw ? (JSON.parse(raw) as AIConnectionMetadata) : null
  } catch {
    return null
  }
}

function saveAIConnection(connection: AIConnectionMetadata) {
  window.localStorage.setItem(AI_CONNECTION_KEY, JSON.stringify(connection))
}

function parseRoute(pathname: string): AppRoute {
  const segments = pathname.split('/').filter(Boolean).map(decodeURIComponent)
  if (segments[0] === 'workspaces' && segments[1] && segments[2] === 'designs' && segments[3]) {
    return { screen: 'design', workspaceId: segments[1], designId: segments[3] }
  }
  if (segments[0] === 'workspaces' && segments[1]) {
    return { screen: 'workspace', workspaceId: segments[1] }
  }
  return { screen: 'home' }
}

function routePath(route: AppRoute) {
  if (route.screen === 'workspace') return `/workspaces/${encodeURIComponent(route.workspaceId)}`
  if (route.screen === 'design') {
    return `/workspaces/${encodeURIComponent(route.workspaceId)}/designs/${encodeURIComponent(route.designId)}`
  }
  return '/'
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

export function App() {
  const initialRoute = parseRoute(window.location.pathname)
  const [route, setRoute] = useState<AppRoute>(initialRoute)
  const [view, setView] = useState<'home' | 'canvas'>(initialRoute.screen === 'design' ? 'canvas' : 'home')
  const [activeWorkspaceId, setActiveWorkspaceId] = useState(
    initialRoute.screen === 'home' ? 'guest-workspace' : initialRoute.workspaceId,
  )
  const [selectedDesignId, setSelectedDesignId] = useState(
    initialRoute.screen === 'design' ? initialRoute.designId : null,
  )
  const [homeProfile, setHomeProfile] = useState<BackendProfile | null>(null)
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
  const [showExport, setShowExport] = useState(false)
  const [pendingConnector, setPendingConnector] = useState<PendingConnector | null>(null)
  const [focusMode, setFocusMode] = useState(false)
  const [designsOpen, setDesignsOpen] = useState(false)
  const [requirementsOpen, setRequirementsOpen] = useState(false)
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
  const [aiConnection, setAIConnection] = useState<AIConnectionMetadata | null>(() => loadAIConnection())
  const [aiSettingsOpen, setAISettingsOpen] = useState(false)
  const [copilotOpen, setCopilotOpen] = useState(false)
  const canvasHostRef = useRef<HTMLDivElement | null>(null)
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
  const isCanvasReadOnly = canvasMode === 'journey'

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

  function getDraggedComponentType(event: DragEvent) {
    return (
      event.dataTransfer.getData('application/x-sde-component-type') ||
      event.dataTransfer.getData('text/plain')
    ) as ComponentType
  }

  function handleCanvasDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault()
    event.stopPropagation()
    if (isCanvasReadOnly) return
    const type = getDraggedComponentType(event)
    if (!type || !componentCatalog.some((item) => item.type === type)) return
    const bounds = canvasHostRef.current?.getBoundingClientRect()
    createReactFlowComponent(type, {
      x: Math.max(80, event.clientX - (bounds?.left ?? 0) - 110),
      y: Math.max(80, event.clientY - (bounds?.top ?? 0) - 50),
    })
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
          onHome={() => applyRoute({ screen: 'home' })}
          onWorkspace={() => applyRoute({ screen: 'workspace', workspaceId: activeWorkspaceId })}
          onSelectDesign={selectBackendDesign}
          onOpenAISettings={() => setAISettingsOpen(true)}
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
        onHome={() => applyRoute({ screen: 'home' })}
        onWorkspace={() => applyRoute({ screen: 'workspace', workspaceId: activeWorkspaceId })}
        onSelectDesign={selectBackendDesign}
        onOpenAISettings={() => setAISettingsOpen(true)}
      />
      <main className={`app-shell ${focusMode ? 'focus-mode' : ''}`}>
        <aside className="left-rail">
          <div className="brand">
            <Boxes size={20} />
            <div>
              <strong>{backendSync.workspace?.name ?? 'Guest Workspace'}</strong>
              <span>{workspaceDesigns.length} design{workspaceDesigns.length === 1 ? '' : 's'}</span>
            </div>
          </div>

          <section className="panel-section">
            <div className="section-title collapsible-title">
              <button type="button" onClick={() => setDesignsOpen((value) => !value)}>
                {designsOpen ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
                Designs
              </button>
            </div>
            {designsOpen ? (
              <div className="design-list compact-design-list">
                {workspaceDesigns.map((backendDesign) => (
                  <div className="compact-design-row" key={backendDesign.id}>
                    <button
                      className={`design-list-item ${backendDesign.document.id === design.id ? 'active' : ''}`}
                      onClick={() => selectBackendDesign(backendDesign)}
                    >
                      <strong>{backendDesign.name || backendDesign.document.title || 'Untitled design'}</strong>
                      <small>{new Date(backendDesign.updatedAt).toLocaleString()}</small>
                    </button>
                    <button
                      className="icon-danger-button"
                      type="button"
                      aria-label={`Delete ${backendDesign.name || backendDesign.document.title || 'design'}`}
                      onClick={() => requestDeleteDesign(backendDesign)}
                    >
                      <Trash2 size={15} />
                    </button>
                  </div>
                ))}
              </div>
            ) : null}
            <button className="command" type="button" disabled={homeAction === 'design'} onClick={openNewDesignBrief}>
              <FilePlus2 size={16} /> {homeAction === 'design' ? 'Creating...' : 'New design'}
            </button>
          </section>

          <section className="panel-section">
            <div className="section-title">Components</div>
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
                  <span className="swatch" style={{ background: item.color }} />
                  <span>
                    <strong>{item.label}</strong>
                    <small>{item.description}</small>
                  </span>
                  <Plus size={16} />
                </button>
              ))}
            </div>
          </section>

          <section className="panel-section">
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
          <div className="top-actions">
            <div className="mode-switch" role="group" aria-label="Canvas mode">
              <button
                className={canvasMode === 'design' ? 'active' : ''}
                type="button"
                onClick={() => setCanvasMode('design')}
              >
                Edit design
              </button>
              <button
                className={canvasMode === 'journey' ? 'active' : ''}
                type="button"
                disabled={!activeJourney}
                onClick={() => {
                  setCanvasMode('journey')
                  setSelectedComponentId(null)
                  setSelectedConnectorId(null)
                }}
              >
                Journey view
              </button>
            </div>
            <button className="command compact" onClick={saveDraft}>
              <Save size={16} /> {saveState === 'saving' ? 'Saving' : saveState === 'saved' ? 'Saved' : 'Save'}
            </button>
            <button
              className="command compact primary-compact"
              onClick={() => openAnalysisModal({ run: analysisState !== 'running' })}
              disabled={!selectedDesignId}
            >
              <Sparkles size={16} /> {analysisState === 'running' ? 'Analysing' : 'Analyse'}
            </button>
            <button className="command compact" onClick={() => setCopilotOpen(true)} disabled={!aiConnection}>
              <Sparkles size={16} /> Copilot
            </button>
            <button className="command compact" onClick={() => setDocsModalOpen(true)}>
              <BookOpen size={16} /> Docs
            </button>
            {saveState === 'error' ? <span className="save-error">Save failed</span> : null}
            {analysisState === 'ready' && analysisReport ? (
              <button className="analysis-pill button-pill" type="button" onClick={() => openAnalysisModal()}>
                Score {analysisReport.score}
              </button>
            ) : null}
            {analysisState === 'error' ? (
              <span className="save-error">Analysis failed</span>
            ) : null}
            <span className={`sync-pill ${backendSync.status}`}>{backendSync.status}</span>
            <button className="command compact" onClick={() => setShowExport((value) => !value)}>
              <Braces size={16} /> Structured view
            </button>
            <button className="icon-command" onClick={() => setFocusMode((value) => !value)} title="Toggle focus mode">
              <ChevronsLeftRight size={17} />
            </button>
          </div>
        </header>
        <div
          className="canvas-host"
          ref={canvasHostRef}
          onDragEnterCapture={(event) => {
            event.preventDefault()
            event.dataTransfer.dropEffect = 'copy'
          }}
          onDragOverCapture={(event) => {
            event.preventDefault()
            event.dataTransfer.dropEffect = 'copy'
          }}
          onDropCapture={handleCanvasDrop}
        >
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
              }}
              onSelectConnector={(connectorId) => {
                setSelectedConnectorId(connectorId || null)
                if (connectorId) setSelectedComponentId(null)
              }}
              onMoveComponent={moveReactFlowComponent}
              onResizeComponent={resizeReactFlowComponent}
              onConnectComponents={(fromComponentId, toComponentId) => setPendingConnector({ fromComponentId, toComponentId })}
              onReconnectConnector={reconnectConnector}
              onDuplicateComponent={duplicateReactFlowComponent}
              onDeleteComponents={deleteComponents}
              onDeleteConnectors={deleteConnectors}
            />
          </StratumCanvasBoundary>
        </div>
      </section>

        <aside className="right-rail">
          {showExport ? (
            <StructuredView design={design} />
          ) : !isCanvasReadOnly && selectedConnector ? (
            <ConnectorInspector
              connector={selectedConnector}
              onChange={updateConnectorFromInspector}
              onDelete={() => deleteConnectors([selectedConnector.id])}
            />
          ) : !isCanvasReadOnly && selectedComponent ? (
            <ComponentInspector
              component={selectedComponent}
              onChange={updateComponentFromInspector}
              onDelete={() => deleteComponents([selectedComponent.id])}
            />
          ) : (
            <AnalysisWorkspace
              design={design}
              requirementsOpen={requirementsOpen}
              activeJourneyId={activeJourneyId}
              activeJourneyStepIndex={activeJourneyStepIndex}
              onToggleRequirements={() => setRequirementsOpen((value) => !value)}
              onChangeRequirements={updateDesign}
              onCreatePrimaryJourney={createPrimaryJourney}
              onSelectJourney={(journeyId) => {
                setActiveJourneyId(journeyId)
                setActiveJourneyStepIndex(0)
                setCanvasMode('journey')
                setSelectedComponentId(null)
                setSelectedConnectorId(null)
              }}
              onChangeJourney={updateJourney}
              onDeleteJourney={deleteJourney}
              onSetJourneyStep={setActiveJourneyStepIndex}
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
      {aiSettingsOpen ? (
        <AISettingsModal
          connection={aiConnection}
          onClose={() => setAISettingsOpen(false)}
          onVerified={(connection) => {
            saveAIConnection(connection)
            setAIConnection(connection)
            setAISettingsOpen(false)
          }}
        />
      ) : null}
      {copilotOpen ? (
        <CopilotDraftModal
          aiConnection={aiConnection}
          onClose={() => setCopilotOpen(false)}
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
  onHome,
  onWorkspace,
  onSelectDesign,
  onOpenAISettings,
}: {
  profile: BackendProfile | null
  route: AppRoute
  workspaceName?: string
  designTitle: string
  designs?: BackendDesign[]
  selectedDesignId?: string | null
  aiConnection?: AIConnectionMetadata | null
  onHome: () => void
  onWorkspace: () => void
  onSelectDesign?: (design: BackendDesign) => void
  onOpenAISettings: () => void
}) {
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
        {route.screen !== 'home' ? (
          <button className={route.screen === 'workspace' ? 'active' : ''} onClick={onWorkspace}>
            {workspaceName ?? 'Workspace'}
          </button>
        ) : null}
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

      <button className="profile-chip" type="button" onClick={onOpenAISettings} title="Configure AI provider">
        <span>{profile?.displayName ?? 'Guest Designer'}</span>
        <strong>{aiConnection ? `${aiConnection.provider} connected` : 'AI not connected'}</strong>
      </button>
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

function AISettingsModal({
  connection,
  onClose,
  onVerified,
}: {
  connection: AIConnectionMetadata | null
  onClose: () => void
  onVerified: (connection: AIConnectionMetadata) => void
}) {
  const [provider, setProvider] = useState(connection?.provider ?? 'openai')
  const [model, setModel] = useState(connection?.model ?? 'gpt-5.1')
  const [baseUrl, setBaseUrl] = useState(connection?.baseUrl ?? '')
  const [apiKey, setApiKey] = useState('')
  const [status, setStatus] = useState<'idle' | 'verifying' | 'error'>('idle')
  const [message, setMessage] = useState<string | null>(null)

  async function verify() {
    setStatus('verifying')
    setMessage(null)
    try {
      const response = await verifyAIProvider({ provider, model, baseUrl, apiKey })
      if (!response.ok) {
        setStatus('error')
        setMessage(response.message)
        return
      }
      onVerified({
        provider,
        model,
        baseUrl,
        verifiedAt: response.verifiedAt ?? new Date().toISOString(),
      })
    } catch (error) {
      setStatus('error')
      setMessage(error instanceof Error ? error.message : 'Could not verify provider')
    }
  }

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="ai-settings-modal" role="dialog" aria-modal="true" aria-labelledby="ai-settings-title">
        <div className="modal-heading">
          <div>
            <p className="eyebrow">AI Provider</p>
            <h2 id="ai-settings-title">Connect analysis intelligence</h2>
            <span>Keys are sent to the backend for verification and are not stored in browser local storage.</span>
          </div>
        </div>
        <div className="brief-form-grid">
          <label className="field">
            <span>Provider</span>
            <select value={provider} onChange={(event) => setProvider(event.target.value)}>
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="openrouter">OpenRouter</option>
              <option value="custom">OpenAI-compatible</option>
            </select>
          </label>
          <Field label="Model" value={model} onChange={setModel} />
          <Field label="Base URL" value={baseUrl} onChange={setBaseUrl} />
          <label className="field">
            <span>API key</span>
            <input type="password" value={apiKey} onChange={(event) => setApiKey(event.target.value)} />
          </label>
        </div>
        {message ? <div className="analysis-error">{message}</div> : null}
        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onClose} disabled={status === 'verifying'}>
            Cancel
          </button>
          <button className="primary-action" type="button" onClick={() => void verify()} disabled={status === 'verifying'}>
            {status === 'verifying' ? 'Verifying...' : 'Verify connection'}
          </button>
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
                <div className="analysis-finding-list">
                  {analysisReport.findings.length ? (
                    analysisReport.findings.map((finding) => (
                      <article className={`analysis-finding ${finding.severity}`} key={`${finding.suite}-${finding.title}`}>
                        <span>{finding.severity} · {finding.suite}</span>
                        <strong>{finding.title}</strong>
                        <p>{finding.detail}</p>
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

function AnalysisWorkspace({
  design,
  requirementsOpen,
  activeJourneyId,
  activeJourneyStepIndex,
  onToggleRequirements,
  onChangeRequirements,
  onCreatePrimaryJourney,
  onSelectJourney,
  onChangeJourney,
  onDeleteJourney,
  onSetJourneyStep,
}: {
  design: DesignDocument
  requirementsOpen: boolean
  activeJourneyId: string | null
  activeJourneyStepIndex: number
  onToggleRequirements: () => void
  onChangeRequirements: (updater: (current: DesignDocument) => DesignDocument) => void
  onCreatePrimaryJourney: () => void
  onSelectJourney: (journeyId: string) => void
  onChangeJourney: (journey: DesignJourney) => void
  onDeleteJourney: (journeyId: string) => void
  onSetJourneyStep: (stepIndex: number) => void
}) {
  return (
    <div className="analysis-workspace">
      <section className="collapsible-panel">
        <button className="collapsible-panel-trigger" type="button" onClick={onToggleRequirements}>
          <span>
            {requirementsOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
            <Monitor size={18} />
            Requirement Brief
          </span>
          <small>{requirementsOpen ? 'Hide' : 'Edit'}</small>
        </button>
        {requirementsOpen ? <RequirementPanel design={design} onChange={onChangeRequirements} compact /> : null}
      </section>

      <JourneyPanel
        design={design}
        activeJourneyId={activeJourneyId}
        activeStepIndex={activeJourneyStepIndex}
        onCreatePrimaryJourney={onCreatePrimaryJourney}
        onSelectJourney={onSelectJourney}
        onChangeJourney={onChangeJourney}
        onDeleteJourney={onDeleteJourney}
        onSetStep={onSetJourneyStep}
      />
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
  onChange,
  onDelete,
}: {
  component: DesignComponent
  onChange: (component: DesignComponent) => void
  onDelete: () => void
}) {
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

function Field({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="field">
      <span>{label}</span>
      <input value={value} onChange={(event) => onChange(event.target.value)} />
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
