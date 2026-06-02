import { useEffect, useState } from 'react'
import {
  Braces,
  ChevronsLeftRight,
  Database,
  FilePlus2,
  LayoutDashboard,
  MessageSquarePlus,
  Server,
} from 'lucide-react'
import { isPotentialCatalogMatch, normalizedCatalogLabel } from '../app/designUtils'
import type { BackendCatalogAsset } from '../backendApi'
import type { BackendDesign } from '../backendSync'
import { connectorTypeDescription, connectorTypeLabel, connectorTypeOptions } from '../canvas/connectorVisuals'
import { toExportableDesign } from '../designModel'
import type { DesignComponent, DesignConnector, DesignDocument, RedisMetadata } from '../types'
import { Field, NumberField, SelectField, TextareaField } from './FormFields'

export function ComponentInspector({
  component,
  currentDesignId,
  workspaceDesigns,
  catalogAssets,
  onChange,
  onDelete,
  onOpenLinkedDesign,
  onCreateCatalogAsset,
}: {
  component: DesignComponent
  currentDesignId: string
  workspaceDesigns: BackendDesign[]
  catalogAssets: BackendCatalogAsset[]
  onChange: (component: DesignComponent) => void
  onDelete: () => void
  onOpenLinkedDesign: (design: BackendDesign) => void
  onCreateCatalogAsset: (input: {
    name: string
    type: string
    owner?: string
    description?: string
    criticality?: string
    tags?: string[]
  }) => Promise<BackendCatalogAsset>
}) {
  const [catalogError, setCatalogError] = useState<string | null>(null)

  if (component.type === 'design.link') {
    return (
      <LinkedDesignInspector
        component={component}
        currentDesignId={currentDesignId}
        workspaceDesigns={workspaceDesigns}
        onChange={onChange}
        onDelete={onDelete}
        onOpenLinkedDesign={onOpenLinkedDesign}
      />
    )
  }

  const linkedAsset = component.metadata.enterpriseAsset
    ? catalogAssets.find((asset) => asset.id === component.metadata.enterpriseAsset?.assetId)
    : null
  const exactDuplicateCandidates = catalogAssets.filter(
    (asset) => normalizedCatalogLabel(asset.name) === normalizedCatalogLabel(component.name),
  )
  const duplicateCandidates = catalogAssets.filter((asset) => isPotentialCatalogMatch(component.name, asset.name))

  async function promoteToCatalog() {
    setCatalogError(null)
    try {
      const created = await onCreateCatalogAsset({
        name: component.name,
        type: component.type,
        owner: component.owner,
        description: component.purpose,
        criticality: component.criticality,
      })
      linkCatalogAsset(created)
    } catch (error) {
      setCatalogError(error instanceof Error ? error.message : 'Could not add component to catalog.')
    }
  }

  function linkCatalogAsset(asset: BackendCatalogAsset) {
    onChange({
      ...component,
      name: component.name.trim() ? component.name : asset.name,
      owner: component.owner.trim() ? component.owner : asset.owner,
      criticality: (asset.criticality || component.criticality) as DesignComponent['criticality'],
      metadata: {
        ...component.metadata,
        enterpriseAsset: {
          assetId: asset.id,
          name: asset.name,
          type: asset.type,
          owner: asset.owner,
          criticality: asset.criticality,
          linkedAt: new Date().toISOString(),
        },
      },
    })
  }

  function unlinkCatalogAsset() {
    const metadata = { ...component.metadata }
    delete metadata.enterpriseAsset
    onChange({ ...component, metadata })
  }

  return (
    <div className="inspector">
      <div className="inspector-heading">
        {component.type.startsWith('data.') ? <Database size={18} /> : <Server size={18} />}
        <div>
          <strong>{component.name}</strong>
          <span>{component.type}</span>
        </div>
        <button className="text-button danger" type="button" onClick={onDelete}>
          Delete
        </button>
      </div>

      <Field label="Name" value={component.name} onChange={(name) => onChange({ ...component, name })} />
      <div className="inspector-group enterprise-link-card">
        <div className="section-title">Enterprise Catalog</div>
        {component.metadata.enterpriseAsset ? (
          <div className="catalog-link-summary">
            <span className="shared-entity-badge">Shared entity</span>
            <strong>{linkedAsset?.name ?? component.metadata.enterpriseAsset.name}</strong>
            <small>
              {linkedAsset
                ? `${linkedAsset.usedInDesignCount} linked design${linkedAsset.usedInDesignCount === 1 ? '' : 's'}`
                : 'Catalog asset not visible or deleted'}
            </small>
            <button className="text-button" type="button" onClick={unlinkCatalogAsset}>
              Unlink from catalog
            </button>
          </div>
        ) : (
          <>
            {duplicateCandidates.length ? (
              <div className="catalog-suggestions">
                <strong>Possible existing asset</strong>
                {duplicateCandidates.map((asset) => (
                  <button className="catalog-suggestion" type="button" key={asset.id} onClick={() => linkCatalogAsset(asset)}>
                    <span>{asset.name}</span>
                    <small>
                      {asset.type} • {asset.usedInDesignCount} use{asset.usedInDesignCount === 1 ? '' : 's'}
                    </small>
                  </button>
                ))}
              </div>
            ) : null}
            <label className="field">
              <span>Link existing asset</span>
              <select
                value=""
                onChange={(event) => {
                  const asset = catalogAssets.find((item) => item.id === event.target.value)
                  if (asset) linkCatalogAsset(asset)
                }}
              >
                <option value="">Choose from catalog...</option>
                {catalogAssets.map((asset) => (
                  <option key={asset.id} value={asset.id}>
                    {asset.name} ({asset.type})
                  </option>
                ))}
              </select>
            </label>
            {catalogError ? <div className="admin-inline-error">{catalogError}</div> : null}
            <button
              className="command"
              type="button"
              disabled={!component.name.trim() || exactDuplicateCandidates.length > 0}
              onClick={() => void promoteToCatalog()}
              title={
                exactDuplicateCandidates.length
                  ? 'Link the existing catalog asset instead of creating a duplicate.'
                  : 'Create catalog asset'
              }
            >
              <FilePlus2 size={16} /> Add to catalog
            </button>
          </>
        )}
      </div>
      <Field label="Owner" value={component.owner} onChange={(owner) => onChange({ ...component, owner })} />
      <TextareaField
        label="Purpose"
        value={component.purpose}
        onChange={(purpose) => onChange({ ...component, purpose })}
      />
      <div className="inspector-group">
        <div className="section-title">Notes</div>
        <button
          className="command"
          onClick={() =>
            onChange({
              ...component,
              notes: [
                ...component.notes,
                {
                  id: `note_${crypto.randomUUID()}`,
                  body: '',
                  tone: 'neutral',
                },
              ],
            })
          }
        >
          <MessageSquarePlus size={16} /> Add note
        </button>
        {component.notes.map((note) => (
          <div className={`note-editor ${note.tone}`} key={note.id}>
            <select
              value={note.tone}
              onChange={(event) =>
                onChange({
                  ...component,
                  notes: component.notes.map((item) =>
                    item.id === note.id ? { ...item, tone: event.target.value as typeof note.tone } : item,
                  ),
                })
              }
            >
              <option value="neutral">Neutral</option>
              <option value="risk">Risk</option>
              <option value="decision">Decision</option>
              <option value="question">Question</option>
            </select>
            <textarea
              value={note.body}
              placeholder="Add context, risk, decision, or reviewer question..."
              onChange={(event) =>
                onChange({
                  ...component,
                  notes: component.notes.map((item) =>
                    item.id === note.id ? { ...item, body: event.target.value } : item,
                  ),
                })
              }
              rows={3}
            />
            <button
              className="text-button danger"
              onClick={() => onChange({ ...component, notes: component.notes.filter((item) => item.id !== note.id) })}
            >
              Remove
            </button>
          </div>
        ))}
      </div>
      <label className="field">
        <span>Criticality</span>
        <select
          value={component.criticality}
          onChange={(event) => onChange({ ...component, criticality: event.target.value as DesignComponent['criticality'] })}
        >
          <option value="low">Low</option>
          <option value="medium">Medium</option>
          <option value="high">High</option>
          <option value="critical">Critical</option>
        </select>
      </label>

      <div className="inspector-group">
        <div className="section-title">Evaluation</div>
        <NumberField
          label="Expected QPS"
          value={component.metadata.expectedQps ?? null}
          onChange={(expectedQps) => onChange({ ...component, metadata: { ...component.metadata, expectedQps } })}
        />
        <NumberField
          label="Latency budget ms"
          value={component.metadata.latencyBudgetMs ?? null}
          onChange={(latencyBudgetMs) => onChange({ ...component, metadata: { ...component.metadata, latencyBudgetMs } })}
        />
      </div>

      {component.type === 'data.redis' && <RedisInspector component={component} onChange={onChange} />}
    </div>
  )
}

