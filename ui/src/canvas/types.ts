import type { ComponentType, DesignDocument } from '../types'

export interface ReactFlowPreviewProps {
  design: DesignDocument
  selectedComponentId?: string | null
  selectedConnectorId?: string | null
  readOnly?: boolean
  traversalFocus?: {
    componentIds: string[]
    connectorIds: string[]
    activeComponentId?: string | null
    activeConnectorId?: string | null
  } | null
  onSelectComponent?: (componentId: string) => void
  onSelectConnector?: (connectorId: string) => void
  onMoveComponent?: (componentId: string, position: { x: number; y: number }, parentFrameId?: string) => void
  onResizeComponent?: (componentId: string, size: { width: number; height: number }) => void
  onConnectComponents?: (fromComponentId: string, toComponentId: string) => void
  onReconnectConnector?: (connectorId: string, fromComponentId: string, toComponentId: string) => void
  onDropComponent?: (type: ComponentType, position: { x: number; y: number }) => void
  onDropCatalogAsset?: (assetId: string, position: { x: number; y: number }) => void
  onDuplicateComponent?: (componentId: string) => void
  onDeleteComponents?: (componentIds: string[]) => void
  onDeleteConnectors?: (connectorIds: string[]) => void
}
