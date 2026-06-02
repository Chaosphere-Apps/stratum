import type { ConnectorType, DesignConnector, DesignJourneyStep } from '../types'

export type JourneyStepKind = NonNullable<DesignJourneyStep['kind']>

export function inferJourneyStepKind(
  connector?: Pick<DesignConnector, 'type' | 'fromComponentId' | 'toComponentId'> | null,
  visitedComponentIds: Set<string> = new Set(),
): JourneyStepKind {
  if (!connector) return 'component'
  if (connector.fromComponentId && visitedComponentIds.has(connector.toComponentId)) return 'callback'
  if (connector.type === 'asynchronous_event') return 'async'
  if (connector.type === 'batch_transfer') return 'batch'
  if (connector.type === 'observability_signal') return 'signal'
  return 'request'
}

export function journeyStepKindLabel(kind?: DesignJourneyStep['kind'], connectorType?: ConnectorType) {
  if (kind === 'async' || connectorType === 'asynchronous_event') return 'Async'
  if (kind === 'callback') return 'Callback'
  if (kind === 'batch' || connectorType === 'batch_transfer') return 'Batch'
  if (kind === 'signal' || connectorType === 'observability_signal') return 'Signal'
  if (kind === 'decision') return 'Decision'
  if (kind === 'component') return 'Component'
  return 'Request'
}

export function journeyStepSummary(kind?: DesignJourneyStep['kind'], connectorType?: ConnectorType) {
  const label = journeyStepKindLabel(kind, connectorType)
  switch (label) {
    case 'Async':
      return 'Fire-and-continue path. Useful for queues, events, fanout, and retry review.'
    case 'Callback':
      return 'Reverse or returning path. Useful for webhooks, acknowledgements, and cycles.'
    case 'Batch':
      return 'Offline or scheduled movement. Review freshness, idempotency, and recovery.'
    case 'Signal':
      return 'Telemetry or audit path. Review observability and security guarantees.'
    case 'Component':
      return 'Focused stop on a system component.'
    default:
      return 'Synchronous request path.'
  }
}
