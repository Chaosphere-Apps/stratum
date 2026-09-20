import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { AIChatPanel } from '../src/components/AIChatPanel'
import * as api from '../src/backendApi'

vi.mock('../src/backendApi', async () => {
  const actual = await vi.importActual<typeof import('../src/backendApi')>('../src/backendApi')
  return {
    ...actual,
    fetchAIConversations: vi.fn(),
    fetchAIMessages: vi.fn(),
    createAIConversation: vi.fn(),
    sendAIMessage: vi.fn(),
  }
})

const conversation = {
  id: 'conversation-1', workspaceId: 'workspace', designId: 'design', versionId: 'version-1',
  title: 'Reliability review', createdBy: 'user', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z',
}

describe('AIChatPanel', () => {
  beforeEach(() => {
    vi.mocked(api.fetchAIConversations).mockResolvedValue({ conversations: [conversation] })
    vi.mocked(api.fetchAIMessages).mockResolvedValue({ conversation, messages: [] })
    vi.mocked(api.sendAIMessage).mockResolvedValue({
      userMessage: { id: 'm1', conversationId: conversation.id, role: 'user', content: 'Review queue', createdAt: '2026-01-01T00:00:01Z' },
      assistantMessage: {
        id: 'm2', conversationId: conversation.id, role: 'assistant', content: 'Add a retry policy.', createdAt: '2026-01-01T00:00:02Z',
        references: [{ kind: 'component', id: 'queue-1', name: 'Delivery queue' }],
      },
      followUps: [],
    })
  })

  it('loads a version-scoped conversation and navigates through grounded references', async () => {
    const onSelectReference = vi.fn()
    render(
      <AIChatPanel
        workspaceId="workspace"
        designId="design"
        designName="Notifications"
        versions={[{ id: 'version-1', workspaceId: 'workspace', designId: 'design', versionNumber: 4, status: 'draft', remarks: '', document: {}, createdBy: 'user', createdAt: '', updatedAt: '' }]}
        currentVersionId="version-1"
        providerEnabled
        onClose={vi.fn()}
        onSelectReference={onSelectReference}
      />,
    )

    await waitFor(() => expect(screen.getByText(/Saved version v4/)).toBeTruthy())
    fireEvent.change(screen.getByPlaceholderText(/Ask about risks/), { target: { value: 'Review queue' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }))
    await waitFor(() => expect(screen.getByText('Add a retry policy.')).toBeTruthy())
    fireEvent.click(screen.getByRole('button', { name: 'Delivery queue' }))
    expect(onSelectReference).toHaveBeenCalledWith({ kind: 'component', id: 'queue-1', name: 'Delivery queue' })
  })

  it('clearly disables chat when no enterprise provider is configured', async () => {
    render(
      <AIChatPanel workspaceId="workspace" designId="design" designName="Notifications" versions={[]} providerEnabled={false} onClose={vi.fn()} onSelectReference={vi.fn()} />,
    )
    await waitFor(() => expect(screen.getByText('AI is not configured')).toBeTruthy())
    expect((screen.getByRole('button', { name: 'Send message' }) as HTMLButtonElement).disabled).toBe(true)
  })
})
