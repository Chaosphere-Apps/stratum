import type { ConnectorType } from '../types'

export type ConnectorTone = 'request' | 'async' | 'batch' | 'read' | 'write' | 'ai' | 'signal'

export type ConnectorVisualProfile = {
  tone: ConnectorTone
  label: string
  stroke: string
  marker: string
  dash?: string
  shouldAnimate: boolean
}

const connectorProfiles: Record<ConnectorType, ConnectorVisualProfile> = {
  synchronous: {
    tone: 'request',
    label: 'Sync request',
    stroke: '#111827',
    marker: '#111827',
    shouldAnimate: false,
  },
  asynchronous_event: {
    tone: 'async',
    label: 'Async event',
    stroke: '#0891b2',
    marker: '#0891b2',
    dash: '9 7',
    shouldAnimate: true,
  },
  batch_transfer: {
    tone: 'batch',
    label: 'Batch transfer',
    stroke: '#475569',
    marker: '#475569',
    dash: '14 9',
    shouldAnimate: false,
  },
  cache_read: {
    tone: 'read',
    label: 'Cache read',
    stroke: '#16a34a',
    marker: '#16a34a',
    dash: '4 5',
    shouldAnimate: false,
  },
  cache_write: {
    tone: 'write',
    label: 'Cache write',
    stroke: '#d97706',
    marker: '#d97706',
    dash: '7 5',
    shouldAnimate: false,
  },
  model_call: {
    tone: 'ai',
    label: 'Model call',
    stroke: '#9333ea',
    marker: '#9333ea',
    shouldAnimate: true,
  },
  observability_signal: {
    tone: 'signal',
    label: 'Telemetry',
    stroke: '#059669',
    marker: '#059669',
    dash: '2 7',
    shouldAnimate: false,
  },
}

export function connectorVisualProfile(type: ConnectorType) {
  return connectorProfiles[type] ?? connectorProfiles.synchronous
}

export function connectorTypeLabel(type: ConnectorType) {
  return connectorVisualProfile(type).label
}

export function connectorTypeDescription(type: ConnectorType) {
  switch (type) {
    case 'synchronous':
      return 'Direct request/response path. Use when caller waits for completion.'
    case 'asynchronous_event':
      return 'Queued or emitted event. Use when work continues outside the caller path.'
    case 'batch_transfer':
      return 'Scheduled or bulk movement between stores/services.'
    case 'cache_read':
      return 'Read path to a cache or derived store.'
    case 'cache_write':
      return 'Write, invalidation, or warm-up path into a cache.'
    case 'model_call':
      return 'LLM or model inference call with prompt/response semantics.'
    case 'observability_signal':
      return 'Metrics, logs, traces, audit, or security signals.'
    default:
      return 'Architecture connector.'
  }
}

export const connectorTypeOptions: ConnectorType[] = [
  'synchronous',
  'asynchronous_event',
  'batch_transfer',
  'cache_read',
  'cache_write',
  'model_call',
  'observability_signal',
]
