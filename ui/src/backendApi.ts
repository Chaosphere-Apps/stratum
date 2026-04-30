import type { BackendDesign, BackendWorkspace } from './backendSync'

export interface BackendProfile {
  id: string
  displayName: string
  role: string
}

interface ProfileResponse extends BackendProfile {}
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
}
export interface DesignAnalysisReport {
  summary: string
  score: number
  findings: DesignAnalysisFinding[]
  signals: Record<string, number>
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
  return `http://${host}:8081`
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response
  try {
    response = await fetch(`${apiBaseUrl()}${path}`, {
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
    throw new Error(`Backend request failed: ${response.status}${body ? ` - ${body}` : ''}`)
  }

  if (response.status === 204) return undefined as T
  return (await response.json()) as T
}

export function fetchProfile() {
  return request<ProfileResponse>('/api/profile')
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

export function verifyAIProvider(input: AIProviderVerificationInput) {
  return request<AIProviderVerificationResponse>('/api/ai/provider/verify', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
