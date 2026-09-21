import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ComponentProps } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AIChatPanel } from '../src/components/AIChatPanel'

const { fetchAIConversations, fetchAIMessages, sendAIMessage, updateAIConversationAccess } = vi.hoisted(() => ({
  fetchAIConversations: vi.fn(),
  fetchAIMessages: vi.fn(),
  sendAIMessage: vi.fn(),
  updateAIConversationAccess: vi.fn(),
}))

vi.mock('../src/backendApi', async () => {
  const actual = await vi.importActual<typeof import('../src/backendApi')>('../src/backendApi')
  return {
    ...actual,
    createAIConversation: vi.fn(),
    fetchAIConversations,
    fetchAIMessages,
    sendAIMessage,
    updateAIConversationAccess,
  }
})

const conversation = {
  id: 'chat-1',
  workspaceId: 'space-1',
  designId: 'design-1',
  title: 'Architecture review',
  accessMode: 'read' as const,
  createdBy: 'user-1',
  createdAt: '2026-09-21T00:00:00Z',
  updatedAt: '2026-09-21T00:00:00Z',
}

function renderPanel(patch: Partial<ComponentProps<typeof AIChatPanel>> = {}) {
  return render(<AIChatPanel
    workspaceId="space-1"
    designId="design-1"
    designName="Payments"
    versions={[]}
    providerEnabled
    canEditDesign
    onClose={vi.fn()}
    onDesignUpdated={vi.fn()}
    onSelectReference={vi.fn()}
    {...patch}
  />)
}

describe('AIChatPanel tool access', () => {
  beforeEach(() => {
    fetchAIConversations.mockReset().mockResolvedValue({ conversations: [conversation] })
    fetchAIMessages.mockReset().mockResolvedValue({ conversation, messages: [] })
    sendAIMessage.mockReset()
    updateAIConversationAccess.mockReset().mockResolvedValue({ conversation: { ...conversation, accessMode: 'read_write' } })
  })

  it('defaults to read-only and elevates an existing chat through the backend', async () => {
    const user = userEvent.setup()
    renderPanel()

    const readOnly = await screen.findByRole('button', { name: 'Read only' })
    expect(readOnly.getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByText(/cannot change it/i)).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Read + edit' }))
    await waitFor(() => expect(updateAIConversationAccess).toHaveBeenCalledWith('space-1', 'design-1', 'chat-1', 'read_write'))
    expect(screen.getByText(/only when you explicitly ask/i)).toBeTruthy()
  })

  it('does not offer write tools for a saved version', async () => {
    renderPanel({ currentVersionId: 'version-1', versions: [{
      id: 'version-1', designId: 'design-1', workspaceId: 'space-1', versionNumber: 1,
      status: 'reviewed', remarks: '', document: {}, createdBy: 'user-1',
      createdAt: '2026-09-21T00:00:00Z', updatedAt: '2026-09-21T00:00:00Z',
    }] })

    const writeButton = await screen.findByRole('button', { name: 'Read + edit' }) as HTMLButtonElement
    expect(writeButton.disabled).toBe(true)
    expect(updateAIConversationAccess).not.toHaveBeenCalled()
  })

  it('loads a version-scoped conversation and navigates through grounded references', async () => {
    const user = userEvent.setup()
    const versionConversation = { ...conversation, versionId: 'version-1' }
    const onSelectReference = vi.fn()
    fetchAIConversations.mockResolvedValue({ conversations: [versionConversation] })
    fetchAIMessages.mockResolvedValue({ conversation: versionConversation, messages: [] })
    sendAIMessage.mockResolvedValue({
      userMessage: { id: 'm1', conversationId: conversation.id, role: 'user', content: 'Review queue', createdAt: '2026-09-21T00:00:01Z' },
      assistantMessage: {
        id: 'm2', conversationId: conversation.id, role: 'assistant', content: 'Add a retry policy.', createdAt: '2026-09-21T00:00:02Z',
        references: [{ kind: 'component', id: 'queue-1', name: 'Delivery queue' }],
      },
      followUps: ['Inspect retry policy'], designUpdated: false,
    })
    renderPanel({
      currentVersionId: 'version-1',
      versions: [{
        id: 'version-1', designId: 'design-1', workspaceId: 'space-1', versionNumber: 4,
        status: 'draft', remarks: '', document: {}, createdBy: 'user-1', createdAt: '', updatedAt: '',
      }],
      onSelectReference,
    })

    await screen.findByText(/Saved version v4/)
    await user.type(screen.getByPlaceholderText(/Ask about risks/), 'Review queue')
    await user.click(screen.getByRole('button', { name: 'Send message' }))
    await screen.findByText('Add a retry policy.')
    await user.click(screen.getByRole('button', { name: 'Delivery queue' }))
    expect(onSelectReference).toHaveBeenCalledWith({ kind: 'component', id: 'queue-1', name: 'Delivery queue' })
    await user.click(screen.getByRole('button', { name: 'Inspect retry policy' }))
    await waitFor(() => expect(sendAIMessage).toHaveBeenLastCalledWith('space-1', 'design-1', 'chat-1', 'Inspect retry policy'))
  })

  it('clearly disables chat when no enterprise provider is configured', async () => {
    renderPanel({ providerEnabled: false })
    await screen.findByText('AI is not configured')
    expect((screen.getByRole('button', { name: 'Send message' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('shows the safe provider explanation without the generic backend prefix', async () => {
    const user = userEvent.setup()
    sendAIMessage.mockRejectedValue(new Error('Backend request failed: 503 - The AI provider is temporarily unavailable. Your message was saved; try again shortly.'))
    renderPanel()

    await screen.findByRole('button', { name: 'Read only' })
    await user.type(screen.getByPlaceholderText(/Ask about risks/), 'Review this design')
    await user.click(screen.getByRole('button', { name: 'Send message' }))

    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('AI provider is temporarily unavailable')
    expect(alert.textContent).not.toContain('Backend request failed')
  })
})
