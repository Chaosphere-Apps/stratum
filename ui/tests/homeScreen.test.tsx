import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createEmptyDesign } from '../src/designModel'
import type { BackendDesign, BackendWorkspace } from '../src/backendSync'

const { fetchWorkspaceSharePrincipals } = vi.hoisted(() => ({ fetchWorkspaceSharePrincipals: vi.fn() }))

vi.mock('../src/backendApi', () => ({ fetchWorkspaceSharePrincipals }))

import { HomeScreen } from '../src/components/HomeScreen'

const workspaces: BackendWorkspace[] = [
  { id: 'guest-workspace', name: 'Guest Workspace', effectiveAccess: 'read' },
  { id: 'workspace-payments', name: 'Payments', effectiveAccess: 'edit' },
]

const design: BackendDesign = {
  id: 'design-payments',
  workspaceId: 'workspace-payments',
  name: 'Payment processing',
  access: 'private',
  title: 'Payment processing',
  document: {
    ...createEmptyDesign({ id: 'design-payments', title: 'Payment processing' }),
    components: [
      { id: 'api', type: 'compute.service', name: 'API', position: { x: 0, y: 0 }, metadata: {} },
      { id: 'db', type: 'data.sql_database', name: 'Database', position: { x: 100, y: 0 }, metadata: {} },
    ],
    connectors: [
      { id: 'api-db', sourceComponentId: 'api', targetComponentId: 'db', type: 'synchronous', label: 'query' },
    ],
    journeys: [{ id: 'checkout', title: 'Checkout', summary: '', steps: [] }],
  },
  versionNumber: 3,
  updatedAt: '2026-07-10T12:00:00Z',
  effectiveAccess: 'edit',
}

function renderHome(overrides: Partial<React.ComponentProps<typeof HomeScreen>> = {}) {
  const props: React.ComponentProps<typeof HomeScreen> = {
    workspaces,
    designs: [design],
    activeWorkspaceId: 'workspace-payments',
    workspaceSearch: '',
    designSearch: '',
    workspacePage: { limit: 20, hasMore: true, nextCursor: 'workspace-next' },
    designPage: { limit: 20, hasMore: true, nextCursor: 'design-next' },
    loadingMore: null,
    error: null,
    action: null,
    onWorkspaceSearchChange: vi.fn(),
    onDesignSearchChange: vi.fn(),
    onSelectWorkspace: vi.fn(),
    onCreateWorkspace: vi.fn().mockResolvedValue(true),
    onDeleteWorkspace: vi.fn(),
    onOpenDesign: vi.fn(),
    onCloneDesign: vi.fn(),
    onDeleteDesign: vi.fn(),
    onNewDesign: vi.fn(),
    onGenerate: vi.fn(),
    onLoadMoreWorkspaces: vi.fn(),
    onLoadMoreDesigns: vi.fn(),
    onRefresh: vi.fn(),
    ...overrides,
  }
  return { ...render(<HomeScreen {...props} />), props }
}

