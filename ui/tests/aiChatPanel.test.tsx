import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ComponentProps } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AIChatPanel } from '../src/components/AIChatPanel'
import { createEmptyDesign } from '../src/designModel'

const { fetchAIConversations, fetchAIMessages, sendAIMessage, updateAIConversationAccess, applyAIDesignProposal } = vi.hoisted(() => ({
  fetchAIConversations: vi.fn(),
  fetchAIMessages: vi.fn(),
  sendAIMessage: vi.fn(),
  updateAIConversationAccess: vi.fn(),
  applyAIDesignProposal: vi.fn(),
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
    applyAIDesignProposal,
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
    currentDesign={createEmptyDesign({ id: 'design-1', title: 'Payments' })}
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
    applyAIDesignProposal.mockReset()
  })

  it('defaults to read-only and elevates an existing chat through the backend', async () => {
    const user = userEvent.setup()
    renderPanel()

    const readOnly = await screen.findByRole('button', { name: 'Read only' })
    expect(readOnly.getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByText(/cannot change it/i)).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Read + edit' }))
    await waitFor(() => expect(updateAIConversationAccess).toHaveBeenCalledWith('space-1', 'design-1', 'chat-1', 'read_write'))
    expect(screen.getByText(/nothing is saved until you review and apply/i)).toBeTruthy()
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
      followUps: ['Inspect retry policy'],
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

  it('keeps AI changes local until Apply and lets the user discard a proposal', async () => {
    const user = userEvent.setup()
    const currentDesign = createEmptyDesign({ id: 'design-1', title: 'Payments' })
    const proposedDesign = {
      ...currentDesign,
      components: [{
        id: 'cmp-db', shapeId: 'shape-db', type: 'data.sql_database' as const,
        name: 'Payments database', purpose: 'Store payments', owner: '', criticality: 'high' as const,
        metadata: { position: { x: 300, y: 100 } }, notes: [],
      }],
    }
    const updatedDesign = { id: 'design-1', workspaceId: 'space-1', name: 'Payments', title: 'Payments', document: proposedDesign, documentRevision: 'next', access: 'private', updatedAt: '' }
    const onDesignUpdated = vi.fn()
    fetchAIConversations.mockResolvedValue({ conversations: [{ ...conversation, accessMode: 'read_write' }] })
    fetchAIMessages.mockResolvedValue({ conversation: { ...conversation, accessMode: 'read_write' }, messages: [] })
    sendAIMessage.mockResolvedValue({
      userMessage: { id: 'm1', conversationId: 'chat-1', role: 'user', content: 'Add a database', createdAt: '' },
      assistantMessage: { id: 'm2', conversationId: 'chat-1', role: 'assistant', content: 'I propose a database.', createdAt: '' },
      followUps: [], proposedDesign, baseRevision: 'original', updateSummary: 'Add a payment store',
    })
    applyAIDesignProposal.mockResolvedValue({ design: updatedDesign })
    renderPanel({ currentDesign, onDesignUpdated })

    await screen.findByRole('button', { name: 'Read + edit' })
    await user.type(screen.getByPlaceholderText(/Ask about risks/), 'Add a database')
    await user.click(screen.getByRole('button', { name: 'Send message' }))
    expect(await screen.findByText('Add a payment store')).toBeTruthy()
    expect(screen.getByText('Payments database')).toBeTruthy()
    expect(applyAIDesignProposal).not.toHaveBeenCalled()
    expect(onDesignUpdated).not.toHaveBeenCalled()
    expect((screen.getByRole('button', { name: 'Send message' }) as HTMLButtonElement).disabled).toBe(true)

    await user.click(screen.getByRole('button', { name: 'Discard proposal' }))
    expect(screen.queryByText('Add a payment store')).toBeNull()
    expect(onDesignUpdated).not.toHaveBeenCalled()

    await user.type(screen.getByPlaceholderText(/Ask about risks/), 'Add a database')
    await user.click(screen.getByRole('button', { name: 'Send message' }))
    await screen.findByText('Add a payment store')
    await user.click(screen.getByRole('button', { name: 'Apply to design' }))
    await waitFor(() => expect(applyAIDesignProposal).toHaveBeenCalledWith('space-1', 'design-1', 'chat-1', proposedDesign, 'original'))
    expect(onDesignUpdated).toHaveBeenCalledWith(updatedDesign)
    expect(screen.queryByText('Add a payment store')).toBeNull()
  })

  it('keeps a stale proposal reviewable when Apply fails instead of overwriting local state', async () => {
    const user = userEvent.setup()
    const document = createEmptyDesign({ id: 'design-1', title: 'Payments' })
    const onDesignUpdated = vi.fn()
    fetchAIConversations.mockResolvedValue({ conversations: [{ ...conversation, accessMode: 'read_write' }] })
    fetchAIMessages.mockResolvedValue({ conversation: { ...conversation, accessMode: 'read_write' }, messages: [] })
    sendAIMessage.mockResolvedValue({
      userMessage: { id: 'm1', conversationId: 'chat-1', role: 'user', content: 'Improve design', createdAt: '' },
      assistantMessage: { id: 'm2', conversationId: 'chat-1', role: 'assistant', content: 'Here is a proposal.', createdAt: '' },
      followUps: [], proposedDesign: document, baseRevision: 'stale', updateSummary: 'Proposed improvement',
    })
    applyAIDesignProposal.mockRejectedValue(new Error('Backend request failed: 409 - The design changed; request a new proposal.'))
    renderPanel({ currentDesign: document, onDesignUpdated })

    await screen.findByRole('button', { name: 'Read + edit' })
    await user.type(screen.getByPlaceholderText(/Ask about risks/), 'Improve design')
    await user.click(screen.getByRole('button', { name: 'Send message' }))
    await screen.findByText('Proposed improvement')
    await user.click(screen.getByRole('button', { name: 'Apply to design' }))
    expect((await screen.findByRole('alert')).textContent).toContain('design changed')
    expect(screen.getByText('Proposed improvement')).toBeTruthy()
    expect(onDesignUpdated).not.toHaveBeenCalled()
  })

  it('renders assistant Markdown while dropping raw HTML and remote images', async () => {
    fetchAIMessages.mockResolvedValue({
      conversation,
      messages: [{
        id: 'm2', conversationId: conversation.id, role: 'assistant', createdAt: '2026-09-21T00:00:02Z',
        content: '### Recommendation\n\n- Add **bounded retries**\n- Track `queue_depth`\n\n[Runbook](https://docs.example.com)\n\n<img src=x onerror="alert(1)">\n\n![tracker](https://tracker.example/pixel.png)',
      }],
    })
    const { container } = renderPanel()

    expect(await screen.findByRole('heading', { name: 'Recommendation' })).toBeTruthy()
    expect(screen.getByText('bounded retries').tagName).toBe('STRONG')
    expect(screen.getByText('queue_depth').tagName).toBe('CODE')
    const link = screen.getByRole('link', { name: 'Runbook' })
    expect(link.getAttribute('target')).toBe('_blank')
    expect(link.getAttribute('rel')).toContain('noopener')
    expect(container.querySelector('img')).toBeNull()
    expect(container.innerHTML).not.toContain('onerror')
  })
})
