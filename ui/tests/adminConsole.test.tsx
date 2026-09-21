import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ComponentProps } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { AdminConsole } from '../src/components/AdminConsole'
import type {
  BackendAIProviderConfig,
  BackendCatalogAsset,
  BackendMCPConfig,
  BackendSignInConfig,
  BackendStorageStatus,
  BackendTelemetryIntegrationConfig,
  BackendUser,
} from '../src/backendApi'

const timestamp = '2026-09-21T00:00:00Z'

function userFixture(patch: Partial<BackendUser> = {}): BackendUser {
  return {
    id: 'user-1',
    displayName: 'Existing User',
    email: 'existing@example.com',
    passwordSet: true,
    role: 'member',
    status: 'active',
    lastSeenAt: timestamp,
    createdAt: timestamp,
    updatedAt: timestamp,
    ...patch,
  }
}

function signInFixture(patch: Partial<BackendSignInConfig> = {}): BackendSignInConfig {
  return {
    localPasswordEnabled: true,
    ssoEnabled: false,
    provider: 'okta',
    oktaDomain: '',
    issuer: '',
    clientId: '',
    clientSecretSet: false,
    redirectUri: 'https://stratum.example.com/api/auth/oidc/callback',
    postLogoutRedirectUri: 'https://stratum.example.com',
    scopes: 'openid profile email groups',
    groupsClaim: 'groups',
    adminGroup: '',
    reviewerGroup: '',
    jitProvisioning: true,
    updatedAt: timestamp,
    ...patch,
  }
}

function aiFixture(patch: Partial<BackendAIProviderConfig> = {}): BackendAIProviderConfig {
  return { enabled: false, provider: 'openai', model: 'gpt-4.1', baseUrl: '', apiKeySet: false, updatedAt: timestamp, ...patch }
}

function mcpFixture(patch: Partial<BackendMCPConfig> = {}): BackendMCPConfig {
  return {
    enabled: false,
    endpointPath: '/mcp',
    readCatalog: true,
    readDesigns: true,
    createDraftDesign: false,
    runAnalysis: false,
    fetchImpactReport: false,
    requireAdminConsent: true,
    updatedAt: timestamp,
    ...patch,
  }
}

function telemetryFixture(patch: Partial<BackendTelemetryIntegrationConfig> = {}): BackendTelemetryIntegrationConfig {
  return {
    enabled: false,
    provider: 'prometheus',
    displayName: 'Production Prometheus',
    baseUrl: 'https://prometheus.example.com',
    authMode: 'none',
    secretSet: false,
    customHeaderName: '',
    queryWindow: '5m',
    requestTotalMetric: 'http_requests_total',
    requestFailedMetric: 'http_requests_failed_total',
    serverLatencyMetric: 'http_server_duration',
    clientLatencyMetric: 'http_client_duration',
    filters: {
      namespaces: [],
      services: [],
      excludeServices: [],
      excludeEndpoints: [],
      minimumRequestsPerSec: 0,
      includeExternal: false,
      includeDatabaseClients: false,
    },
    updatedAt: timestamp,
    ...patch,
  }
}

function catalogFixture(patch: Partial<BackendCatalogAsset> = {}): BackendCatalogAsset {
  return {
    id: 'asset-1',
    name: 'Payments API',
    normalizedName: 'payments api',
    kind: 'component',
    type: 'compute.service',
    owner: 'Payments',
    description: '',
    criticality: 'high',
    status: 'active',
    aliases: [],
    replacementAssetId: '',
    updateMessage: '',
    tags: [],
    createdBy: 'admin-1',
    createdAt: timestamp,
    updatedAt: timestamp,
    usedInDesignCount: 0,
    ...patch,
  }
}

function adminProps(): ComponentProps<typeof AdminConsole> {
  return {
    users: [],
    activeUserId: 'admin-1',
    workspaces: [],
    storageStatus: null,
    signInConfig: null,
    aiProviderConfig: null,
    mcpConfig: null,
    telemetryConfig: null,
    catalogAssets: [],
    error: null,
    onAddUser: vi.fn().mockResolvedValue(undefined),
    onSaveUser: vi.fn().mockResolvedValue(undefined),
    onGeneratePasswordResetLink: vi.fn().mockResolvedValue({ resetLink: '', expiresAt: '' }),
    onDeleteUser: vi.fn(),
    onSaveSignIn: vi.fn(),
    onSaveAIProvider: vi.fn().mockResolvedValue(true),
    onSaveMCP: vi.fn(),
    onSaveTelemetry: vi.fn(),
    onAddCatalogAsset: vi.fn().mockResolvedValue({}),
    onSaveCatalogAsset: vi.fn().mockResolvedValue(undefined),
    onDeleteCatalogAsset: vi.fn().mockResolvedValue(undefined),
    onRefreshStorage: vi.fn().mockResolvedValue({}),
    onTestDatabase: vi.fn().mockResolvedValue({ ok: true, storage: {} }),
    onConfigureDatabase: vi.fn().mockResolvedValue({}),
    onMigrateStorageUsers: vi.fn().mockResolvedValue({ storage: {}, migratedUsers: 0 }),
    activeSection: 'overview',
    onSelectSection: vi.fn(),
  }
}

