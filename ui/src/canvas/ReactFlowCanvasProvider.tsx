import { useMemo, type CSSProperties } from 'react'
import {
  Brain,
  Cloud,
  Copy,
  Database,
  Edit3,
  FileText,
  Globe2,
  HardDrive,
  StickyNote,
  Trash2,
  Network,
  RadioTower,
  Rows3,
  Server,
  Shield,
  Zap,
  type LucideIcon,
} from 'lucide-react'
import {
  Background,
  BackgroundVariant,
  ConnectionLineType,
  ConnectionMode,
  Controls,
  Handle,
  MarkerType,
  NodeToolbar,
  NodeResizer,
  PanOnScrollMode,
  Position,
  ReactFlow,
  ReactFlowProvider,
  SelectionMode,
  applyEdgeChanges,
  applyNodeChanges,
  type Connection,
  type Edge,
  type EdgeChange,
  type Node,
  type NodeChange,
  type NodeProps,
  type OnEdgesChange,
  type OnNodeDrag,
  type OnNodesChange,
  type OnReconnect,
  type ResizeParams,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { getCatalogItem } from '../catalog'
import type { DesignComponent } from '../types'
import type { ReactFlowPreviewProps } from './types'

type ArchitectureNodeData = {
  component: DesignComponent
  selected: boolean
  readOnly: boolean
  inTraversal: boolean
  activeTraversal: boolean
  dimmedByTraversal: boolean
  onResizeEnd?: (componentId: string, size: { width: number; height: number }) => void
  onRequestDelete?: (componentId: string) => void
  onRequestDuplicate?: (componentId: string) => void
  onRequestSelect?: (componentId: string) => void
}

function ArchitectureNode({ data, id }: NodeProps<Node<ArchitectureNodeData>>) {
  const item = getCatalogItem(data.component.type)

  if (data.component.type === 'frame.cloud') {
    return (
      <div
        className={[
          'rf-cloud-frame',
          data.selected ? 'selected' : '',
          data.inTraversal ? 'in-traversal' : '',
          data.activeTraversal ? 'active-traversal' : '',
          data.dimmedByTraversal ? 'dimmed-by-traversal' : '',
        ].filter(Boolean).join(' ')}
        style={{ '--node-color': item.color } as CSSProperties}
      >
        <NodeResizer
          isVisible={data.selected && !data.readOnly}
          minWidth={320}
          minHeight={220}
          handleClassName="rf-resize-handle"
          lineClassName="rf-resize-line"
          onResizeEnd={(_, params: ResizeParams) =>
            data.onResizeEnd?.(id, { width: params.width, height: params.height })
          }
        />
        <CanvasNodeToolbar
          componentId={id}
          isVisible={data.selected && !data.readOnly}
          onRequestDelete={data.onRequestDelete}
          onRequestDuplicate={data.onRequestDuplicate}
          onRequestSelect={data.onRequestSelect}
        />
        <div className="rf-cloud-frame-title">
          <Cloud size={18} strokeWidth={2.4} />
          <strong>{data.component.name || item.label}</strong>
        </div>
      </div>
    )
  }

  return (
    <div
      className={[
        'rf-architecture-node',
        data.selected ? 'selected' : '',
        data.inTraversal ? 'in-traversal' : '',
        data.activeTraversal ? 'active-traversal' : '',
        data.dimmedByTraversal ? 'dimmed-by-traversal' : '',
      ].filter(Boolean).join(' ')}
      style={{ '--node-color': item.color } as CSSProperties}
      data-component-type={data.component.type}
    >
      <div className="rf-node-depth" aria-hidden="true" />
      <NodeResizer
        isVisible={data.selected && !data.readOnly}
        minWidth={180}
        minHeight={72}
        handleClassName="rf-resize-handle"
        lineClassName="rf-resize-line"
        onResizeEnd={(_, params: ResizeParams) => data.onResizeEnd?.(id, { width: params.width, height: params.height })}
      />
      <CanvasNodeToolbar
        componentId={id}
        isVisible={data.selected && !data.readOnly}
        onRequestDelete={data.onRequestDelete}
        onRequestDuplicate={data.onRequestDuplicate}
        onRequestSelect={data.onRequestSelect}
      />
      <Handle id="target-left" className="rf-handle target" type="target" position={Position.Left} />
      <Handle id="target-top" className="rf-handle target top" type="target" position={Position.Top} />
      <ComponentGlyph type={data.component.type} label={item.shortLabel} />
      <div className="rf-node-copy">
        <strong>{data.component.name || item.label}</strong>
        <span>{data.component.type}</span>
      </div>
      <Handle id="source-right" className="rf-handle source" type="source" position={Position.Right} />
      <Handle id="source-bottom" className="rf-handle source bottom" type="source" position={Position.Bottom} />
    </div>
  )
}

function CanvasNodeToolbar({
  componentId,
  isVisible,
  onRequestDelete,
  onRequestDuplicate,
  onRequestSelect,
}: {
  componentId: string
  isVisible: boolean
  onRequestDelete?: (componentId: string) => void
  onRequestDuplicate?: (componentId: string) => void
  onRequestSelect?: (componentId: string) => void
}) {
  return (
    <NodeToolbar className="rf-node-toolbar nodrag nopan" isVisible={isVisible} position={Position.Top} offset={14}>
      <button type="button" title="Edit details" onClick={() => onRequestSelect?.(componentId)}>
        <Edit3 size={15} />
      </button>
      <button type="button" title="Duplicate" onClick={() => onRequestDuplicate?.(componentId)}>
        <Copy size={15} />
      </button>
      <button type="button" title="Add note" onClick={() => onRequestSelect?.(componentId)}>
        <StickyNote size={15} />
      </button>
      <button type="button" title="Delete" onClick={() => onRequestDelete?.(componentId)}>
        <Trash2 size={15} />
      </button>
    </NodeToolbar>
  )
}

function ComponentGlyph({ type, label }: { type: DesignComponent['type']; label: string }) {
  const Icon = componentIcons[type] ?? Server
  return (
    <div className="rf-node-icon" aria-label={label}>
      <Icon size={24} strokeWidth={2.4} />
    </div>
  )
}

const componentIcons: Partial<Record<DesignComponent['type'], LucideIcon>> = {
  'client.web': Globe2,
  'edge.api_gateway': Network,
  'compute.service': Server,
  'data.sql_database': Database,
  'data.redis': Zap,
  'messaging.queue': Rows3,
  'data.object_store': HardDrive,
  'ai.llm': Brain,
  'external.api': Cloud,
  'observability.telemetry': RadioTower,
  'security.control': Shield,
  'frame.cloud': Cloud,
  'note.sticky': FileText,
}

const nodeTypes = {
  architecture: ArchitectureNode,
}

export function ReactFlowCanvasProvider({
  design,
  selectedComponentId,
  selectedConnectorId,
  onSelectComponent,
  onSelectConnector,
  onMoveComponent,
  onResizeComponent,
  onConnectComponents,
  onReconnectConnector,
  onDuplicateComponent,
  onDeleteComponents,
  onDeleteConnectors,
  readOnly = false,
  traversalFocus,
}: ReactFlowPreviewProps) {
  const traversalComponentIds = useMemo(() => new Set(traversalFocus?.componentIds ?? []), [traversalFocus?.componentIds])
  const traversalConnectorIds = useMemo(() => new Set(traversalFocus?.connectorIds ?? []), [traversalFocus?.connectorIds])
  const hasTraversalFocus = traversalComponentIds.size > 0 || traversalConnectorIds.size > 0

  const nodes = useMemo<Node<ArchitectureNodeData>[]>(
    () =>
      [...design.components]
        .sort((left, right) => {
          if (left.type === 'frame.cloud' && right.type !== 'frame.cloud') return -1
          if (left.type !== 'frame.cloud' && right.type === 'frame.cloud') return 1
          return 0
        })
        .map((component, index) => ({
          id: component.id,
          type: 'architecture',
          parentId: component.metadata.parentFrameId,
          extent: component.metadata.parentFrameId ? 'parent' : undefined,
          position: component.metadata.position ?? {
            x: 120 + (index % 3) * 280,
            y: 120 + Math.floor(index / 3) * 170,
          },
          data: {
            component,
            selected: component.id === selectedComponentId,
            readOnly,
            inTraversal: traversalComponentIds.has(component.id),
            activeTraversal: component.id === traversalFocus?.activeComponentId,
            dimmedByTraversal: hasTraversalFocus && !traversalComponentIds.has(component.id) && component.type !== 'frame.cloud',
            onResizeEnd: onResizeComponent,
            onRequestDelete: (componentId) => onDeleteComponents?.([componentId]),
            onRequestDuplicate: onDuplicateComponent,
            onRequestSelect: onSelectComponent,
          },
          selected: component.id === selectedComponentId,
          style: {
            width: component.metadata.size?.width ?? (component.type === 'frame.cloud' ? 540 : 230),
            height: component.metadata.size?.height ?? (component.type === 'frame.cloud' ? 340 : 78),
          },
          zIndex: component.type === 'frame.cloud' ? 0 : 2,
        })),
    [
      design.components,
      hasTraversalFocus,
      onDeleteComponents,
      onDuplicateComponent,
      onResizeComponent,
      onSelectComponent,
      readOnly,
      selectedComponentId,
      traversalComponentIds,
      traversalFocus?.activeComponentId,
    ],
  )

  const edges = useMemo<Edge[]>(
    () =>
      design.connectors.map((connector) => {
        const inTraversal = traversalConnectorIds.has(connector.id)
        const activeTraversal = connector.id === traversalFocus?.activeConnectorId
        const dimmedByTraversal = hasTraversalFocus && !inTraversal
        return {
          id: connector.id,
          source: connector.fromComponentId,
          target: connector.toComponentId,
          animated: connector.animated || inTraversal,
          type: 'smoothstep',
          selected: connector.id === selectedConnectorId,
          reconnectable: true,
          zIndex: inTraversal ? 8 : 4,
          interactionWidth: 28,
          label: (
            <div
              className={[
                'rf-edge-label',
                connector.id === selectedConnectorId ? 'selected' : '',
                inTraversal ? 'in-traversal' : '',
                activeTraversal ? 'active-traversal' : '',
                dimmedByTraversal ? 'dimmed-by-traversal' : '',
              ].filter(Boolean).join(' ')}
            >
              <strong>{connector.type.replaceAll('_', ' ')}</strong>
              {connector.protocol ? <span>{connector.protocol}</span> : null}
            </div>
          ),
          labelShowBg: false,
          labelBgPadding: [0, 0],
          labelBgBorderRadius: 0,
          labelStyle: {
            fill: '#0f172a',
          },
          markerEnd: {
            type: MarkerType.ArrowClosed,
            color: activeTraversal || connector.id === selectedConnectorId ? '#2563eb' : inTraversal ? '#0f766e' : '#0f172a',
          },
          style: {
            strokeWidth: activeTraversal || connector.id === selectedConnectorId ? 3.5 : inTraversal ? 3 : 2.5,
            stroke: activeTraversal || connector.id === selectedConnectorId ? '#2563eb' : inTraversal ? '#0f766e' : '#0f172a',
            opacity: dimmedByTraversal ? 0.22 : 1,
          },
        }
      }),
    [design.connectors, hasTraversalFocus, selectedConnectorId, traversalConnectorIds, traversalFocus?.activeConnectorId],
  )

  const handleNodesChange: OnNodesChange<Node<ArchitectureNodeData>> = (changes) => {
    if (readOnly && changes.some((change) => change.type === 'remove' || change.type === 'position' || change.type === 'dimensions')) {
      return
    }
    const removed = changes.filter((change): change is NodeChange & { type: 'remove' } => change.type === 'remove')
    if (removed.length) {
      onDeleteComponents?.(removed.map((change) => change.id))
      return
    }

    applyNodeChanges(changes, nodes).forEach((node) => {
      const changed = changes.find((change) => 'id' in change && change.id === node.id)
      if (changed?.type === 'position' && node.position) {
        onMoveComponent?.(node.id, node.position, node.parentId)
      }
    })
  }

  const handleNodeDragStop: OnNodeDrag<Node<ArchitectureNodeData>> = (_, node) => {
    if (readOnly) return
    if (node.data.component.type === 'frame.cloud') return
    const width = Number(node.style?.width ?? node.width ?? 230)
    const height = Number(node.style?.height ?? node.height ?? 78)
    const nodeAbsolute = node.parentId
      ? {
          x: (design.components.find((item) => item.id === node.parentId)?.metadata.position?.x ?? 0) + node.position.x,
          y: (design.components.find((item) => item.id === node.parentId)?.metadata.position?.y ?? 0) + node.position.y,
        }
      : node.position
    const center = { x: nodeAbsolute.x + width / 2, y: nodeAbsolute.y + height / 2 }
    const nextParent = design.components.find((component) => {
      if (component.type !== 'frame.cloud' || component.id === node.id) return false
      const framePosition = component.metadata.position ?? { x: 0, y: 0 }
      const frameSize = component.metadata.size ?? { width: 540, height: 340 }
      return (
        center.x >= framePosition.x &&
        center.x <= framePosition.x + frameSize.width &&
        center.y >= framePosition.y &&
        center.y <= framePosition.y + frameSize.height
      )
    })
    if (nextParent?.id === node.parentId) return
    if (nextParent) {
      const framePosition = nextParent.metadata.position ?? { x: 0, y: 0 }
      onMoveComponent?.(node.id, { x: nodeAbsolute.x - framePosition.x, y: nodeAbsolute.y - framePosition.y }, nextParent.id)
      return
    }
    if (node.parentId) {
      onMoveComponent?.(node.id, nodeAbsolute, undefined)
    }
  }

  const handleEdgesChange: OnEdgesChange<Edge> = (changes: EdgeChange[]) => {
    if (readOnly && changes.some((change) => change.type === 'remove')) {
      return
    }
    const removed = changes.filter((change) => change.type === 'remove')
    if (removed.length) {
      onDeleteConnectors?.(removed.map((change) => change.id))
      return
    }
    applyEdgeChanges(changes, edges)
  }

  function handleConnect(connection: Connection) {
    if (readOnly) return
    if (!connection.source || !connection.target || connection.source === connection.target) return
    onConnectComponents?.(connection.source, connection.target)
  }

  const handleReconnect: OnReconnect<Edge> = (oldEdge, nextConnection) => {
    if (readOnly) return
    if (!nextConnection.source || !nextConnection.target || nextConnection.source === nextConnection.target) return
    onReconnectConnector?.(oldEdge.id, nextConnection.source, nextConnection.target)
  }

  return (
    <ReactFlowProvider>
      <div className="react-flow-canvas">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          fitView
          fitViewOptions={{ padding: 0.25 }}
          proOptions={{ hideAttribution: true }}
          nodesDraggable={!readOnly}
          nodesConnectable={!readOnly}
          elementsSelectable
          edgesReconnectable={!readOnly}
          deleteKeyCode={readOnly ? null : ['Backspace', 'Delete']}
          connectOnClick={!readOnly}
          connectionMode={ConnectionMode.Loose}
          reconnectRadius={18}
          selectionOnDrag
          selectionMode={SelectionMode.Partial}
          panOnDrag={[1, 2]}
          panOnScroll
          panOnScrollMode={PanOnScrollMode.Free}
          zoomOnScroll={false}
          zoomOnPinch
          zoomOnDoubleClick={false}
          onNodesChange={handleNodesChange}
          onEdgesChange={handleEdgesChange}
          onConnect={handleConnect}
          onReconnect={handleReconnect}
          onNodeDragStop={handleNodeDragStop}
          onNodeClick={(_, node) => {
            onSelectConnector?.('')
            onSelectComponent?.(node.id)
          }}
          onEdgeClick={(_, edge) => {
            onSelectComponent?.('')
            onSelectConnector?.(edge.id)
          }}
          onPaneClick={() => {
            onSelectComponent?.('')
            onSelectConnector?.('')
          }}
          connectionLineType={ConnectionLineType.SmoothStep}
          connectionRadius={42}
          defaultEdgeOptions={{
            type: 'smoothstep',
            zIndex: 4,
            markerEnd: { type: MarkerType.ArrowClosed, color: '#0f172a' },
            style: { strokeWidth: 2.5, stroke: '#0f172a' },
          }}
        >
          <Background variant={BackgroundVariant.Dots} gap={28} size={1.2} color="#cbd5e1" />
          <Controls position="bottom-right" />
        </ReactFlow>
      </div>
    </ReactFlowProvider>
  )
}
