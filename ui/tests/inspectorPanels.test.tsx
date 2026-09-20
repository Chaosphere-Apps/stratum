import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { BackendCatalogAsset } from '../src/backendApi'
import type { BackendDesign } from '../src/backendSync'
import { createComponent, createEmptyDesign } from '../src/designModel'
import type { DesignComponent, DesignConnector } from '../src/types'
import { ComponentInspector, ConnectorInspector, StructuredView } from '../src/components/InspectorPanels'

const activeAsset: BackendCatalogAsset = {
  id: 'asset-api', name: 'Payments API', normalizedName: 'payments api', kind: 'component', type: 'compute.service',
  owner: 'Payments', description: 'Canonical API', criticality: 'high', status: 'active', aliases: [],
  replacementAssetId: '', updateMessage: '', tags: [], createdBy: 'admin', createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z', usedInDesignCount: 3,
}

const retiredAsset: BackendCatalogAsset = {
  ...activeAsset, id: 'asset-old', name: 'Legacy API', normalizedName: 'legacy api', status: 'deprecated',
  replacementAssetId: activeAsset.id, updateMessage: 'Move to Payments API.', usedInDesignCount: 7,
}

function component(type: DesignComponent['type'] = 'compute.service', name = 'Checkout Service') {
  return createComponent({ shapeId: 'shape-one', type, name })
}

function renderComponentInspector(item: DesignComponent, overrides: Partial<React.ComponentProps<typeof ComponentInspector>> = {}) {
  const props: React.ComponentProps<typeof ComponentInspector> = {
    component: item,
    currentDesignId: 'design-current',
    workspaceDesigns: [],
    catalogAssets: [activeAsset, retiredAsset],
    onChange: vi.fn(),
    onDelete: vi.fn(),
    onOpenLinkedDesign: vi.fn(),
    onCreateCatalogAsset: vi.fn().mockResolvedValue(activeAsset),
    ...overrides,
  }
  return { ...render(<ComponentInspector {...props} />), props }
}

