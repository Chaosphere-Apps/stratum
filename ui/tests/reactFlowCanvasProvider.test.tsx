import { act, fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

const flowState = vi.hoisted(() => ({ props: null as Record<string, unknown> | null }))

vi.mock('@xyflow/react', async () => {
  const React = await import('react')
  function ReactFlow(props: Record<string, unknown> & { children?: React.ReactNode }) {
    flowState.props = props
    const initialized = React.useRef(false)
    React.useEffect(() => {
      if (initialized.current) return
      initialized.current = true
      const onInit = props.onInit as ((instance: { screenToFlowPosition: (point: { x: number; y: number }) => { x: number; y: number } }) => void) | undefined
      onInit?.({ screenToFlowPosition: (point) => point })
    }, [])
    return <div data-testid="react-flow">{props.children}</div>
  }
  return {
    ReactFlow,
    ReactFlowProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    Background: () => <div data-testid="background" />,
    Controls: () => <div data-testid="controls" />,
    Handle: () => null,
    NodeToolbar: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    NodeResizer: () => null,
    Position: { Left: 'left', Right: 'right', Top: 'top', Bottom: 'bottom' },
    MarkerType: { ArrowClosed: 'arrowclosed' },
    BackgroundVariant: { Dots: 'dots' },
    ConnectionLineType: { SmoothStep: 'smoothstep' },
    ConnectionMode: { Loose: 'loose' },
    SelectionMode: { Partial: 'partial' },
    PanOnScrollMode: { Free: 'free' },
    applyNodeChanges: (changes: Array<{ type: string; id: string }>, nodes: Array<{ id: string }>) =>
      nodes.filter((node) => !changes.some((change) => change.type === 'remove' && change.id === node.id)),
    applyEdgeChanges: vi.fn(),
  }
})

import { createEmptyDesign } from '../src/designModel'
import { ReactFlowCanvasProvider } from '../src/canvas/ReactFlowCanvasProvider'
import type { DesignDocument } from '../src/types'

function designFixture(): DesignDocument {
  return {
    ...createEmptyDesign({ id: 'design-1', title: 'Checkout' }),
    components: [
      { id: 'client', shapeId: 'client', type: 'client.web', name: 'Client', purpose: '', owner: '', criticality: 'medium', metadata: { position: { x: 20, y: 30 } }, notes: [] },
      { id: 'service', shapeId: 'service', type: 'compute.service', name: 'Checkout API', purpose: '', owner: '', criticality: 'high', metadata: { position: { x: 300, y: 30 } }, notes: [] },
    ],
    connectors: [{
      id: 'edge-1', fromComponentId: 'client', toComponentId: 'service', type: 'synchronous', protocol: 'HTTPS',
      timeoutMs: 500, consistencyExpectation: '', notes: '', animated: false,
    }],
  }
}

function callback<T>(name: string) {
  return flowState.props?.[name] as T
}

describe('ReactFlowCanvasProvider', () => {
  it('propagates selection, valid connections, reconnections, and removals', () => {
    const onSelectionChange = vi.fn()
    const onConnectComponents = vi.fn()
    const onReconnectConnector = vi.fn()
    const onDeleteComponents = vi.fn()
    const onDeleteConnectors = vi.fn()
    render(<ReactFlowCanvasProvider design={designFixture()} onSelectionChange={onSelectionChange}
      onConnectComponents={onConnectComponents} onReconnectConnector={onReconnectConnector}
      onDeleteComponents={onDeleteComponents} onDeleteConnectors={onDeleteConnectors} />)

    callback<(selection: { nodes: Array<{ id: string }>; edges: Array<{ id: string }> }) => void>('onSelectionChange')({ nodes: [{ id: 'client' }], edges: [{ id: 'edge-1' }] })
    callback<(connection: { source: string; target: string }) => void>('onConnect')({ source: 'client', target: 'service' })
    callback<(connection: { source: string; target: string }) => void>('onConnect')({ source: 'client', target: 'client' })
    callback<(edge: { id: string }, connection: { source: string; target: string }) => void>('onReconnect')({ id: 'edge-1' }, { source: 'service', target: 'client' })
    callback<(changes: Array<{ type: string; id: string }>) => void>('onNodesChange')([{ type: 'remove', id: 'service' }])
    callback<(changes: Array<{ type: string; id: string }>) => void>('onEdgesChange')([{ type: 'remove', id: 'edge-1' }])

    expect(onSelectionChange).toHaveBeenCalledWith(['client'], ['edge-1'])
    expect(onConnectComponents).toHaveBeenCalledTimes(1)
    expect(onConnectComponents).toHaveBeenCalledWith('client', 'service')
    expect(onReconnectConnector).toHaveBeenCalledWith('edge-1', 'service', 'client')
    expect(onDeleteComponents).toHaveBeenCalledWith(['service'])
    expect(onDeleteConnectors).toHaveBeenCalledWith(['edge-1'])
  })

  it('drops catalog assets and components at centered cursor positions', () => {
    const onDropComponent = vi.fn()
    const onDropCatalogAsset = vi.fn()
    render(<ReactFlowCanvasProvider design={designFixture()} onDropComponent={onDropComponent} onDropCatalogAsset={onDropCatalogAsset} />)
    act(() => callback<(instance: { screenToFlowPosition: (point: { x: number; y: number }) => { x: number; y: number } }) => void>('onInit')({ screenToFlowPosition: (point) => point }))
    const onDrop = callback<(event: { preventDefault: () => void; stopPropagation: () => void; clientX: number; clientY: number; dataTransfer: { getData: (type: string) => string; dropEffect: string } }) => void>('onDrop')
    onDrop({ preventDefault: vi.fn(), stopPropagation: vi.fn(),
      clientX: 500, clientY: 300,
      dataTransfer: { getData: (type: string) => type === 'application/x-sde-component-type' ? 'compute.service' : '', dropEffect: '' },
    })
    expect(onDropComponent).toHaveBeenCalledWith('compute.service', { x: 385, y: 261 })

    onDrop({ preventDefault: vi.fn(), stopPropagation: vi.fn(),
      clientX: 600, clientY: 400,
      dataTransfer: { getData: (type: string) => type === 'application/x-stratum-catalog-asset-id' ? 'asset-1' : '', dropEffect: '' },
    })
    expect(onDropCatalogAsset).toHaveBeenCalledWith('asset-1', { x: 485, y: 361 })
  })

  it('blocks all canvas mutations in read-only mode', () => {
    const onConnectComponents = vi.fn()
    const onDeleteComponents = vi.fn()
    const onDeleteConnectors = vi.fn()
    const onDropComponent = vi.fn()
    render(<ReactFlowCanvasProvider design={designFixture()} readOnly onConnectComponents={onConnectComponents}
      onDeleteComponents={onDeleteComponents} onDeleteConnectors={onDeleteConnectors} onDropComponent={onDropComponent} />)

    callback<(connection: { source: string; target: string }) => void>('onConnect')({ source: 'client', target: 'service' })
    callback<(changes: Array<{ type: string; id: string }>) => void>('onNodesChange')([{ type: 'remove', id: 'service' }])
    callback<(changes: Array<{ type: string; id: string }>) => void>('onEdgesChange')([{ type: 'remove', id: 'edge-1' }])
    fireEvent.drop(document.querySelector('.react-flow-canvas') as HTMLElement, {
      dataTransfer: { getData: () => 'compute.service', dropEffect: '' }, clientX: 1, clientY: 1,
    })
    expect(onConnectComponents).not.toHaveBeenCalled()
    expect(onDeleteComponents).not.toHaveBeenCalled()
    expect(onDeleteConnectors).not.toHaveBeenCalled()
    expect(onDropComponent).not.toHaveBeenCalled()
  })

  it('offers duplicate and delete actions through the node context menu', () => {
    const onDuplicateComponent = vi.fn()
    const onDeleteComponents = vi.fn()
    const onSelectComponent = vi.fn()
    render(<ReactFlowCanvasProvider design={designFixture()} selectedComponentIds={['client', 'service']}
      onDuplicateComponent={onDuplicateComponent} onDeleteComponents={onDeleteComponents} onSelectComponent={onSelectComponent} />)
    const nodes = flowState.props?.nodes as Array<{ id: string; data: { component: { name: string } } }>
    act(() => callback<(event: { preventDefault: () => void; clientX: number; clientY: number }, node: typeof nodes[number]) => void>('onNodeContextMenu')(
      { preventDefault: vi.fn(), clientX: 100, clientY: 100 }, nodes.find((node) => node.id === 'client')!,
    ))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Duplicate' }))
    expect(onDuplicateComponent).toHaveBeenCalledWith('client')

    act(() => callback<(event: { preventDefault: () => void; clientX: number; clientY: number }, node: typeof nodes[number]) => void>('onNodeContextMenu')(
      { preventDefault: vi.fn(), clientX: 100, clientY: 100 }, nodes.find((node) => node.id === 'client')!,
    ))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Delete 2' }))
    expect(onDeleteComponents).toHaveBeenCalledWith(['client', 'service'])
  })
})