describe('HomeScreen', () => {
  beforeEach(() => {
    fetchWorkspaceSharePrincipals.mockReset()
    fetchWorkspaceSharePrincipals.mockResolvedValue({
      principals: [
        { type: 'user', id: 'user-reviewer', name: 'Riya Reviewer', description: 'riya@example.com' },
        { type: 'group', id: 'group-architecture', name: 'Architecture Guild', description: 'Okta group' },
      ],
    })
  })

  it('loads the component module', () => {
    expect(HomeScreen).toBeTypeOf('function')
  })

  it('renders the workspace baseline', () => {
    renderHome({ workspacePage: null, designPage: { limit: 20, hasMore: false } })
    expect(screen.getByText('2 available')).toBeTruthy()
  })

  it('supports workspace discovery, design discovery, and card actions', async () => {
    const user = userEvent.setup()
    const { props } = renderHome()

    expect(screen.getByRole('heading', { name: 'Make system design a shared source of truth.' })).toBeTruthy()
    expect(screen.getByText('2 available')).toBeTruthy()
    expect(screen.getByText('components')).toBeTruthy()
    expect(screen.getByText(/connections/)).toBeTruthy()
    expect(screen.getAllByText(/journeys/)).toHaveLength(2)
    expect(screen.getByText('v3')).toBeTruthy()

    await user.type(screen.getByPlaceholderText('Search designs and workspaces...'), 'pay')
    expect(props.onWorkspaceSearchChange).toHaveBeenLastCalledWith('y')
    expect(props.onDesignSearchChange).toHaveBeenLastCalledWith('y')
    expect(screen.getByText('Can edit')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: /Guest WorkspaceDefault workspace/i }))
    expect(props.onSelectWorkspace).toHaveBeenCalledWith('guest-workspace')
    await user.click(screen.getByRole('button', { name: /Payment processingUpdated/i }))
    expect(props.onOpenDesign).toHaveBeenCalledWith(design)
    await user.click(screen.getByRole('button', { name: 'Clone Payment processing' }))
    expect(props.onCloneDesign).toHaveBeenCalledWith(design)
    await user.click(screen.getByRole('button', { name: 'Delete Payment processing' }))
    expect(props.onDeleteDesign).toHaveBeenCalledWith(design)
    await user.click(screen.getByRole('button', { name: 'New design' }))
    expect(props.onNewDesign).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: 'Load more workspaces' }))
    expect(props.onLoadMoreWorkspaces).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: 'Load more designs' }))
    expect(props.onLoadMoreDesigns).toHaveBeenCalledTimes(1)
  })

  it('creates a private workspace with optional user and group shares', async () => {
    const user = userEvent.setup()
    const onCreateWorkspace = vi.fn().mockResolvedValue(true)
    renderHome({ onCreateWorkspace })

    await user.click(screen.getByRole('button', { name: 'New workspace' }))
    const dialog = await screen.findByRole('dialog', { name: 'Create a focused space for a system' })
    expect(within(dialog).getByText('Private by default')).toBeTruthy()
    expect(fetchWorkspaceSharePrincipals).toHaveBeenCalledTimes(1)

    await user.type(within(dialog).getByPlaceholderText('e.g. Payments platform'), 'Risk platform')
    const shareSearch = within(dialog).getByPlaceholderText('Find a person or group')
    await user.type(shareSearch, 'riya')
    await user.click(await within(dialog).findByRole('button', { name: /Riya Reviewer/i }))
    await user.selectOptions(within(dialog).getByRole('combobox', { name: 'Access for Riya Reviewer' }), 'contributor')

    await user.type(shareSearch, 'architecture')
    await user.click(await within(dialog).findByRole('button', { name: /Architecture Guild/i }))
    expect(within(dialog).getByText('2 selected')).toBeTruthy()

    await user.click(within(dialog).getByRole('button', { name: 'Create workspace' }))
    expect(onCreateWorkspace).toHaveBeenCalledWith({
      name: 'Risk platform',
      shares: [
        { principalType: 'user', principalId: 'user-reviewer', accessLevel: 'contributor' },
        { principalType: 'group', principalId: 'group-architecture', accessLevel: 'viewer' },
      ],
    })
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('keeps the workspace dialog open when creation fails and allows shares to be removed', async () => {
    const user = userEvent.setup()
    const onCreateWorkspace = vi.fn().mockResolvedValue(false)
    renderHome({ onCreateWorkspace })

    await user.click(screen.getByRole('button', { name: 'New workspace' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByPlaceholderText('e.g. Payments platform'), 'Platform')
    await user.type(within(dialog).getByPlaceholderText('Find a person or group'), 'riya')
    await user.click(await within(dialog).findByRole('button', { name: /Riya Reviewer/i }))
    await user.click(within(dialog).getByRole('button', { name: 'Remove Riya Reviewer' }))
    expect(within(dialog).getByText('0 selected')).toBeTruthy()
    await user.click(within(dialog).getByRole('button', { name: 'Create workspace' }))
    expect(onCreateWorkspace).toHaveBeenCalled()
    expect(screen.getByRole('dialog')).toBeTruthy()
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('renders directory failures, empty searches, loading states, and retry behavior', async () => {
    const user = userEvent.setup()
    fetchWorkspaceSharePrincipals.mockRejectedValueOnce(new Error('Directory unavailable'))
    const onRefresh = vi.fn()
    renderHome({
      workspaces: [],
      designs: [],
      activeWorkspaceId: '',
      workspaceSearch: 'missing',
      designSearch: 'missing',
      workspacePage: null,
      designPage: null,
      error: 'Could not load workspace data',
      action: 'design',
      onRefresh,
    })

    expect(screen.getByText('No matching workspaces')).toBeTruthy()
    expect(screen.getByText('No matching designs')).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Creating...' }) as HTMLButtonElement).disabled).toBe(true)
    await user.click(screen.getByRole('button', { name: 'Retry' }))
    expect(onRefresh).toHaveBeenCalledTimes(1)

    await user.click(screen.getByRole('button', { name: 'New workspace' }))
    expect(await screen.findByText('Directory unavailable')).toBeTruthy()
    await user.type(screen.getByPlaceholderText('Find a person or group'), 'nobody')
    expect(screen.getByText('No matching people or groups')).toBeTruthy()
  })

  it('shows the empty-workspace call to action and conditionally offers workspace deletion', async () => {
    const user = userEvent.setup()
    const onDeleteWorkspace = vi.fn()
    const onNewDesign = vi.fn()
    renderHome({
      designs: [],
      workspacePage: null,
      designPage: { limit: 20, hasMore: false },
      onDeleteWorkspace,
      onNewDesign,
    })

    expect(screen.getByText('No designs yet')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Create design' }))
    expect(onNewDesign).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: 'Delete Payments' }))
    expect(onDeleteWorkspace).toHaveBeenCalledWith(workspaces[1])
    expect(screen.queryByRole('button', { name: 'Delete Guest Workspace' })).toBeNull()
  })

  it('shows effective access before opening and suppresses edit actions for read-only designs', () => {
    renderHome({
      designs: [{ ...design, effectiveAccess: 'read' }],
      workspaces: [{ ...workspaces[1], effectiveAccess: 'read' }],
    })

    expect(screen.getAllByText(/View only/).length).toBeGreaterThanOrEqual(2)
    expect(screen.queryByRole('button', { name: 'Delete Payment processing' })).toBeNull()
    expect((screen.getByRole('button', { name: 'New design' }) as HTMLButtonElement).disabled).toBe(true)
  })
})
