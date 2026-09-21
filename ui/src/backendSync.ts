import { useCallback, useEffect, useRef, useState } from 'react'
import type { DesignDocument } from './types'

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected'

export interface BackendWorkspace {
  id: string
  name: string
  ownerId?: string
  effectiveAccess?: 'read' | 'edit'
}

export interface BackendDesign {
  id: string
  workspaceId: string
  name: string
  access: 'private' | 'workspace' | 'public'
  title: string
  document: DesignDocument
  documentRevision: string
  canvasSnapshot?: unknown
  versionNumber?: number
  updatedAt: string
  effectiveAccess?: 'read' | 'edit'
}

interface Envelope<T = unknown> {
  type: string
  requestId?: string
  payload?: T
  error?: {
    code: string
    message: string
  }
}

interface WorkspaceSnapshotPayload {
  snapshot: {
    workspace: {
      id: string
      name: string
    }
    designs: BackendDesign[]
  }
}

interface DesignUpdatedPayload {
  design: BackendDesign
}

interface DesignConflictPayload {
  design: BackendDesign
}

export interface CollaborationPresence {
  userId: string
  displayName: string
  role: string
  designId?: string
  sessionCount: number
}

interface PresenceUpdatedPayload {
  members: CollaborationPresence[]
}

interface UseBackendDesignSyncOptions {
  workspaceId: string
  selectedDesignId?: string | null
  enabled?: boolean
  onRemoteDesign: (design: BackendDesign, source: 'snapshot' | 'remote') => void
  onDesignSynced?: (design: BackendDesign) => void
  onConflict?: (design: BackendDesign) => void
}

export function useBackendDesignSync({ workspaceId, selectedDesignId, enabled = true, onRemoteDesign, onDesignSynced, onConflict }: UseBackendDesignSyncOptions) {
  const [status, setStatus] = useState<ConnectionStatus>(enabled ? 'connecting' : 'disconnected')
  const [workspace, setWorkspace] = useState<BackendWorkspace | null>(null)
  const [designs, setDesigns] = useState<BackendDesign[]>([])
  const [presence, setPresence] = useState<CollaborationPresence[]>([])
  const socketRef = useRef<WebSocket | null>(null)
  const pendingRequestIds = useRef(new Set<string>())
  const serverRevisions = useRef(new Map<string, string>())
  const remoteHandlerRef = useRef(onRemoteDesign)
  const syncedHandlerRef = useRef(onDesignSynced)
  const conflictHandlerRef = useRef(onConflict)
  const hydratedRef = useRef(false)

  useEffect(() => {
    remoteHandlerRef.current = onRemoteDesign
    syncedHandlerRef.current = onDesignSynced
    conflictHandlerRef.current = onConflict
  }, [onConflict, onDesignSynced, onRemoteDesign])

  useEffect(() => {
    if (!enabled) {
      socketRef.current?.close()
      socketRef.current = null
      hydratedRef.current = false
      pendingRequestIds.current.clear()
      setStatus('disconnected')
      setWorkspace(null)
      setDesigns([])
      setPresence([])
      return
    }

    let closed = false
    let reconnectTimer: number | undefined

    function connect() {
      if (closed) return
      hydratedRef.current = false
      setDesigns([])
      setStatus('connecting')

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const sameOriginWs = `${protocol}//${window.location.host}`
      const devWs = `ws://${window.location.hostname || '127.0.0.1'}:8081`
      const port = window.location.port
      const wsOrigin = import.meta.env.VITE_BACKEND_WS_ORIGIN || (port.startsWith('517') || port === '4173' ? devWs : sameOriginWs)
      const wsUrl = `${wsOrigin}/ws?workspaceId=${encodeURIComponent(workspaceId)}${selectedDesignId ? `&designId=${encodeURIComponent(selectedDesignId)}` : ''}`
      const socket = new WebSocket(wsUrl)
      socketRef.current = socket

      socket.onopen = () => {
        setStatus('connected')
      }

      socket.onmessage = (event) => {
        const envelope = JSON.parse(event.data) as Envelope
        if (envelope.type === 'workspace.snapshot') {
          const payload = envelope.payload as WorkspaceSnapshotPayload
          setWorkspace(payload.snapshot.workspace)
          setDesigns(payload.snapshot.designs)
          serverRevisions.current = new Map(payload.snapshot.designs.map((design) => [design.id, design.documentRevision]))
          const designToHydrate =
            (selectedDesignId ? payload.snapshot.designs.find((design) => design.id === selectedDesignId) : null) ??
            payload.snapshot.designs[0]
          if (designToHydrate && !hydratedRef.current) {
            hydratedRef.current = true
            remoteHandlerRef.current(designToHydrate, 'snapshot')
          }
          return
        }

        if (envelope.type === 'design.updated') {
          const payload = envelope.payload as DesignUpdatedPayload
          serverRevisions.current.set(payload.design.id, payload.design.documentRevision)
          setDesigns((current) => {
            const next = current.filter((design) => design.id !== payload.design.id)
            return [payload.design, ...next].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt))
          })
          if (envelope.requestId && pendingRequestIds.current.has(envelope.requestId)) {
            pendingRequestIds.current.delete(envelope.requestId)
            syncedHandlerRef.current?.(payload.design)
            return
          }
          remoteHandlerRef.current(payload.design, 'remote')
          return
        }

        if (envelope.type === 'presence.updated') {
          const payload = envelope.payload as PresenceUpdatedPayload
          setPresence((payload.members ?? []).filter((member) => !selectedDesignId || member.designId === selectedDesignId))
          return
        }

        if (envelope.type === 'error') {
          if (envelope.requestId) pendingRequestIds.current.delete(envelope.requestId)
          if (envelope.error?.code === 'design_conflict' && envelope.payload) {
            const payload = envelope.payload as DesignConflictPayload
            serverRevisions.current.set(payload.design.id, payload.design.documentRevision)
            conflictHandlerRef.current?.(payload.design)
            return
          }
          console.warn('Backend WebSocket error', envelope.error)
        }
      }

      socket.onclose = () => {
        if (socketRef.current === socket) {
          socketRef.current = null
        }
        pendingRequestIds.current.clear()
        setPresence([])
        setStatus('disconnected')
        if (!closed) {
          reconnectTimer = window.setTimeout(connect, 1500)
        }
      }

      socket.onerror = () => {
        setStatus('disconnected')
      }
    }

    connect()

    return () => {
      closed = true
      if (reconnectTimer) window.clearTimeout(reconnectTimer)
      socketRef.current?.close()
    }
  }, [enabled, selectedDesignId, workspaceId])

  const saveDesign = useCallback((design: DesignDocument, baseRevision?: string) => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN || pendingRequestIds.current.size > 0) {
      return false
    }

    const requestId = crypto.randomUUID()
    pendingRequestIds.current.add(requestId)
    const expectedRevision = baseRevision || serverRevisions.current.get(design.id)
    if (!expectedRevision) {
      pendingRequestIds.current.delete(requestId)
      return false
    }
    socket.send(
      JSON.stringify({
        type: 'design.upsert',
        requestId,
        payload: {
          design,
          baseRevision: expectedRevision,
        },
      }),
    )
    return true
  }, [])

  return {
    status,
    workspace,
    designs,
    presence,
    saveDesign,
  }
}
