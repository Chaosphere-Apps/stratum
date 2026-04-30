import type { ComponentType, RedisMetadata } from './types'

export interface CatalogItem {
  type: ComponentType
  label: string
  shortLabel: string
  description: string
  color: string
}

export const componentCatalog: CatalogItem[] = [
  {
    type: 'client.web',
    label: 'Web Client',
    shortLabel: 'Web',
    description: 'Browser or customer-facing frontend.',
    color: '#2563eb',
  },
  {
    type: 'edge.api_gateway',
    label: 'API Gateway',
    shortLabel: 'API GW',
    description: 'Entry point, routing, auth, rate limits.',
    color: '#0f766e',
  },
  {
    type: 'compute.service',
    label: 'Service',
    shortLabel: 'Svc',
    description: 'Application service or API.',
    color: '#7c3aed',
  },
  {
    type: 'data.sql_database',
    label: 'SQL Database',
    shortLabel: 'SQL',
    description: 'Relational database or transactional store.',
    color: '#b45309',
  },
  {
    type: 'data.redis',
    label: 'Redis',
    shortLabel: 'Redis',
    description: 'Cache, sessions, rate limits, locks, pub/sub.',
    color: '#dc2626',
  },
  {
    type: 'messaging.queue',
    label: 'Queue',
    shortLabel: 'Queue',
    description: 'Async buffering and background processing.',
    color: '#0891b2',
  },
  {
    type: 'data.object_store',
    label: 'Object Store',
    shortLabel: 'Obj',
    description: 'Blob storage for files, exports, backups.',
    color: '#4d7c0f',
  },
  {
    type: 'ai.llm',
    label: 'LLM',
    shortLabel: 'LLM',
    description: 'Model call, agent, classifier, or generation step.',
    color: '#c026d3',
  },
  {
    type: 'external.api',
    label: 'External API',
    shortLabel: 'Ext',
    description: 'Third-party or partner system.',
    color: '#475569',
  },
  {
    type: 'observability.telemetry',
    label: 'Observability',
    shortLabel: 'Obs',
    description: 'Metrics, logs, traces, alerting, audit events.',
    color: '#16a34a',
  },
  {
    type: 'security.control',
    label: 'Security Control',
    shortLabel: 'Sec',
    description: 'Auth, policy filter, redaction, WAF, KMS.',
    color: '#be123c',
  },
  {
    type: 'frame.cloud',
    label: 'Cloud Frame',
    shortLabel: 'Cloud',
    description: 'Boundary for cloud, region, account, VPC, or environment.',
    color: '#64748b',
  },
  {
    type: 'note.sticky',
    label: 'Sticky Note',
    shortLabel: 'Note',
    description: 'Visible context, risk, decision, or open question.',
    color: '#facc15',
  },
]

export function getCatalogItem(type: ComponentType) {
  return componentCatalog.find((item) => item.type === type) ?? componentCatalog[2]
}

export function defaultRedisMetadata(): RedisMetadata {
  return {
    usage: 'unknown',
    clusterMode: 'unknown',
    replication: 'unknown',
    persistence: 'unknown',
    evictionPolicy: 'unknown',
    consistencyExpectation: 'unknown',
    failoverBehavior: 'unknown',
    backupRestore: 'unknown',
    memoryLimitGb: null,
    expectedQps: null,
    hotKeyRisk: 'unknown',
  }
}