function LinkedDesignInspector({
  component,
  currentDesignId,
  workspaceDesigns,
  onChange,
  onDelete,
  onOpenLinkedDesign,
}: {
  component: DesignComponent
  currentDesignId: string
  workspaceDesigns: BackendDesign[]
  onChange: (component: DesignComponent) => void
  onDelete: () => void
  onOpenLinkedDesign: (design: BackendDesign) => void
}) {
  const linkableDesigns = workspaceDesigns.filter((design) => design.id !== currentDesignId)
  const linkedDesign = component.metadata.linkedDesign
  const linkedBackendDesign = linkedDesign
    ? workspaceDesigns.find((design) => design.id === linkedDesign.designId && design.workspaceId === linkedDesign.workspaceId)
    : null
  const currentLinkedTitle = linkedBackendDesign
    ? linkedBackendDesign.name || linkedBackendDesign.document.title || 'Untitled design'
    : linkedDesign?.title

  useEffect(() => {
    if (!linkedDesign || !linkedBackendDesign) return
    const title = linkedBackendDesign.name || linkedBackendDesign.document.title || 'Untitled design'
    if (
      linkedDesign.title === title &&
      linkedDesign.access === linkedBackendDesign.access &&
      linkedDesign.updatedAt === linkedBackendDesign.updatedAt
    ) {
      return
    }
    onChange({
      ...component,
      metadata: {
        ...component.metadata,
        linkedDesign: {
          workspaceId: linkedBackendDesign.workspaceId,
          designId: linkedBackendDesign.id,
          title,
          access: linkedBackendDesign.access,
          updatedAt: linkedBackendDesign.updatedAt,
        },
      },
    })
  }, [component, linkedBackendDesign, linkedDesign, onChange])

  function updateLinkedDesign(designId: string) {
    if (!designId) {
      const metadata = { ...component.metadata }
      delete metadata.linkedDesign
      onChange({ ...component, metadata })
      return
    }
    const target = workspaceDesigns.find((design) => design.id === designId)
    if (!target) return
    const title = target.name || target.document.title || 'Untitled design'
    onChange({
      ...component,
      name: component.name === 'Linked Design' || component.name.trim() === '' ? title : component.name,
      metadata: {
        ...component.metadata,
        linkedDesign: {
          workspaceId: target.workspaceId,
          designId: target.id,
          title,
          access: target.access,
          updatedAt: target.updatedAt,
        },
      },
    })
  }

  return (
    <div className="inspector linked-design-inspector">
      <div className="inspector-heading">
        <LayoutDashboard size={18} />
        <div>
          <strong>{component.name || 'Linked Design'}</strong>
          <span>Reference to another design</span>
        </div>
        <button className="text-button danger" type="button" onClick={onDelete}>
          Delete
        </button>
      </div>

      <Field label="Label" value={component.name} onChange={(name) => onChange({ ...component, name })} />

      <div className="inspector-group">
        <div className="section-title">Target</div>
        <label className="field">
          <span>Target design</span>
          <select value={linkedDesign?.designId ?? ''} onChange={(event) => updateLinkedDesign(event.target.value)}>
            <option value="">Choose a design...</option>
            {linkableDesigns.map((design) => (
              <option key={design.id} value={design.id}>
                {design.name || design.document.title || 'Untitled design'}
              </option>
            ))}
          </select>
        </label>
        {linkedDesign ? (
          <>
            <Field label="Title" value={currentLinkedTitle ?? 'Unavailable design'} onChange={() => undefined} disabled />
            <Field
              label="Access"
              value={linkedBackendDesign?.access ?? `${linkedDesign.access} (not visible)`}
              onChange={() => undefined}
              disabled
            />
            <Field
              label="Last synced"
              value={new Date(linkedBackendDesign?.updatedAt ?? linkedDesign.updatedAt).toLocaleString()}
              onChange={() => undefined}
              disabled
            />
            <button
              className="command"
              type="button"
              disabled={!linkedBackendDesign}
              onClick={() => linkedBackendDesign && onOpenLinkedDesign(linkedBackendDesign)}
            >
              <LayoutDashboard size={16} /> Open linked design
            </button>
          </>
        ) : (
          <div className="analysis-empty compact-empty">
            <strong>No target selected</strong>
            <span>Links are limited to designs visible in this workspace.</span>
          </div>
        )}
      </div>
    </div>
  )
}

