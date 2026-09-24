import { useEffect, useMemo, useRef, useState } from 'react'
import { Bot, ChevronDown, Eye, MessageCirclePlus, PencilLine, Send, ShieldCheck, X } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  applyAIDesignProposal,
  createAIConversation,
  fetchAIConversations,
  fetchAIMessages,
  sendAIMessage,
  updateAIConversationAccess,
  type BackendAIConversation,
  type BackendAIMessage,
  type BackendAIMessageReference,
  type BackendDesignVersion,
} from '../backendApi'
import type { BackendDesign } from '../backendSync'

interface AIChatPanelProps {
  workspaceId: string
  designId: string
  designName: string
  versions: BackendDesignVersion[]
  currentVersionId?: string
  providerEnabled: boolean
  canEditDesign: boolean
  currentDesign: BackendDesign['document']
  writeAccessReason?: string
  onClose: () => void
  onDesignUpdated: (design: BackendDesign) => void
  onSelectReference: (reference: BackendAIMessageReference) => void
}

const starterPrompts = [
  'What is the highest-risk assumption in this design?',
  'Trace the critical request path and identify failure points.',
  'Which requirement is least supported by the current architecture?',
]

const chatMarkdownElements = [
  'p', 'strong', 'em', 'ul', 'ol', 'li', 'code', 'pre', 'blockquote',
  'h1', 'h2', 'h3', 'h4', 'a', 'hr', 'br', 'table', 'thead', 'tbody', 'tr', 'th', 'td',
]

