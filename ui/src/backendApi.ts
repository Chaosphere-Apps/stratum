import type { BackendDesign, BackendWorkspace } from './backendSync'

export interface BackendProfile {
  id: string
  displayName: string
  email?: string
  role: string
  status?: string
  setupRequired?: boolean
}

interface ProfileResponse extends BackendProfile {}
export interface BackendUser {
  id: string
  displayName: string
  email: string
  passwordSet: boolean
  role: string
  status: string
  lastSeenAt: string
  createdAt: string
  updatedAt: string
}
export interface BackendSignInConfig {
  localPasswordEnabled: boolean
  ssoEnabled: boolean
  provider: string
  oktaDomain: string
  issuer: string
  clientId: string
  clientSecretSet: boolean
  redirectUri: string
  postLogoutRedirectUri: string
  scopes: string
  groupsClaim: string
  adminGroup: string
  reviewerGroup: string
  jitProvisioning: boolean
  updatedAt: string
}
export interface BackendAIProviderConfig {
  enabled: boolean
  provider: string
  model: string
  baseUrl: string
  apiKeySet: boolean
  verifiedAt?: string
  updatedAt: string
}
export interface BackendMCPConfig {
  enabled: boolean
  endpointPath: string
  readCatalog: boolean
  readDesigns: boolean
  createDraftDesign: boolean
  runAnalysis: boolean
  fetchImpactReport: boolean
  requireAdminConsent: boolean
  updatedAt: string
}
export interface BackendCatalogAsset {
  id: string
  name: string
  normalizedName: string
  type: string
  owner: string
  description: string
  criticality: string
  tags: string[]
  metadata?: Record<string, unknown>
  createdBy: string
  createdAt: string
  updatedAt: string
  usedInDesignCount: number
}
export interface BackendWorkspaceAccess {
  workspaceId: string
  userId: string
  canRead: boolean
  canCreateDesign: boolean
  canManage: boolean
  createdAt: string
  updatedAt: string
}
export interface BackendDesignAccess {
  workspaceId: string
  designId: string
  userId: string
  canRead: boolean
  canEdit: boolean
  canComment: boolean
  canReview: boolean
  canManage: boolean
  createdAt: string
  updatedAt: string
}
interface CatalogAssetsResponse {
  assets: BackendCatalogAsset[]
}
interface CatalogAssetResponse {
  asset: BackendCatalogAsset
}
interface WorkspaceAccessResponse {
  access: BackendWorkspaceAccess[]
}
interface WorkspaceAccessGrantResponse {
  access: BackendWorkspaceAccess
}
interface DesignAccessResponse {
  access: BackendDesignAccess[]
}
interface DesignAccessGrantResponse {
  access: BackendDesignAccess
}
interface SetupStatusResponse {
  requiresSetup: boolean
  requiresPasswordSetup: boolean
}
interface UserResponse {
  user: BackendUser
}
interface AuthResponse {
  user: BackendUser
}
interface UsersResponse {
  users: BackendUser[]
}
interface SignInConfigResponse {
  signIn: BackendSignInConfig
}
interface AIProviderConfigResponse {
  aiProvider: BackendAIProviderConfig
}
interface MCPConfigResponse {
  mcp: BackendMCPConfig
}
export interface BackendDesignComment {
  id: string
  workspaceId: string
  designId: string
  authorId: string
  body: string
  componentId?: string
  connectorId?: string
  createdAt: string
}
interface DesignCommentsResponse {
  comments: BackendDesignComment[]
}
interface DesignCommentResponse {
  comment: BackendDesignComment
}
export interface BackendDesignReviewRequest {
  id: string
  workspaceId: string
  designId: string
  requestedBy: string
  reviewerId: string
  status: 'requested' | 'approved' | 'changes_requested'
  message: string
  summary: string
  createdAt: string
  updatedAt: string
  completedAt?: string
}
interface DesignReviewsResponse {
  reviews: BackendDesignReviewRequest[]
}
interface DesignReviewResponse {
  review: BackendDesignReviewRequest
}
export interface BackendNotification {
  id: string
  userId: string
  workspaceId: string
  designId: string
  type: string
  title: string
  body: string
  read: boolean
  createdAt: string
}
interface NotificationsResponse {
  notifications: BackendNotification[]
}
interface WorkspacesResponse {
  workspaces: BackendWorkspace[]
}
interface WorkspaceResponse {
  workspace: BackendWorkspace
}
interface DesignsResponse {
  designs: BackendDesign[]
}
interface DesignResponse {
  design: BackendDesign
}
export interface BackendDesignDoc {
  id: string
  workspaceId: string
  designId: string
  title: string
  body: string
  format: 'html' | 'markdown'
  createdBy: string
  createdAt: string
  updatedAt: string
}
interface DesignDocsResponse {
  docs: BackendDesignDoc[]
}
interface DesignDocResponse {
  doc: BackendDesignDoc
}
export interface DesignAnalysisFinding {
  severity: 'low' | 'medium' | 'high'
  suite: string
  title: string
  detail: string
  impact?: string
  evidence?: string
  recommendation?: string
  componentId?: string
  connectorId?: string
  confidence?: string
}
export interface DesignAnalysisWorkflowStep {
  id: string
  label: string
  status: string
  detail: string
}
export interface DesignAIReviewPoint {
  severity: 'low' | 'medium' | 'high'
  suite: string
  title: string
  detail: string
  impact: string
  recommendation: string
  componentId?: string
  connectorId?: string
}
export interface DesignAIReview {
  status: string
  provider?: string
  model?: string
  promptVersion: string
  executiveReview?: string
  strengths?: string[]
  risks?: DesignAIReviewPoint[]
  recommendations?: DesignAIReviewPoint[]
  openQuestions?: string[]
  error?: string
}
export interface DesignAnalysisReport {
  summary: string
  score: number
  findings: DesignAnalysisFinding[]
  signals: Record<string, number>
  workflow?: DesignAnalysisWorkflowStep[]
  aiReview?: DesignAIReview
}
interface AnalysisResponse {
  analysis: DesignAnalysisReport
}
export interface AIProviderVerificationInput {
  provider: string
  model: string
  baseUrl: string
  apiKey: string
}
export interface AIProviderVerificationResponse {
  ok: boolean
  message: string
  provider?: string
  model?: string
  verifiedAt?: string
}