export function ConnectorInspector({
  connector,
  onChange,
  onDelete,
}: {
  connector: DesignConnector
  onChange: (connector: DesignConnector) => void
  onDelete: () => void
}) {
  return (
    <div className="inspector">
      <div className="inspector-heading">
        <ChevronsLeftRight size={18} />
        <div>
          <strong>{connector.type.replaceAll('_', ' ')}</strong>
          <span>Connector</span>
        </div>
        <button className="text-button danger" type="button" onClick={onDelete}>
          Delete
        </button>
      </div>

      <div className="connector-type-picker">
        <div className="section-title">Flow type</div>
        <div className="connector-type-grid">
          {connectorTypeOptions.map((type) => (
            <button
              className={connector.type === type ? 'active' : ''}
              type="button"
              key={type}
              onClick={() => onChange({ ...connector, type })}
            >
              <strong>{connectorTypeLabel(type)}</strong>
              <span>{connectorTypeDescription(type)}</span>
            </button>
          ))}
        </div>
      </div>
      <Field label="Protocol" value={connector.protocol} onChange={(protocol) => onChange({ ...connector, protocol })} />
      <NumberField label="Timeout ms" value={connector.timeoutMs} onChange={(timeoutMs) => onChange({ ...connector, timeoutMs })} />
      <Field
        label="Consistency"
        value={connector.consistencyExpectation}
        onChange={(consistencyExpectation) => onChange({ ...connector, consistencyExpectation })}
      />
      <TextareaField label="Notes" value={connector.notes} onChange={(notes) => onChange({ ...connector, notes })} />
      <label className="field checkbox-field">
        <span>Animated flow</span>
        <input
          type="checkbox"
          checked={connector.animated}
          onChange={(event) => onChange({ ...connector, animated: event.target.checked })}
        />
      </label>
    </div>
  )
}