export function AIChatMarkdown({ content }: { content: string }) {
  return (
    <div className="ai-chat-markdown">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        allowedElements={chatMarkdownElements}
        unwrapDisallowed
        skipHtml
        components={{
          a: ({ children, ...props }) => <a {...props} target="_blank" rel="noreferrer noopener">{children}</a>,
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}

function aiChatErrorMessage(reason: unknown, fallback: string) {
  if (!(reason instanceof Error)) return fallback
  return reason.message.replace(/^Backend request failed: \d+(?:\s+-\s+)?/, '') || fallback
}

export function AIChatPanel({
  workspaceId,
  designId,
  designName,
  versions,
  currentVersionId,
  providerEnabled,
  canEditDesign,
  currentDesign,
  writeAccessReason,
  onClose,
  onDesignUpdated,
  onSelectReference,
}: AIChatPanelProps) {
  const [conversations, setConversations] = useState<BackendAIConversation[]>([])
  const [activeId, setActiveId] = useState('')
  const [messages, setMessages] = useState<BackendAIMessage[]>([])
  const [followUps, setFollowUps] = useState<string[]>([])
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [newConversationAccess, setNewConversationAccess] = useState<'read' | 'read_write'>('read')
  const [pendingProposal, setPendingProposal] = useState<{
    document: BackendDesign['document']
    baseRevision: string
    conversationId: string
    summary: string
  } | null>(null)
  const [proposalDocumentOpen, setProposalDocumentOpen] = useState(false)
  const endRef = useRef<HTMLDivElement | null>(null)

  const activeConversation = conversations.find((item) => item.id === activeId)
  const accessMode = activeConversation?.accessMode ?? newConversationAccess
  const writeAvailable = canEditDesign && !currentVersionId
  const proposalChanges = useMemo(() => {
    if (!pendingProposal) return null
    const before = new Map(currentDesign.components.map((component) => [component.id, component]))
    const after = new Map(pendingProposal.document.components.map((component) => [component.id, component]))
    const previousConnections = new Map(currentDesign.connectors.map((connector) => [connector.id, connector]))
    const proposedConnections = new Map(pendingProposal.document.connectors.map((connector) => [connector.id, connector]))
    return {
      added: pendingProposal.document.components.filter((component) => !before.has(component.id)).map((component) => component.name),
      removed: currentDesign.components.filter((component) => !after.has(component.id)).map((component) => component.name),
      changed: pendingProposal.document.components.filter((component) => {
        const previous = before.get(component.id)
        return previous && JSON.stringify(previous) !== JSON.stringify(component)
      }).map((component) => component.name),
      addedConnections: pendingProposal.document.connectors.filter((connector) => !previousConnections.has(connector.id)).length,
      removedConnections: currentDesign.connectors.filter((connector) => !proposedConnections.has(connector.id)).length,
      changedConnections: pendingProposal.document.connectors.filter((connector) => {
        const previous = previousConnections.get(connector.id)
        return previous && JSON.stringify(previous) !== JSON.stringify(connector)
      }).length,
      requirementsChanged: JSON.stringify(currentDesign.requirementBrief) !== JSON.stringify(pendingProposal.document.requirementBrief),
      journeysChanged: JSON.stringify(currentDesign.journeys) !== JSON.stringify(pendingProposal.document.journeys),
    }
  }, [currentDesign, pendingProposal])
  const versionLabel = useMemo(() => {
    const id = activeConversation?.versionId ?? currentVersionId
    if (!id) return 'Working design'
    const version = versions.find((item) => item.id === id)
    return version ? `Saved version v${version.versionNumber}` : 'Saved version'
  }, [activeConversation?.versionId, currentVersionId, versions])

  useEffect(() => {
    let live = true
    setLoading(true)
    setError(null)
    void fetchAIConversations(workspaceId, designId)
      .then(({ conversations: loaded }) => {
        if (!live) return
        setConversations(loaded)
        setActiveId((current) => current && loaded.some((item) => item.id === current) ? current : loaded[0]?.id ?? '')
      })
      .catch((reason) => live && setError(aiChatErrorMessage(reason, 'Could not load AI conversations')))
      .finally(() => live && setLoading(false))
    return () => { live = false }
  }, [workspaceId, designId])

  useEffect(() => {
    if (!activeId) {
      setMessages([])
      setFollowUps([])
      return
    }
    let live = true
    setLoading(true)
    setError(null)
    setFollowUps([])
    void fetchAIMessages(workspaceId, designId, activeId)
      .then(({ messages: loaded }) => live && setMessages(loaded))
      .catch((reason) => live && setError(aiChatErrorMessage(reason, 'Could not load this conversation')))
      .finally(() => live && setLoading(false))
    return () => { live = false }
  }, [workspaceId, designId, activeId])

  useEffect(() => {
    endRef.current?.scrollIntoView?.({ behavior: 'smooth', block: 'nearest' })
  }, [messages, busy])

  async function startConversation() {
    if (pendingProposal) return
    setBusy(true)
    setError(null)
    try {
      const response = await createAIConversation(workspaceId, designId, {
        title: `Architecture review · ${new Date().toLocaleDateString()}`,
        versionId: currentVersionId,
        accessMode: writeAvailable ? newConversationAccess : 'read',
      })
      setConversations((current) => [response.conversation, ...current])
      setActiveId(response.conversation.id)
      setMessages([])
      setFollowUps([])
    } catch (reason) {
      setError(aiChatErrorMessage(reason, 'Could not start a conversation'))
    } finally {
      setBusy(false)
    }
  }

  async function submit(content = draft) {
    const message = content.trim()
    if (!message || busy || !providerEnabled || pendingProposal) return
    setBusy(true)
    setError(null)
    setFollowUps([])
    try {
      let conversationId = activeId
      if (!conversationId) {
        const created = await createAIConversation(workspaceId, designId, {
          title: message.length > 54 ? `${message.slice(0, 54)}…` : message,
          versionId: currentVersionId,
          accessMode: writeAvailable ? newConversationAccess : 'read',
        })
        conversationId = created.conversation.id
        setConversations((current) => [created.conversation, ...current])
        setActiveId(conversationId)
      }
      setDraft('')
      const response = await sendAIMessage(workspaceId, designId, conversationId, message)
      setMessages((current) => [...current, response.userMessage, response.assistantMessage])
      setFollowUps(response.followUps ?? [])
      if (response.proposedDesign && response.baseRevision) {
        setProposalDocumentOpen(false)
        setPendingProposal({
          document: response.proposedDesign,
          baseRevision: response.baseRevision,
          conversationId,
          summary: response.updateSummary || 'Review the proposed design changes.',
        })
      }
    } catch (reason) {
      setError(aiChatErrorMessage(reason, 'AI could not answer this question'))
    } finally {
      setBusy(false)
    }
  }

  async function applyProposal() {
    if (!pendingProposal || busy || !writeAvailable || accessMode !== 'read_write') return
    setBusy(true)
    setError(null)
    try {
      const response = await applyAIDesignProposal(workspaceId, designId, pendingProposal.conversationId, pendingProposal.document, pendingProposal.baseRevision)
      onDesignUpdated(response.design)
      setPendingProposal(null)
    } catch (reason) {
      setError(aiChatErrorMessage(reason, 'Could not apply the AI proposal'))
    } finally {
      setBusy(false)
    }
  }

  async function changeAccessMode(nextAccessMode: 'read' | 'read_write') {
    if (nextAccessMode === 'read_write' && !writeAvailable) return
    if (!activeConversation) {
      setNewConversationAccess(nextAccessMode)
      return
    }
    setBusy(true)
    setError(null)
    try {
      const response = await updateAIConversationAccess(workspaceId, designId, activeConversation.id, nextAccessMode)
      setConversations((current) => current.map((item) => item.id === response.conversation.id ? response.conversation : item))
    } catch (reason) {
      setError(aiChatErrorMessage(reason, 'Could not change AI tool access'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="ai-chat-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && !pendingProposal && onClose()}>
      <aside className="ai-chat-panel" role="dialog" aria-modal="true" aria-labelledby="ai-chat-title">
        <header className="ai-chat-header">
          <div className="ai-chat-heading">
            <span className="ai-chat-mark"><Bot size={18} /></span>
            <div><strong id="ai-chat-title">Architecture copilot</strong><small>{designName} · {versionLabel}</small></div>
          </div>
          <button className="icon-button" type="button" onClick={onClose} disabled={Boolean(pendingProposal)} title={pendingProposal ? 'Apply or discard the pending proposal before closing.' : undefined} aria-label="Close architecture copilot"><X size={18} /></button>
        </header>

        <div className="ai-chat-threadbar">
          <label>
            <span>Conversation</span>
            <span className="ai-chat-select-wrap">
              <select value={activeId} onChange={(event) => setActiveId(event.target.value)} disabled={!conversations.length || Boolean(pendingProposal)}>
                {!conversations.length ? <option value="">New conversation</option> : null}
                {conversations.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}
              </select>
              <ChevronDown size={15} />
            </span>
          </label>
          <button className="secondary-action compact" type="button" onClick={() => void startConversation()} disabled={busy || Boolean(pendingProposal)}>
            <MessageCirclePlus size={16} /> New
          </button>
        </div>

        <div className="ai-chat-access">
          <span className="ai-chat-access-label"><ShieldCheck size={16} /><span><strong>Design access</strong><small>Applies only to this conversation</small></span></span>
          <div className="ai-chat-access-options" role="group" aria-label="AI design access">
            <button type="button" className={accessMode === 'read' ? 'active' : ''} onClick={() => void changeAccessMode('read')} disabled={busy || Boolean(pendingProposal)} aria-pressed={accessMode === 'read'}><Eye size={15} /> Read only</button>
            <button type="button" className={accessMode === 'read_write' ? 'active' : ''} onClick={() => void changeAccessMode('read_write')} disabled={busy || Boolean(pendingProposal) || !writeAvailable} aria-pressed={accessMode === 'read_write'} title={!writeAvailable ? writeAccessReason ?? 'Edit access is unavailable for this design.' : undefined}><PencilLine size={15} /> Read + edit</button>
          </div>
          <small>{accessMode === 'read_write' ? 'The copilot may propose changes when you ask. Nothing is saved until you review and apply the proposal.' : writeAccessReason ?? 'The copilot can inspect this design but cannot change it.'}</small>
        </div>

        <div className="ai-chat-messages" aria-live="polite">
          {!providerEnabled ? (
            <div className="ai-chat-empty"><Bot size={24} /><strong>AI is not configured</strong><span>An administrator must connect the approved provider in the AI suite.</span></div>
          ) : loading && !messages.length ? (
            <div className="ai-chat-empty"><span className="ai-thinking" /><strong>Loading conversation</strong></div>
          ) : !messages.length ? (
            <div className="ai-chat-welcome">
              <span className="ai-chat-mark large"><Bot size={22} /></span>
              <strong>Ask from the architecture, not from a blank chat.</strong>
              <p>Answers are grounded in this design and its deterministic analysis. Design changes require Read + edit access and an explicit request.</p>
              <div className="ai-starter-prompts">
                {starterPrompts.map((prompt) => <button key={prompt} type="button" onClick={() => void submit(prompt)}>{prompt}</button>)}
              </div>
            </div>
          ) : messages.map((message) => (
            <article className={`ai-chat-message ${message.role}`} key={message.id}>
              <span>{message.role === 'assistant' ? 'Stratum' : 'You'}</span>
              {message.role === 'assistant' ? <AIChatMarkdown content={message.content} /> : <p>{message.content}</p>}
              {message.references?.length ? (
                <div className="ai-message-references">
                  {message.references.map((reference) => (
                    <button key={`${reference.kind}:${reference.id}`} type="button" onClick={() => onSelectReference(reference)}>
                      {reference.name || reference.id}
                    </button>
                  ))}
                </div>
              ) : null}
            </article>
          ))}
          {followUps.length && !busy ? (
            <div className="ai-chat-followups">
              <span>Continue the review</span>
              {followUps.map((followUp) => <button key={followUp} type="button" onClick={() => void submit(followUp)}>{followUp}</button>)}
            </div>
          ) : null}
          {pendingProposal && proposalChanges ? (
            <section className="ai-chat-proposal" aria-label="Proposed design changes">
              <strong>Review proposed design</strong>
              <p>{pendingProposal.summary}</p>
              <small>Temporary preview · nothing has been saved to the design.</small>
              <div className="ai-chat-proposal-changes">
                <span>{proposalChanges.added.length} added</span>
                <span>{proposalChanges.changed.length} changed</span>
                <span>{proposalChanges.removed.length} removed</span>
                <span>{proposalChanges.addedConnections} connections added</span>
                <span>{proposalChanges.changedConnections} connections changed</span>
                <span>{proposalChanges.removedConnections} connections removed</span>
              </div>
              {proposalChanges.requirementsChanged ? <p>Requirements changed</p> : null}
              {proposalChanges.journeysChanged ? <p>Journeys changed</p> : null}
              {proposalChanges.added.length ? <p><b>Added:</b> {proposalChanges.added.join(', ')}</p> : null}
              {proposalChanges.changed.length ? <p><b>Changed:</b> {proposalChanges.changed.join(', ')}</p> : null}
              {proposalChanges.removed.length ? <p className="ai-chat-proposal-warning"><b>Removed:</b> {proposalChanges.removed.join(', ')}</p> : null}
              <details className="ai-chat-proposal-document" onToggle={(event) => setProposalDocumentOpen(event.currentTarget.open)}>
                <summary>Inspect full proposed document</summary>
                {proposalDocumentOpen ? <pre>{JSON.stringify(pendingProposal.document, null, 2)}</pre> : null}
              </details>
              <div className="ai-chat-proposal-actions">
                <button className="secondary-action" type="button" onClick={() => setPendingProposal(null)} disabled={busy}>Discard proposal</button>
                <button className="primary-action" type="button" onClick={() => void applyProposal()} disabled={busy || !writeAvailable || accessMode !== 'read_write'} title={!writeAvailable ? writeAccessReason : undefined}>Apply to design</button>
              </div>
            </section>
          ) : null}
          {busy ? <div className="ai-chat-thinking"><span className="ai-thinking" /> Reviewing the model…</div> : null}
          <div ref={endRef} />
        </div>

        {error ? <div className="ai-chat-error" role="alert">{error}</div> : null}
        <form className="ai-chat-composer" onSubmit={(event) => { event.preventDefault(); void submit() }}>
          <textarea
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            placeholder="Ask about risks, flows, requirements, or tradeoffs…"
            rows={3}
            maxLength={8000}
            disabled={!providerEnabled || busy || Boolean(pendingProposal)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void submit() }
            }}
          />
          <div><small>{pendingProposal ? 'Apply or discard the proposal before continuing.' : `${draft.length.toLocaleString()}/8,000 · Enter to send · Shift+Enter for a new line`}</small><button type="submit" disabled={!draft.trim() || busy || !providerEnabled || Boolean(pendingProposal)} aria-label="Send message"><Send size={17} /></button></div>
        </form>
      </aside>
    </div>
  )
}
