import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../src/backendApi'

type Contract = {
  name: string
  run: () => Promise<unknown>
  path: string
  method?: string
}

describe('backend API contracts', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockResolvedValue(new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => vi.unstubAllGlobals())

  const contracts: Contract[] = [
    { name: 'profile', run: api.fetchProfile, path: '/api/profile' },
    { name: 'setup status', run: api.fetchSetupStatus, path: '/api/setup/status' },
    { name: 'storage status', run: api.fetchStorageStatus, path: '/api/storage/status' },
    { name: 'first admin', run: () => api.createFirstAdmin({ displayName: 'Admin', email: 'admin@test', password: 'password' }), path: '/api/setup/first-admin', method: 'POST' },
    { name: 'initial password', run: () => api.setInitialAdminPassword({ email: 'admin@test', password: 'password' }), path: '/api/setup/admin-password', method: 'POST' },
    { name: 'login', run: () => api.login({ email: 'admin@test', password: 'password' }), path: '/api/auth/login', method: 'POST' },
    { name: 'reset password', run: () => api.resetPassword({ token: 'token', password: 'password' }), path: '/api/auth/password-reset', method: 'POST' },
    { name: 'logout', run: api.logout, path: '/api/auth/logout', method: 'POST' },
    { name: 'users pagination', run: () => api.fetchUsers({ query: ' Ada ', cursor: 'next', limit: 25 }), path: '/api/users?query=Ada&cursor=next&limit=25' },
    { name: 'create user', run: () => api.createAdminUser({ displayName: 'Ada', email: 'ada@test', role: 'member' }), path: '/api/admin/users', method: 'POST' },
    { name: 'update user', run: () => api.updateAdminUser('user / 1', { status: 'active' }), path: '/api/admin/users/user%20%2F%201', method: 'PATCH' },
    { name: 'password reset link', run: () => api.createPasswordResetLink('user/1'), path: '/api/admin/users/user%2F1/password-reset-link', method: 'POST' },
    { name: 'delete user', run: () => api.deleteAdminUser('user/1'), path: '/api/admin/users/user%2F1', method: 'DELETE' },
    { name: 'sign in config', run: api.fetchSignInConfig, path: '/api/admin/sign-in' },
    { name: 'update sign in', run: () => api.updateSignInConfig({ ssoEnabled: true }), path: '/api/admin/sign-in', method: 'PATCH' },
    { name: 'AI config', run: api.fetchAIProviderConfig, path: '/api/admin/ai-provider' },
    { name: 'MCP config', run: api.fetchMCPConfig, path: '/api/admin/mcp' },
    { name: 'telemetry config', run: api.fetchTelemetryIntegrationConfig, path: '/api/admin/telemetry' },
    { name: 'storage admin', run: api.fetchAdminStorageStatus, path: '/api/admin/storage' },
    { name: 'test database', run: () => api.testStorageDatabase('postgres://test'), path: '/api/admin/storage/test', method: 'POST' },
    { name: 'configure database', run: () => api.configureStorageDatabase('postgres://test'), path: '/api/admin/storage/database', method: 'PATCH' },
    { name: 'migrate users', run: api.migrateStorageUsers, path: '/api/admin/storage/migrate-users', method: 'POST' },
    { name: 'catalog pagination', run: () => api.fetchCatalogAssets(' cache ', { limit: 10 }), path: '/api/catalog/assets?query=cache&limit=10' },
    { name: 'create catalog asset', run: () => api.createCatalogAsset({ name: 'Cache', type: 'data.redis' }), path: '/api/catalog/assets', method: 'POST' },
    { name: 'delete catalog asset', run: () => api.deleteCatalogAsset('asset/1'), path: '/api/admin/catalog/assets/asset%2F1', method: 'DELETE' },
    { name: 'notifications', run: api.fetchNotifications, path: '/api/notifications' },
    { name: 'mark notification read', run: () => api.markNotificationRead('note/1'), path: '/api/notifications/note%2F1/read', method: 'POST' },
    { name: 'workspaces', run: () => api.fetchWorkspaces({ limit: 24 }), path: '/api/workspaces?limit=24' },
    { name: 'share principals', run: api.fetchWorkspaceSharePrincipals, path: '/api/workspace-share-principals' },
    { name: 'create workspace', run: () => api.createWorkspace({ name: 'Payments', shares: [] }), path: '/api/workspaces', method: 'POST' },
    { name: 'workspace access', run: () => api.fetchWorkspaceAccess('space/1'), path: '/api/workspaces/space%2F1/access' },
    { name: 'grant workspace access', run: () => api.grantWorkspaceAccess('space', { userId: 'user', canRead: true, canCreateDesign: false, canManage: false }), path: '/api/workspaces/space/access', method: 'POST' },
    { name: 'groups', run: api.fetchAccessGroups, path: '/api/admin/access/groups' },
    { name: 'create group', run: () => api.createAccessGroup({ name: 'Reviewers' }), path: '/api/admin/access/groups', method: 'POST' },
    { name: 'group members', run: () => api.fetchAccessGroupMembers('group/1'), path: '/api/admin/access/groups/group%2F1/members' },
    { name: 'replace group members', run: () => api.replaceAccessGroupMembers('group', ['user']), path: '/api/admin/access/groups/group/members', method: 'PUT' },
    { name: 'workspace group access', run: () => api.fetchWorkspaceGroupAccess('space'), path: '/api/workspaces/space/group-access' },
    { name: 'designs pagination', run: () => api.fetchWorkspaceDesigns('space', { query: 'pay', limit: 5 }), path: '/api/workspaces/space/designs?query=pay&limit=5' },
    { name: 'create design', run: () => api.createDesign('space', 'Payments'), path: '/api/workspaces/space/designs', method: 'POST' },
    { name: 'get design', run: () => api.fetchWorkspaceDesign('space', 'design'), path: '/api/workspaces/space/designs/design' },
    { name: 'save document', run: () => api.saveDesignDocument('space', 'design', { document: { title: 'Pay' }, baseRevision: 'sha256:abc' }), path: '/api/workspaces/space/designs/design/document', method: 'PUT' },
    { name: 'versions', run: () => api.fetchDesignVersions('space', 'design', { limit: 10 }), path: '/api/workspaces/space/designs/design/versions?limit=10' },
    { name: 'create version', run: () => api.createDesignVersion('space', 'design', 'remark'), path: '/api/workspaces/space/designs/design/versions', method: 'POST' },
    { name: 'update version', run: () => api.updateDesignVersionStatus('space', 'design', 'version', 'reviewed'), path: '/api/workspaces/space/designs/design/versions/version', method: 'PATCH' },
    { name: 'analyze design', run: () => api.analyzeDesign('space', 'design'), path: '/api/workspaces/space/designs/design/analysis', method: 'POST' },
    { name: 'AI conversations', run: () => api.fetchAIConversations('space', 'design'), path: '/api/workspaces/space/designs/design/ai/conversations' },
    { name: 'create AI conversation', run: () => api.createAIConversation('space', 'design', { versionId: 'version' }), path: '/api/workspaces/space/designs/design/ai/conversations', method: 'POST' },
    { name: 'update AI conversation access', run: () => api.updateAIConversationAccess('space', 'design', 'chat/1', 'read_write'), path: '/api/workspaces/space/designs/design/ai/conversations/chat%2F1', method: 'PATCH' },
    { name: 'AI messages', run: () => api.fetchAIMessages('space', 'design', 'chat/1'), path: '/api/workspaces/space/designs/design/ai/conversations/chat%2F1/messages' },
    { name: 'send AI message', run: () => api.sendAIMessage('space', 'design', 'chat/1', 'Review this'), path: '/api/workspaces/space/designs/design/ai/conversations/chat%2F1/messages', method: 'POST' },
    { name: 'design docs', run: () => api.fetchDesignDocs('space', 'design'), path: '/api/workspaces/space/designs/design/docs' },
    { name: 'create doc', run: () => api.createDesignDoc('space', 'design', { title: 'ADR' }), path: '/api/workspaces/space/designs/design/docs', method: 'POST' },
    { name: 'comments', run: () => api.fetchDesignComments('space', 'design', { limit: 5 }), path: '/api/workspaces/space/designs/design/comments?limit=5' },
    { name: 'create comment', run: () => api.createDesignComment('space', 'design', { body: 'Review' }), path: '/api/workspaces/space/designs/design/comments', method: 'POST' },
    { name: 'reviews', run: () => api.fetchDesignReviews('space', 'design'), path: '/api/workspaces/space/designs/design/reviews' },
    { name: 'request review', run: () => api.requestDesignReview('space', 'design', { versionId: 'v1', reviewerIds: ['reviewer'], message: '' }), path: '/api/workspaces/space/designs/design/reviews', method: 'POST' },
    { name: 'verify AI provider', run: () => api.verifyAIProvider({ provider: 'openai', model: 'test', baseUrl: 'https://example.test', apiKey: 'fake' }), path: '/api/ai/provider/verify', method: 'POST' },
  ]

  it.each(contracts)('uses the expected $name contract', async ({ run, path, method = 'GET' }) => {
    await run()
    expect(fetchMock).toHaveBeenCalledOnce()
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe(path)
    expect(init.credentials).toBe('include')
    expect(init.method ?? 'GET').toBe(method)
    expect(init.headers).toMatchObject({ 'Content-Type': 'application/json' })
  })

  it('maps structured, plain-text, empty, and network failures', async () => {
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ error: 'invalid grant' }), { status: 400 }))
    await expect(api.fetchProfile()).rejects.toThrow('Backend request failed: 400 - invalid grant')

    fetchMock.mockResolvedValueOnce(new Response('upstream failed', { status: 502 }))
    await expect(api.fetchProfile()).rejects.toThrow('Backend request failed: 502 - upstream failed')

    fetchMock.mockResolvedValueOnce(new Response(null, { status: 500 }))
    await expect(api.fetchProfile()).rejects.toThrow('Backend request failed: 500')

    fetchMock.mockRejectedValueOnce(new Error('connection refused'))
    await expect(api.fetchProfile()).rejects.toThrow('Backend unavailable: connection refused')
  })

  it('sends only writable AI provider fields and never echoes response metadata', async () => {
	await api.updateAIProviderConfig({
		enabled: true,
		provider: 'google',
		model: 'gemini-2.5-flash',
		baseUrl: '',
		apiKey: 'new-secret',
		apiKeySet: true,
		verifiedAt: '2026-09-21T00:00:00Z',
		updatedAt: '2026-09-21T00:00:00Z',
	})

	const [, init] = fetchMock.mock.calls[0]
	expect(JSON.parse(init.body as string)).toEqual({
		enabled: true,
		provider: 'google',
		model: 'gemini-2.5-flash',
		baseUrl: '',
		apiKey: 'new-secret',
	})
  })

  it('handles empty success responses', async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))
    await expect(api.deleteWorkspace('space')).resolves.toBeUndefined()
  })

  it('returns the current server document for an optimistic concurrency conflict', async () => {
    const current = {
      id: 'design',
      workspaceId: 'space',
      name: 'Payments',
      access: 'private',
      title: 'Payments',
      document: { id: 'design', title: 'Payments', components: [], connectors: [], journeys: [] },
      documentRevision: 'sha256:current',
      updatedAt: '2026-08-30T10:00:00Z',
    }
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ error: 'design was updated by another editor', design: current }), {
      status: 409,
      headers: { 'Content-Type': 'application/json' },
    }))

    await expect(api.saveDesignDocument('space', 'design', {
      document: current.document,
      baseRevision: 'sha256:stale',
    })).rejects.toMatchObject({
      name: 'BackendDesignConflictError',
      design: current,
    })
  })
})