function apiBaseUrl() {
  if (import.meta.env.VITE_BACKEND_API_URL) return import.meta.env.VITE_BACKEND_API_URL
  const host = window.location.hostname || '127.0.0.1'
  const port = window.location.port
  if (port === '8081' || !port) return ''
  if (port.startsWith('517') || port === '4173') return `${window.location.protocol}//${host}:8081`
  if (!window.location.port) return ''
  return `http://${host}:8081`
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response
  try {
    response = await fetch(`${apiBaseUrl()}${path}`, {
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        ...(init?.headers ?? {}),
      },
      ...init,
    })
  } catch (error) {
    throw new Error(error instanceof Error ? `Backend unavailable: ${error.message}` : 'Backend unavailable')
  }

  if (!response.ok) {
    const body = await response.text().catch(() => '')
    let message = body
    try {
      const parsed = body ? (JSON.parse(body) as { error?: string }) : null
      message = parsed?.error ?? body
    } catch {
      message = body
    }
    throw new Error(`Backend request failed: ${response.status}${message ? ` - ${message}` : ''}`)
  }

  if (response.status === 204) return undefined as T
  return (await response.json()) as T
}

export function fetchProfile() {
  return request<ProfileResponse>('/api/profile')
}

export function fetchSetupStatus() {
  return request<SetupStatusResponse>('/api/setup/status')
}