describe('AdminConsole', () => {
	it('shows backend errors inside the active admin content as a wrapping alert', () => {
		const props = adminProps()
		props.activeSection = 'ai'
		props.aiProviderConfig = aiFixture()
		props.error = 'AI provider was not saved. request contains unsupported field "apiKeySet"'
		render(<AdminConsole {...props} />)

		const alert = screen.getByRole('alert')
		expect(alert.textContent).toContain('request contains unsupported field "apiKeySet"')
		expect(alert.parentElement?.classList.contains('admin-section-content')).toBe(true)
	})

  it('renders the governance dashboard and exposes every administration section', async () => {
    const user = userEvent.setup()
    const props = adminProps()
    render(<AdminConsole {...props} />)

    expect(screen.getByText('Complete identity provider')).toBeTruthy()
    expect(screen.getByText('Configure analysis AI')).toBeTruthy()
    expect(screen.getByText('Review MCP readiness')).toBeTruthy()

    for (const section of [
      'Dashboard',
      'Users and roles',
      'Access center',
      'Workspaces',
      'Enterprise catalog',
      'Sign-in and SSO',
      'AI suite',
      'Integrations',
      'Security and audit',
      'System settings',
    ]) {
      expect(screen.getByRole('button', { name: new RegExp(section, 'i') })).toBeTruthy()
    }

    await user.click(screen.getByRole('button', { name: /Users and roles/i }))
    expect(props.onSelectSection).toHaveBeenCalledWith('users')
  })

  it('guides administrators through Google AI Studio configuration', async () => {
    const user = userEvent.setup()
    const props = adminProps()
    props.activeSection = 'ai'
    props.aiProviderConfig = aiFixture({ apiKeySet: true })
    render(<AdminConsole {...props} />)

    await user.click(screen.getByRole('button', { name: 'Change configuration' }))
    await user.selectOptions(screen.getByLabelText('Provider'), 'google')

    expect((screen.getByLabelText('Model') as HTMLInputElement).value).toBe('gemini-2.5-flash')
    expect((screen.getByLabelText('API key') as HTMLInputElement).value).toBe('')
    const studioLink = screen.getByRole('link', { name: 'Google AI Studio' }) as HTMLAnchorElement
    expect(studioLink.href).toBe('https://aistudio.google.com/app/apikey')
    expect(screen.getByText(/never returns the saved key to the browser/i)).toBeTruthy()

    await user.click(screen.getByRole('checkbox', { name: /Use AI for analysis synthesis/i }))
    await user.type(screen.getByLabelText('API key'), 'google-secret')
    await user.click(screen.getByRole('button', { name: 'Save and verify AI provider' }))
    expect(props.onSaveAIProvider).toHaveBeenCalledWith(expect.objectContaining({
      enabled: true,
      provider: 'google',
      model: 'gemini-2.5-flash',
      apiKey: 'google-secret',
      apiKeySet: false,
    }))
  })

	it('shows pending and verified feedback for a successful AI provider save', async () => {
		const user = userEvent.setup()
		const props = adminProps()
		let finishSave: (saved: boolean) => void = () => undefined
		props.activeSection = 'ai'
		props.aiProviderConfig = aiFixture({ enabled: true, apiKeySet: true })
		props.onSaveAIProvider = vi.fn(() => new Promise<boolean>((resolve) => { finishSave = resolve }))
		render(<AdminConsole {...props} />)

		await user.click(screen.getByRole('button', { name: 'Change configuration' }))
		await user.click(screen.getByRole('button', { name: 'Save and verify AI provider' }))
		const pendingButton = screen.getByRole('button', { name: /Saving and verifying/i }) as HTMLButtonElement
		expect(pendingButton.disabled).toBe(true)
		expect(screen.queryByRole('status')).toBeNull()

		finishSave(true)
		const confirmation = await screen.findByRole('status')
		expect(confirmation.textContent).toContain('AI provider saved and verified')
		expect(screen.getByText('gpt-4.1')).toBeTruthy()
		expect(screen.queryByLabelText('API key (stored)')).toBeNull()
	})

	it('does not show verified feedback when the backend rejects an AI provider save', async () => {
		const user = userEvent.setup()
		const props = adminProps()
		props.activeSection = 'ai'
		props.aiProviderConfig = aiFixture({ enabled: true, apiKeySet: true })
		props.onSaveAIProvider = vi.fn().mockResolvedValue(false)
		render(<AdminConsole {...props} />)

		await user.click(screen.getByRole('button', { name: 'Change configuration' }))
		await user.click(screen.getByRole('button', { name: 'Save and verify AI provider' }))
		await waitFor(() => expect(screen.getByRole('button', { name: 'Save and verify AI provider' })).toBeTruthy())
		expect(screen.queryByRole('status')).toBeNull()
	})

	it('shows a configured provider summary before exposing reconfiguration fields', async () => {
		const user = userEvent.setup()
		const props = adminProps()
		props.activeSection = 'ai'
		props.aiProviderConfig = aiFixture({ enabled: true, provider: 'google', model: 'gemini-2.5-flash', apiKeySet: true })
		render(<AdminConsole {...props} />)

		expect(screen.getByText('AI provider active')).toBeTruthy()
		expect(screen.getByText('Google AI Studio (Gemini)')).toBeTruthy()
		expect(screen.queryByLabelText('API key (stored)')).toBeNull()

		await user.click(screen.getByRole('button', { name: 'Change configuration' }))
		expect(screen.getByLabelText('API key (stored)')).toBeTruthy()
	})

  it('normalizes new users and blocks duplicate email addresses', async () => {
    const user = userEvent.setup()
    const props = adminProps()
    props.activeSection = 'users'
    props.users = [userFixture()]
    render(<AdminConsole {...props} />)

    await user.type(screen.getByLabelText('Name'), 'New Architect')
    await user.type(screen.getByLabelText('Email'), ' EXISTING@example.com ')
    await user.click(screen.getByRole('button', { name: 'Add user' }))
    expect(screen.getByText('A user with this email already exists.')).toBeTruthy()
    expect(props.onAddUser).not.toHaveBeenCalled()

    await user.clear(screen.getByLabelText('Email'))
    await user.type(screen.getByLabelText('Email'), ' ARCHITECT@EXAMPLE.COM ')
    await user.selectOptions(screen.getByLabelText('Role'), 'architect')
    await user.click(screen.getByRole('button', { name: 'Add user' }))
    await waitFor(() => expect(props.onAddUser).toHaveBeenCalledWith({
      displayName: 'New Architect',
      email: 'architect@example.com',
      role: 'architect',
    }))
  })

  it('protects the active admin while allowing controlled user edits and deletion', async () => {
    const user = userEvent.setup()
    const props = adminProps()
    props.activeSection = 'users'
    props.users = [
      userFixture({ id: 'admin-1', displayName: 'Current Admin', email: 'admin@example.com', role: 'admin' }),
      userFixture({ id: 'user-2', displayName: 'Reviewer', email: 'reviewer@example.com', role: 'reviewer' }),
    ]
    render(<AdminConsole {...props} />)

    const deleteButtons = screen.getAllByRole('button', { name: 'Delete' }) as HTMLButtonElement[]
    expect(deleteButtons[0].disabled).toBe(true)
    expect(deleteButtons[1].disabled).toBe(false)
    await user.click(deleteButtons[1])
    expect(props.onDeleteUser).toHaveBeenCalledWith('user-2')

    const editButtons = screen.getAllByRole('button', { name: 'Edit' })
    await user.click(editButtons[1])
    await user.selectOptions(screen.getAllByLabelText('Role')[1], 'architect')
    await user.selectOptions(screen.getAllByLabelText('Status')[0], 'disabled')
    await user.clear(screen.getAllByLabelText('Email')[1])
    await user.type(screen.getAllByLabelText('Email')[1], ' REVIEWER.NEW@EXAMPLE.COM ')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(props.onSaveUser).toHaveBeenCalledWith(expect.objectContaining({
      id: 'user-2',
      email: 'reviewer.new@example.com',
      role: 'architect',
      status: 'disabled',
    })))
  })

  it('submits customized sign-in and role-mapping settings', async () => {
    const user = userEvent.setup()
    const props = adminProps()
    props.activeSection = 'identity'
    props.signInConfig = signInFixture()
    render(<AdminConsole {...props} />)

    await user.click(screen.getByRole('checkbox', { name: /Okta SSO/i }))
    await user.type(screen.getByLabelText('Okta domain'), 'acme.okta.com')
    await user.type(screen.getByLabelText('Issuer'), 'https://acme.okta.com/oauth2/default')
    await user.type(screen.getByLabelText('Client ID'), 'client-123')
    await user.type(screen.getByLabelText('Admin group'), 'stratum-admins')
    await user.click(screen.getByRole('button', { name: 'Save sign-in settings' }))

    expect(props.onSaveSignIn).toHaveBeenCalledWith(expect.objectContaining({
      ssoEnabled: true,
      oktaDomain: 'acme.okta.com',
      issuer: 'https://acme.okta.com/oauth2/default',
      clientId: 'client-123',
      adminGroup: 'stratum-admins',
      jitProvisioning: true,
    }))
  })

  it('submits MCP capabilities and keeps admin consent explicit', async () => {
    const user = userEvent.setup()
    const props = adminProps()
    props.activeSection = 'integrations'
    props.mcpConfig = mcpFixture()
    props.telemetryConfig = telemetryFixture()
    render(<AdminConsole {...props} />)

    await user.click(screen.getByRole('button', { name: /MCP readiness/i }))
    await user.click(screen.getByRole('checkbox', { name: /Enable MCP configuration/i }))
    await user.click(screen.getByRole('checkbox', { name: /Run analysis/i }))
    await user.click(screen.getByRole('button', { name: 'Save MCP settings' }))

    expect(props.onSaveMCP).toHaveBeenCalledWith(expect.objectContaining({
      enabled: true,
      endpointPath: '/mcp',
      readCatalog: true,
      runAnalysis: true,
      requireAdminConsent: true,
    }))
  })

  it('parses telemetry filters and submits security-sensitive connection settings', async () => {
    const user = userEvent.setup()
    const props = adminProps()
    props.activeSection = 'integrations'
    props.mcpConfig = mcpFixture()
    props.telemetryConfig = telemetryFixture()
    render(<AdminConsole {...props} />)

    await user.click(screen.getByRole('checkbox', { name: /Enable observed architecture/i }))
    await user.selectOptions(screen.getByLabelText('Auth mode'), 'custom_header')
    await user.type(screen.getByLabelText('Custom header'), 'X-Prometheus-Key')
    await user.type(screen.getByLabelText('Secret'), 'telemetry-secret')
    fireEvent.change(screen.getByLabelText('Include namespaces'), { target: { value: 'payments, platform\nedge' } })
    await user.click(screen.getByRole('checkbox', { name: /Include external systems/i }))
    await user.click(screen.getByRole('button', { name: 'Save telemetry integration' }))

    expect(props.onSaveTelemetry).toHaveBeenCalledWith(expect.objectContaining({
      enabled: true,
      authMode: 'custom_header',
      customHeaderName: 'X-Prometheus-Key',
      secret: 'telemetry-secret',
      filters: expect.objectContaining({ namespaces: ['payments', 'platform', 'edge'], includeExternal: true }),
    }))
  })

  it('validates storage configuration and confirms user migration', async () => {
    const user = userEvent.setup()
    const props = adminProps()
    const storage: BackendStorageStatus = {
      mode: 'stateless',
      stateless: true,
      databaseConfigured: true,
      databaseConnected: true,
      databasePending: true,
      canMigrateUsers: true,
      cacheUserCount: 2,
      databaseUserCount: 0,
    }
    props.activeSection = 'settings'
    props.storageStatus = storage
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<AdminConsole {...props} />)

    await user.click(screen.getByRole('button', { name: 'Test database' }))
    expect(screen.getByText('Database URL is required.')).toBeTruthy()
    expect(props.onTestDatabase).not.toHaveBeenCalled()

    await user.type(screen.getByLabelText('Database URL'), 'postgres://db.example.com/stratum')
    await user.click(screen.getByRole('button', { name: 'Test database' }))
    await waitFor(() => expect(props.onTestDatabase).toHaveBeenCalledWith('postgres://db.example.com/stratum'))
    expect(screen.getByText('Database connection test passed.')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Migrate users to database' }))
    await waitFor(() => expect(props.onMigrateStorageUsers).toHaveBeenCalled())
    expect(screen.getByText('Migrated 0 users and switched to database storage.')).toBeTruthy()
  })

  it('prevents duplicate catalog assets and submits normalized aliases', async () => {
    const user = userEvent.setup()
    const props = adminProps()
    props.activeSection = 'catalog'
    props.catalogAssets = [catalogFixture()]
    render(<AdminConsole {...props} />)

    await user.type(screen.getByLabelText('Asset name'), ' PAYMENTS API ')
    expect(screen.getByText((_content, element) => element?.classList.contains('catalog-duplicate-warning') ?? false)).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Add catalog asset' }) as HTMLButtonElement).disabled).toBe(true)

    await user.clear(screen.getByLabelText('Asset name'))
    await user.type(screen.getByLabelText('Asset name'), 'Orders API')
    await user.type(screen.getByLabelText('Aliases'), 'orders, order service')
    await user.selectOptions(screen.getByLabelText('Criticality'), 'critical')
    await user.click(screen.getByRole('button', { name: 'Add catalog asset' }))
    await waitFor(() => expect(props.onAddCatalogAsset).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Orders API',
      aliases: ['orders', 'order service'],
      criticality: 'critical',
    })))
  })
})