describe('ComponentInspector', () => {
  it('edits component identity, evaluation metadata, notes, and deletion', () => {
    const item = component()
    const { props } = renderComponentInspector(item)

    fireEvent.change(screen.getByRole('textbox', { name: 'Name' }), { target: { value: 'Checkout API' } })
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({ name: 'Checkout API' }))
    fireEvent.change(screen.getByRole('textbox', { name: 'Owner' }), { target: { value: 'Checkout' } })
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({ owner: 'Checkout' }))
    fireEvent.change(screen.getByRole('textbox', { name: 'Purpose' }), { target: { value: 'Accept payments' } })
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({ purpose: 'Accept payments' }))
    fireEvent.change(screen.getByRole('combobox', { name: 'Criticality' }), { target: { value: 'critical' } })
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({ criticality: 'critical' }))
    fireEvent.change(screen.getByRole('spinbutton', { name: 'Expected QPS' }), { target: { value: '2500' } })
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({ metadata: expect.objectContaining({ expectedQps: 2500 }) }))
    fireEvent.click(screen.getByRole('button', { name: 'Add note' }))
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({ notes: [expect.objectContaining({ body: '', tone: 'neutral' })] }))
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(props.onDelete).toHaveBeenCalledTimes(1)
  })

  it('links suggested catalog assets and blocks exact duplicate creation', () => {
    const exact = component('compute.service', 'Payments API')
    const { props, rerender } = renderComponentInspector(exact)
    const addButton = screen.getByRole('button', { name: 'Add to catalog' }) as HTMLButtonElement
    expect(addButton.disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: /Payments API/i }))
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({
      metadata: expect.objectContaining({ enterpriseAsset: expect.objectContaining({ assetId: activeAsset.id }) }),
    }))

    const linked = { ...exact, metadata: { enterpriseAsset: {
      assetId: retiredAsset.id, name: retiredAsset.name, kind: retiredAsset.kind, type: retiredAsset.type,
      owner: retiredAsset.owner, criticality: retiredAsset.criticality, status: retiredAsset.status,
      replacementAssetId: retiredAsset.replacementAssetId, updateMessage: retiredAsset.updateMessage,
      linkedAt: '2026-07-01T00:00:00Z',
    } } }
    rerender(<ComponentInspector {...props} component={linked} />)
    expect(screen.getByRole('alert')).toBeTruthy()
    expect(screen.getByText('Move to Payments API.')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Unlink from catalog' }))
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({ metadata: {} }))
  })

  it('promotes a unique component to the catalog and links the result', async () => {
    const item = component('compute.service', 'Risk Engine')
    const onCreateCatalogAsset = vi.fn().mockResolvedValue({ ...activeAsset, id: 'asset-risk', name: 'Risk Engine' })
    const { props } = renderComponentInspector(item, { onCreateCatalogAsset, catalogAssets: [] })

    fireEvent.click(screen.getByRole('button', { name: 'Add to catalog' }))
    await vi.waitFor(() => expect(onCreateCatalogAsset).toHaveBeenCalledWith(expect.objectContaining({ name: 'Risk Engine' })))
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({
      metadata: expect.objectContaining({ enterpriseAsset: expect.objectContaining({ assetId: 'asset-risk' }) }),
    }))
  })

  it('edits Redis-specific architecture metadata', () => {
    const item = component('data.redis', 'Redis')
    const { props } = renderComponentInspector(item, { catalogAssets: [] })

    fireEvent.change(screen.getByRole('combobox', { name: 'Usage' }), { target: { value: 'rate_limiter' } })
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({
      metadata: expect.objectContaining({ redis: expect.objectContaining({ usage: 'rate_limiter' }) }),
    }))
    fireEvent.change(screen.getByRole('spinbutton', { name: 'Memory limit GB' }), { target: { value: '8' } })
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({
      metadata: expect.objectContaining({ redis: expect.objectContaining({ memoryLimitGb: 8 }) }),
    }))
  })

  it('selects and opens a linked design without exposing unrelated component fields', () => {
    const target: BackendDesign = {
      id: 'design-target', workspaceId: 'workspace-one', name: 'Ledger', access: 'workspace', title: 'Ledger',
      document: createEmptyDesign({ id: 'design-target', title: 'Ledger' }), versionNumber: 1, updatedAt: '2026-07-10T00:00:00Z',
    }
    const item = component('design.link', 'Linked Design')
    const { props, rerender } = renderComponentInspector(item, { workspaceDesigns: [target] })

    expect(screen.queryByRole('textbox', { name: 'Owner' })).toBeNull()
    fireEvent.change(screen.getByRole('combobox', { name: 'Target design' }), { target: { value: target.id } })
    expect(props.onChange).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Ledger', metadata: expect.objectContaining({ linkedDesign: expect.objectContaining({ designId: target.id }) }),
    }))

    const linked = { ...item, name: 'Ledger', metadata: { linkedDesign: {
      workspaceId: target.workspaceId, designId: target.id, title: target.name, access: target.access, updatedAt: target.updatedAt,
    } } }
    rerender(<ComponentInspector {...props} component={linked} />)
    fireEvent.click(screen.getByRole('button', { name: 'Open linked design' }))
    expect(props.onOpenLinkedDesign).toHaveBeenCalledWith(target)
  })
})

describe('ConnectorInspector', () => {
  it('edits flow semantics and deletes the connector', () => {
    const connector: DesignConnector = {
      id: 'edge-one', fromComponentId: 'a', toComponentId: 'b', type: 'synchronous', protocol: 'HTTP',
      timeoutMs: 1000, consistencyExpectation: 'read-after-write', notes: '', animated: false,
    }
    const onChange = vi.fn()
    const onDelete = vi.fn()
    render(<ConnectorInspector connector={connector} onChange={onChange} onDelete={onDelete} />)

    fireEvent.click(screen.getByRole('button', { name: /Async event/i }))
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ type: 'asynchronous_event' }))
    fireEvent.change(screen.getByRole('textbox', { name: 'Protocol' }), { target: { value: 'gRPC' } })
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ protocol: 'gRPC' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Animated flow' }))
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ animated: true }))
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(onDelete).toHaveBeenCalledTimes(1)
  })
})

describe('StructuredView', () => {
  it('renders the exportable semantic model and canvas provider', () => {
    const design = createEmptyDesign({ id: 'design-one', title: 'Checkout' })
    render(<StructuredView design={design} />)
    const output = screen.getByText((_, element) => element?.tagName === 'PRE')
    expect(output.textContent).toContain('"provider": "react-flow"')
    expect(output.textContent).toContain('"title": "Checkout"')
  })
})