export function createFirstAdmin(input: { displayName: string; email: string; password: string }) {
  return request<AuthResponse>('/api/setup/first-admin', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function setInitialAdminPassword(input: { email: string; password: string }) {
  return request<AuthResponse>('/api/setup/admin-password', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function login(input: { email: string; password: string }) {
  return request<AuthResponse>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function logout() {
  return request<void>('/api/auth/logout', {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export function fetchUsers() {
  return request<UsersResponse>('/api/users')
}

export function createAdminUser(input: { displayName: string; email: string; role: string; password?: string }) {
  return request<UserResponse>('/api/admin/users', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateAdminUser(userId: string, input: { displayName?: string; email?: string; role?: string; status?: string; password?: string }) {
  return request<UserResponse>(`/api/admin/users/${encodeURIComponent(userId)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteAdminUser(userId: string) {
  return request<void>(`/api/admin/users/${encodeURIComponent(userId)}`, {
    method: 'DELETE',
  })
}

export function fetchSignInConfig() {
  return request<SignInConfigResponse>('/api/admin/sign-in')
}

export function updateSignInConfig(input: Partial<BackendSignInConfig> & { clientSecret?: string }) {
  return request<SignInConfigResponse>('/api/admin/sign-in', {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function fetchAIProviderConfig() {
  return request<AIProviderConfigResponse>('/api/admin/ai-provider')
}

export function updateAIProviderConfig(input: Partial<BackendAIProviderConfig> & { apiKey?: string }) {
  return request<AIProviderConfigResponse>('/api/admin/ai-provider', {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function fetchMCPConfig() {
  return request<MCPConfigResponse>('/api/admin/mcp')
}

export function updateMCPConfig(input: BackendMCPConfig) {
  return request<MCPConfigResponse>('/api/admin/mcp', {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function fetchCatalogAssets(query = '') {
  const suffix = query.trim() ? `?query=${encodeURIComponent(query.trim())}` : ''
  return request<CatalogAssetsResponse>(`/api/catalog/assets${suffix}`)
}

export function createCatalogAsset(input: {
  name: string
  type: string
  owner?: string
  description?: string
  criticality?: string
  tags?: string[]
  metadata?: Record<string, unknown>
}) {
  return request<CatalogAssetResponse>('/api/catalog/assets', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateCatalogAsset(assetId: string, input: Partial<BackendCatalogAsset>) {
  return request<CatalogAssetResponse>(`/api/admin/catalog/assets/${encodeURIComponent(assetId)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteCatalogAsset(assetId: string) {
  return request<void>(`/api/admin/catalog/assets/${encodeURIComponent(assetId)}`, {
    method: 'DELETE',
  })
}

export function fetchNotifications() {
  return request<NotificationsResponse>('/api/notifications')
}

export function markNotificationRead(notificationId: string) {
  return request<void>(`/api/notifications/${encodeURIComponent(notificationId)}/read`, {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export function fetchWorkspaces() {
  return request<WorkspacesResponse>('/api/workspaces')
}

export function createWorkspace(name: string) {
  return request<WorkspaceResponse>('/api/workspaces', {
    method: 'POST',
    body: JSON.stringify({ name }),
  })
}

export function deleteWorkspace(workspaceId: string) {
  return request<void>(`/api/workspaces/${encodeURIComponent(workspaceId)}`, {
    method: 'DELETE',
  })
}

export function fetchWorkspaceAccess(workspaceId: string) {
  return request<WorkspaceAccessResponse>(`/api/workspaces/${encodeURIComponent(workspaceId)}/access`)
}

export function grantWorkspaceAccess(workspaceId: string, input: Omit<BackendWorkspaceAccess, 'workspaceId' | 'createdAt' | 'updatedAt'>) {
  return request<WorkspaceAccessGrantResponse>(`/api/workspaces/${encodeURIComponent(workspaceId)}/access`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function revokeWorkspaceAccess(workspaceId: string, userId: string) {
  return request<void>(`/api/workspaces/${encodeURIComponent(workspaceId)}/access/${encodeURIComponent(userId)}`, {
    method: 'DELETE',
  })
}

export function fetchWorkspaceDesigns(workspaceId: string) {
  return request<DesignsResponse>(`/api/workspaces/${encodeURIComponent(workspaceId)}/designs`)
}

export function createDesign(workspaceId: string, name: string, document?: unknown) {
  const body = document === undefined || document === null ? { name } : { name, document }
  return request<DesignResponse>(`/api/workspaces/${encodeURIComponent(workspaceId)}/designs`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function deleteDesign(workspaceId: string, designId: string) {
  return request<void>(`/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}`, {
    method: 'DELETE',
  })
}

export function updateDesignMetadata(workspaceId: string, designId: string, input: { name?: string; access?: string }) {
  return request<DesignResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}`,
    {
      method: 'PATCH',
      body: JSON.stringify(input),
    },
  )
}

export function fetchDesignAccess(workspaceId: string, designId: string) {
  return request<DesignAccessResponse>(`/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/access`)
}

export function grantDesignAccess(workspaceId: string, designId: string, input: Omit<BackendDesignAccess, 'workspaceId' | 'designId' | 'createdAt' | 'updatedAt'>) {
  return request<DesignAccessGrantResponse>(`/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/access`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function revokeDesignAccess(workspaceId: string, designId: string, userId: string) {
  return request<void>(`/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/access/${encodeURIComponent(userId)}`, {
    method: 'DELETE',
  })
}

export function saveDesignDocument(
  workspaceId: string,
  designId: string,
  input: { document: unknown },
) {
  return request<DesignResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/document`,
    {
      method: 'PUT',
      body: JSON.stringify(input),
    },
  )
}

export function fetchWorkspaceDesign(workspaceId: string, designId: string) {
  return request<DesignResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}`,
  )
}

export function analyzeDesign(workspaceId: string, designId: string) {
  return request<AnalysisResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/analysis`,
    {
      method: 'POST',
      body: JSON.stringify({}),
    },
  )
}

export function fetchDesignDocs(workspaceId: string, designId: string) {
  return request<DesignDocsResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/docs`,
  )
}

export function createDesignDoc(
  workspaceId: string,
  designId: string,
  input: { title: string; body?: string; format?: 'html' | 'markdown' },
) {
  return request<DesignDocResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/docs`,
    {
      method: 'POST',
      body: JSON.stringify(input),
    },
  )
}

export function updateDesignDoc(
  workspaceId: string,
  designId: string,
  docId: string,
  input: { title?: string; body?: string; format?: 'html' | 'markdown' },
) {
  return request<DesignDocResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/docs/${encodeURIComponent(docId)}`,
    {
      method: 'PATCH',
      body: JSON.stringify(input),
    },
  )
}

export function deleteDesignDoc(workspaceId: string, designId: string, docId: string) {
  return request<void>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/docs/${encodeURIComponent(docId)}`,
    {
      method: 'DELETE',
    },
  )
}

export function fetchDesignComments(workspaceId: string, designId: string) {
  return request<DesignCommentsResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/comments`,
  )
}

export function createDesignComment(
  workspaceId: string,
  designId: string,
  input: { body: string; componentId?: string; connectorId?: string },
) {
  return request<DesignCommentResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/comments`,
    {
      method: 'POST',
      body: JSON.stringify(input),
    },
  )
}

export function fetchDesignReviews(workspaceId: string, designId: string) {
  return request<DesignReviewsResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/reviews`,
  )
}

export function requestDesignReview(
  workspaceId: string,
  designId: string,
  input: { reviewerId: string; message: string },
) {
  return request<DesignReviewResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/reviews`,
    {
      method: 'POST',
      body: JSON.stringify(input),
    },
  )
}

export function updateDesignReview(
  workspaceId: string,
  designId: string,
  reviewId: string,
  input: { status: BackendDesignReviewRequest['status']; summary: string },
) {
  return request<DesignReviewResponse>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/designs/${encodeURIComponent(designId)}/reviews/${encodeURIComponent(reviewId)}`,
    {
      method: 'PATCH',
      body: JSON.stringify(input),
    },
  )
}

export function verifyAIProvider(input: AIProviderVerificationInput) {
  return request<AIProviderVerificationResponse>('/api/ai/provider/verify', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
