export type ComponentType =
  | 'client.web'
  | 'edge.api_gateway'
  | 'compute.service'
  | 'data.sql_database'
  | 'data.redis'
  | 'messaging.queue'
  | 'data.object_store'
  | 'ai.llm'
  | 'external.api'
  | 'observability.telemetry'
  | 'security.control'
  | 'design.link'
  | 'note.sticky'
  | 'frame.cloud'

export type ConnectorType =
  | 'synchronous'
  | 'asynchronous_event'
  | 'batch_transfer'
  | 'cache_read'
  | 'cache_write'
  | 'model_call'
  | 'observability_signal'

export type Criticality = 'low' | 'medium' | 'high' | 'critical'
export type Unknownable<T extends string> = T | 'unknown'

export interface RequirementBrief {
  useCase: string
  functionalRequirements: string
  nonFunctionalRequirements: string
  targetRps: number | null
  availabilityRequirement: string
  sla: string
  problemStatement: string
  primaryActors: string
  trafficNotes: string
  consistencyNotes: string
  openQuestions: string
}

export interface RedisMetadata {
  usage: Unknownable<'cache' | 'session_store' | 'rate_limiter' | 'pub_sub' | 'lock_manager'>
  clusterMode: Unknownable<'enabled' | 'disabled'>
  replication: Unknownable<'none' | 'primary_replica' | 'multi_az' | 'cross_region'>
  persistence: Unknownable<'none' | 'rdb' | 'aof' | 'rdb_aof' | 'managed_default'>
  evictionPolicy: Unknownable<'noeviction' | 'allkeys_lru' | 'volatile_lru' | 'allkeys_lfu' | 'volatile_ttl'>
  consistencyExpectation: Unknownable<'best_effort_cache' | 'read_after_write' | 'strong_for_locking' | 'eventual'>
  failoverBehavior: Unknownable<'automatic' | 'manual' | 'data_loss_possible'>
  backupRestore: Unknownable<'configured' | 'not_required' | 'missing'>
  memoryLimitGb: number | null
  expectedQps: number | null
  hotKeyRisk: Unknownable<'low' | 'medium' | 'high'>
}

export interface LinkedDesignMetadata {
  workspaceId: string
  designId: string
  title: string
  access: 'private' | 'workspace' | 'public'
  updatedAt: string
}

export interface EnterpriseAssetRefMetadata {
  assetId: string
  name: string
  type: string
  owner: string
  criticality: string
  linkedAt: string
}

export type ComponentMetadata = {
  expectedQps?: number | null
  latencyBudgetMs?: number | null
  position?: {
    x: number
    y: number
  }
  parentFrameId?: string
  size?: {
    width: number
    height: number
  }
  notes?: string
  redis?: RedisMetadata
  linkedDesign?: LinkedDesignMetadata
  enterpriseAsset?: EnterpriseAssetRefMetadata
}

export interface DesignComponent {
  id: string
  shapeId: string
  type: ComponentType
  name: string
  purpose: string
  owner: string
  criticality: Criticality
  metadata: ComponentMetadata
  notes: DesignNote[]
}

export interface DesignConnector {
  id: string
  shapeId?: string
  fromComponentId: string
  toComponentId: string
  type: ConnectorType
  protocol: string
  timeoutMs: number | null
  consistencyExpectation: string
  notes: string
  animated: boolean
}

export interface DesignNote {
  id: string
  body: string
  tone: 'neutral' | 'risk' | 'decision' | 'question'
}

export interface DesignJourneyStep {
  id: string
  componentId?: string
  connectorId?: string
  title: string
  description: string
}

export interface DesignJourney {
  id: string
  title: string
  description: string
  entryComponentId?: string
  steps: DesignJourneyStep[]
  createdAt: string
  updatedAt: string
}

export interface DesignDocument {
  schemaVersion: 'sde-ui/v0.1'
  id: string
  title: string
  requirementBrief: RequirementBrief
  components: DesignComponent[]
  connectors: DesignConnector[]
  journeys: DesignJourney[]
  updatedAt: string
}
