import {
  Activity,
  ChevronsLeftRight,
  Cloud,
  Database,
  FilePlus2,
  Globe2,
  MessageSquare,
  Monitor,
  Route,
  Server,
  ShieldCheck,
  Sparkles,
} from 'lucide-react'
import { componentCatalog } from '../catalog'
import type { ComponentType } from '../types'

export function CatalogGlyph({ type }: { type: ComponentType }) {
  if (type === 'client.web') return <Monitor size={17} />
  if (type === 'edge.api_gateway') return <Route size={17} />
  if (type === 'data.sql_database' || type === 'data.redis' || type === 'data.object_store') return <Database size={17} />
  if (type === 'messaging.queue') return <ChevronsLeftRight size={17} />
  if (type === 'ai.llm') return <Sparkles size={17} />
  if (type === 'external.api') return <Globe2 size={17} />
  if (type === 'observability.telemetry') return <Activity size={17} />
  if (type === 'security.control') return <ShieldCheck size={17} />
  if (type === 'design.link') return <FilePlus2 size={17} />
  if (type === 'frame.cloud') return <Cloud size={17} />
  if (type === 'note.sticky') return <MessageSquare size={17} />
  return <Server size={17} />
}

export function isCatalogComponentType(type: string): type is ComponentType {
  return componentCatalog.some((item) => item.type === type)
}
