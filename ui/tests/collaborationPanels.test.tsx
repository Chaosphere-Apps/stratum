import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import type {
  BackendDesignComment,
  BackendDesignReviewRequest,
  BackendDesignVersion,
  BackendNotification,
  BackendUser,
} from '../src/backendApi'
import { NotificationsModal, RequestReviewModal, ReviewWorkspace } from '../src/components/CollaborationPanels'

const users: BackendUser[] = [
  {
    id: 'author', displayName: 'Ada Architect', email: 'ada@example.com', passwordSet: true,
    role: 'architect', status: 'active', lastSeenAt: '2026-07-18T10:00:00Z',
    createdAt: '2026-07-01T10:00:00Z', updatedAt: '2026-07-18T10:00:00Z',
  },
  {
    id: 'reviewer', displayName: 'Ravi Reviewer', email: 'ravi@example.com', passwordSet: true,
    role: 'reviewer', status: 'active', lastSeenAt: '2026-07-18T10:00:00Z',
    createdAt: '2026-07-01T10:00:00Z', updatedAt: '2026-07-18T10:00:00Z',
  },
]

const version: BackendDesignVersion = {
  id: 'version-3', designId: 'design-1', workspaceId: 'workspace-1', versionNumber: 3,
  status: 'pending_review', remarks: 'Review checkout flow', document: {}, createdBy: 'author',
  createdAt: '2026-07-18T09:00:00Z', updatedAt: '2026-07-18T09:00:00Z',
}

const reviews: BackendDesignReviewRequest[] = [
  {
    id: 'review-current', workspaceId: 'workspace-1', designId: 'design-1', versionId: version.id,
    versionNumber: 3, requestedBy: 'author', reviewerId: 'reviewer', status: 'requested',
    message: 'Please validate failure handling', summary: '', createdAt: '2026-07-18T09:00:00Z',
    updatedAt: '2026-07-18T10:00:00Z',
  },
  {
    id: 'review-old', workspaceId: 'workspace-1', designId: 'design-1', versionId: 'version-2',
    versionNumber: 2, requestedBy: 'author', reviewerId: 'reviewer', status: 'approved',
    message: '', summary: 'Looks good', createdAt: '2026-07-17T09:00:00Z', updatedAt: '2026-07-17T10:00:00Z',
  },
]

const comments: BackendDesignComment[] = [
  {
    id: 'comment-1', workspaceId: 'workspace-1', designId: 'design-1', authorId: 'reviewer',
    body: 'What happens when Redis fails?', componentId: 'redis-1', createdAt: '2026-07-18T10:00:00Z',
  },
]

function renderWorkspace(overrides: Partial<React.ComponentProps<typeof ReviewWorkspace>> = {}) {
  const props: React.ComponentProps<typeof ReviewWorkspace> = {
    mode: 'comment', users, activeUserId: 'reviewer', comments, reviews, activeVersion: version,
    selectedComponent: null, selectedConnector: null, commentDraft: '', reviewSummaryDrafts: {},
    isSaving: false, error: null, offline: false, onCommentDraftChange: vi.fn(),
    onSubmitComment: vi.fn(), onRequestReview: vi.fn(), onReviewSummaryChange: vi.fn(),
    onCompleteReview: vi.fn(), onRefresh: vi.fn(), ...overrides,
  }
  render(<ReviewWorkspace {...props} />)
  return props
}