function RedisInspector({
  component,
  onChange,
}: {
  component: DesignComponent
  onChange: (component: DesignComponent) => void
}) {
  const redis = component.metadata.redis
  if (!redis) return null
  const currentRedis: RedisMetadata = redis

  function updateRedis(patch: Partial<RedisMetadata>) {
    onChange({ ...component, metadata: { ...component.metadata, redis: { ...currentRedis, ...patch } } })
  }

  return (
    <div className="inspector-group">
      <div className="section-title">Redis Details</div>
      <SelectField
        label="Usage"
        value={redis.usage}
        values={['unknown', 'cache', 'session_store', 'rate_limiter', 'pub_sub', 'lock_manager']}
        onChange={(usage) => updateRedis({ usage: usage as typeof redis.usage })}
      />
      <SelectField
        label="Cluster mode"
        value={redis.clusterMode}
        values={['unknown', 'enabled', 'disabled']}
        onChange={(clusterMode) => updateRedis({ clusterMode: clusterMode as typeof redis.clusterMode })}
      />
      <SelectField
        label="Replication"
        value={redis.replication}
        values={['unknown', 'none', 'primary_replica', 'multi_az', 'cross_region']}
        onChange={(replication) => updateRedis({ replication: replication as typeof redis.replication })}
      />
      <SelectField
        label="Persistence"
        value={redis.persistence}
        values={['unknown', 'none', 'rdb', 'aof', 'rdb_aof', 'managed_default']}
        onChange={(persistence) => updateRedis({ persistence: persistence as typeof redis.persistence })}
      />
      <SelectField
        label="Eviction policy"
        value={redis.evictionPolicy}
        values={['unknown', 'noeviction', 'allkeys_lru', 'volatile_lru', 'allkeys_lfu', 'volatile_ttl']}
        onChange={(evictionPolicy) => updateRedis({ evictionPolicy: evictionPolicy as typeof redis.evictionPolicy })}
      />
      <SelectField
        label="Consistency"
        value={redis.consistencyExpectation}
        values={['unknown', 'best_effort_cache', 'read_after_write', 'strong_for_locking', 'eventual']}
        onChange={(consistencyExpectation) =>
          updateRedis({ consistencyExpectation: consistencyExpectation as typeof redis.consistencyExpectation })
        }
      />
      <SelectField
        label="Failover"
        value={redis.failoverBehavior}
        values={['unknown', 'automatic', 'manual', 'data_loss_possible']}
        onChange={(failoverBehavior) => updateRedis({ failoverBehavior: failoverBehavior as typeof redis.failoverBehavior })}
      />
      <SelectField
        label="Backup/restore"
        value={redis.backupRestore}
        values={['unknown', 'configured', 'not_required', 'missing']}
        onChange={(backupRestore) => updateRedis({ backupRestore: backupRestore as typeof redis.backupRestore })}
      />
      <NumberField label="Memory limit GB" value={redis.memoryLimitGb} onChange={(memoryLimitGb) => updateRedis({ memoryLimitGb })} />
      <NumberField label="Redis QPS" value={redis.expectedQps} onChange={(expectedQps) => updateRedis({ expectedQps })} />
      <SelectField
        label="Hot key risk"
        value={redis.hotKeyRisk}
        values={['unknown', 'low', 'medium', 'high']}
        onChange={(hotKeyRisk) => updateRedis({ hotKeyRisk: hotKeyRisk as typeof redis.hotKeyRisk })}
      />
    </div>
  )
}

export function StructuredView({ design }: { design: DesignDocument }) {
  return (
    <div className="inspector structured">
      <div className="inspector-heading">
        <Braces size={18} />
        <div>
          <strong>Structured JSON</strong>
          <span>Semantic model plus React Flow layout</span>
        </div>
      </div>
      <pre>{JSON.stringify(toExportableDesign(design), null, 2)}</pre>
    </div>
  )
}
