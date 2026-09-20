import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { createEmptyDesign } from '../src/designModel'
import type {
  BackendDesignReviewRequest,
  BackendDesignVersion,
  BackendNotification,
  BackendProfile,
} from '../src/backendApi'
import type { BackendDesign } from '../src/backendSync'
import { AppNavbar } from '../src/components/AppNavbar'

const profile: BackendProfile = {
  id: 'user-admin',
  displayName: 'Ada Admin',
  email: 'ada@example.com',
  role: 'admin',
}

const designs: BackendDesign[] = [
  {
    id: 'design-one',
    workspaceId: 'workspace-one',
    name: 'Checkout',
    access: 'private',
    title: 'Checkout',
    document: createEmptyDesign({ id: 'design-one', title: 'Checkout' }),
    versionNumber: 2,
    updatedAt: '2026-07-10T12:00:00Z',
  },
  {
    id: 'design-two',
    workspaceId: 'workspace-one',
    name: 'Settlement',
    access: 'private',
    title: 'Settlement',
    document: createEmptyDesign({ id: 'design-two', title: 'Settlement' }),
    versionNumber: 1,
    updatedAt: '2026-07-11T12:00:00Z',
  },
]

const versions: BackendDesignVersion[] = [
  {
    id: 'version-two', designId: 'design-one', workspaceId: 'workspace-one', versionNumber: 2,
    status: 'draft', remarks: 'Try async settlement', document: {}, createdBy: profile.id,
    createdAt: '2026-07-11T12:00:00Z', updatedAt: '2026-07-11T12:00:00Z',
  },
  {
    id: 'version-one', designId: 'design-one', workspaceId: 'workspace-one', versionNumber: 1,
    status: 'reviewed', remarks: 'Approved baseline', document: {}, createdBy: profile.id,
    createdAt: '2026-07-10T12:00:00Z', updatedAt: '2026-07-10T12:00:00Z',
  },
]

const reviews: BackendDesignReviewRequest[] = [
  {
    id: 'review-one', workspaceId: 'workspace-one', designId: 'design-one', versionId: 'version-one',
    versionNumber: 1, requestedBy: profile.id, reviewerId: 'reviewer-one', status: 'approved',
    message: '', summary: '', createdAt: '2026-07-10T12:00:00Z', updatedAt: '2026-07-10T12:00:00Z',
  },
]

const notifications: BackendNotification[] = [
  { id: 'n1', userId: profile.id, workspaceId: 'workspace-one', designId: 'design-one', type: 'review', title: 'Review', body: '', read: false, createdAt: '2026-07-11T12:00:00Z' },
  { id: 'n2', userId: profile.id, workspaceId: 'workspace-one', designId: 'design-one', type: 'review', title: 'Review', body: '', read: true, createdAt: '2026-07-10T12:00:00Z' },
]

function renderNavbar(overrides: Partial<React.ComponentProps<typeof AppNavbar>> = {}) {
  const props: React.ComponentProps<typeof AppNavbar> = {
    profile,
    currentUser: profile,
    route: { screen: 'design', workspaceId: 'workspace-one', designId: 'design-one' },
    workspaceName: 'Payments',
    designTitle: 'Checkout',
    designs,
    selectedDesignId: 'design-one',
    designVersions: versions,
    reviews,
    notifications,
    onHome: vi.fn(),
    onWorkspace: vi.fn(),
    onSelectDesign: vi.fn(),
    onVersionStatusChange: vi.fn(),
    onVersionReadyForReview: vi.fn(),
    onDeleteVersion: vi.fn(),
    onViewVersion: vi.fn(),
    onAdmin: vi.fn(),
    onOpenNotifications: vi.fn(),
    onLogout: vi.fn(),
    ...overrides,
  }
  return { ...render(<AppNavbar {...props} />), props }
}

describe('AppNavbar', () => {
  it('navigates breadcrumbs and switches designs', async () => {
    const user = userEvent.setup()
    const { props } = renderNavbar()

    await user.click(screen.getByRole('button', { name: /Home/i }))
    expect(props.onHome).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: 'Payments' }))
    expect(props.onWorkspace).toHaveBeenCalledTimes(1)
    await user.selectOptions(screen.getByRole('combobox', { name: 'Switch design' }), 'design-two')
    expect(props.onSelectDesign).toHaveBeenCalledWith(designs[1])
  })

  it('exposes actions for the selected version and preserves per-version review state', async () => {
    const user = userEvent.setup()
    const { props } = renderNavbar()

    await user.click(screen.getByTitle('Design versions'))
    const menu = document.querySelector('.version-menu-popover') as HTMLElement
    expect(within(menu).getByText('1/1 approved')).toBeTruthy()
    expect(within(menu).getByText('Try async settlement')).toBeTruthy()

    await user.click(within(menu).getByRole('button', { name: /Ready for review v2/i }))
    expect(props.onVersionReadyForReview).toHaveBeenCalledWith('version-two')
    const reviewedVersionRow = within(menu).getByText('v1').closest('.version-menu-row') as HTMLElement
    await user.click(within(reviewedVersionRow).getByRole('button', { name: /View/i }))
    expect(props.onViewVersion).toHaveBeenCalledWith(versions[1])
    await user.click(within(menu).getByRole('button', { name: 'Make live' }))
    expect(props.onVersionStatusChange).toHaveBeenCalledWith('version-one', 'live')
    await user.click(within(menu).getByTitle('Delete v1'))
    expect(props.onDeleteVersion).toHaveBeenCalledWith('version-one')
  })

  it('shows unread notifications and admin account actions', async () => {
    const user = userEvent.setup()
    const { props } = renderNavbar()

    const notificationButton = screen.getByTitle('Notifications')
    expect(within(notificationButton).getByText('1')).toBeTruthy()
    await user.click(notificationButton)
    expect(props.onOpenNotifications).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByTitle('ada@example.com'))
    await user.click(screen.getByRole('button', { name: /Admin console/i }))
    expect(props.onAdmin).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: /Sign out/i }))
    expect(props.onLogout).toHaveBeenCalledTimes(1)
  })

  it('keeps admin navigation hidden for non-admin users', () => {
    const member = { ...profile, id: 'member', role: 'member' }
    renderNavbar({ profile: member, currentUser: member, route: { screen: 'home' }, designVersions: [], reviews: [] })
    fireEvent.click(screen.getByTitle('ada@example.com'))
    expect(screen.queryByRole('button', { name: /Admin console/i })).toBeNull()
  })
})