describe('ReviewWorkspace', () => {
  it('shows per-version review metrics and completes the assigned review', () => {
    const props = renderWorkspace({ reviewSummaryDrafts: { 'review-current': 'Retry policy is unclear' } })

    expect(screen.getByText('v3')).toBeTruthy()
    expect(screen.getByText('1 requested')).toBeTruthy()
    expect(screen.getByText('0 approved')).toBeTruthy()
    fireEvent.change(screen.getByPlaceholderText('Summarize your review...'), { target: { value: 'Add timeout budgets' } })
    expect(props.onReviewSummaryChange).toHaveBeenCalledWith('review-current', 'Add timeout budgets')
    fireEvent.click(screen.getByRole('button', { name: 'Request changes' }))
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    expect(props.onCompleteReview).toHaveBeenNthCalledWith(1, 'review-current', 'changes_requested')
    expect(props.onCompleteReview).toHaveBeenNthCalledWith(2, 'review-current', 'approved')
  })

  it('captures comments and identifies the selected target', () => {
    const props = renderWorkspace({
      commentDraft: 'Document this decision',
      selectedComponent: { id: 'service-1', type: 'compute.service', name: 'Checkout API', x: 0, y: 0, metadata: {} },
    })
    expect(screen.getByText('Target: Checkout API')).toBeTruthy()
    fireEvent.change(screen.getByPlaceholderText('Capture a question, decision, or review note...'), { target: { value: 'New note' } })
    expect(props.onCommentDraftChange).toHaveBeenCalledWith('New note')
    fireEvent.click(screen.getByRole('button', { name: 'Add comment' }))
    expect(props.onSubmitComment).toHaveBeenCalledTimes(1)
    expect(screen.getByText('What happens when Redis fails?')).toBeTruthy()
    expect(screen.getByText('Component: redis-1')).toBeTruthy()
  })

  it('keeps view mode useful when collaboration is offline', () => {
    const props = renderWorkspace({ mode: 'view', offline: true, error: 'Load failed', comments: [], reviews: [] })
    expect(screen.getByText('Collaboration backend offline')).toBeTruthy()
    expect(screen.getByText('Load failed')).toBeTruthy()
    expect(screen.getByText('No reviews yet')).toBeTruthy()
    expect(screen.getByText('No comments yet')).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Request review' }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(props.onRefresh).toHaveBeenCalledTimes(1)
  })
})

describe('RequestReviewModal', () => {
  it('requires at least one reviewer and submits all selected reviewers', () => {
    const onRequest = vi.fn()
    render(<RequestReviewModal users={users} activeUserId="author" version={version} isSaving={false} onCancel={vi.fn()} onRequest={onRequest} />)
    const submit = screen.getByRole('button', { name: 'Request review' }) as HTMLButtonElement
    expect(submit.disabled).toBe(true)
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.change(screen.getByRole('textbox', { name: 'Message' }), { target: { value: 'Review resilience' } })
    fireEvent.click(submit)
    expect(onRequest).toHaveBeenCalledWith(['reviewer'], 'Review resilience')
  })

  it('does not allow review requests without a saved version', () => {
    render(<RequestReviewModal users={users} activeUserId="author" version={null} isSaving={false} onCancel={vi.fn()} onRequest={vi.fn()} />)
    expect(screen.getByText('Create or save a version before requesting review.')).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Request review' }) as HTMLButtonElement).disabled).toBe(true)
  })
})

describe('NotificationsModal', () => {
  it('marks unread notifications and closes the modal', () => {
    const notifications: BackendNotification[] = [{
      id: 'notification-1', userId: 'reviewer', workspaceId: 'workspace-1', designId: 'design-1',
      type: 'review_requested', title: 'Review requested', body: 'Checkout v3 needs review', read: false,
      createdAt: '2026-07-18T10:00:00Z',
    }]
    const onMarkRead = vi.fn()
    const onClose = vi.fn()
    render(<NotificationsModal notifications={notifications} users={users} onMarkRead={onMarkRead} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(onMarkRead).toHaveBeenCalledWith('notification-1')
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(screen.getByText('For Ravi Reviewer')).toBeTruthy()
  })

  it('renders the notification empty state', () => {
    render(<NotificationsModal notifications={[]} users={users} onMarkRead={vi.fn()} onClose={vi.fn()} />)
    expect(screen.getByText('No notifications')).toBeTruthy()
  })
})
