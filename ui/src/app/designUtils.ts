import { getCatalogItem } from '../catalog'
import type { ComponentType, DesignComponent, DesignDocument, DesignJourney } from '../types'
import { inferJourneyStepKind } from './journeyModel'
export { nextVersionStatusOptions, versionStatusLabel, versionTransitionLabel } from './versionLifecycle'

export function isJourneyComponent(component: DesignComponent) {
  return component.type !== 'frame.cloud' && component.type !== 'note.sticky'
}

export function buildPrimaryJourney(design: DesignDocument): DesignJourney | null {
  const components = design.components.filter(isJourneyComponent)
  if (!components.length) return null

  const componentById = new Map(design.components.map((component) => [component.id, component]))
  const incoming = new Set(design.connectors.map((connector) => connector.toComponentId))
  const preferredStartTypes: ComponentType[] = ['client.web', 'external.api', 'edge.api_gateway']
  const start =
    components.find((component) => !incoming.has(component.id) && preferredStartTypes.includes(component.type)) ??
    components.find((component) => !incoming.has(component.id)) ??
    components[0]
  const now = new Date().toISOString()
  const steps: DesignJourney['steps'] = []
  const visitedComponents = new Set<string>()
  const visitedConnectors = new Set<string>()

  function pushComponentStep(component: DesignComponent, prefix = 'Start') {
    if (visitedComponents.has(component.id) || steps.length >= 18) return
    visitedComponents.add(component.id)
    steps.push({
      id: `step_${crypto.randomUUID()}`,
      componentId: component.id,
      kind: 'component',
      title: component.name || getCatalogItem(component.type).label,
      description: `${prefix} at ${component.name || getCatalogItem(component.type).label}.`,
    })
  }

  function walk(component: DesignComponent) {
    if (steps.length >= 18) return
    const outgoing = design.connectors.filter(
      (connector) => connector.fromComponentId === component.id && !visitedConnectors.has(connector.id),
    )
    for (const connector of outgoing) {
      if (steps.length >= 18) return
      visitedConnectors.add(connector.id)
      const target = componentById.get(connector.toComponentId)
      const sourceName = component.name || getCatalogItem(component.type).label
      const targetName = target ? target.name || getCatalogItem(target.type).label : 'unknown target'
      steps.push({
        id: `step_${crypto.randomUUID()}`,
        componentId: target?.id,
        connectorId: connector.id,
        kind: inferJourneyStepKind(connector, visitedComponents),
        fromComponentId: connector.fromComponentId,
        toComponentId: connector.toComponentId,
        title: `${sourceName} to ${targetName}`,
        description: `${sourceName} sends ${connector.type.replaceAll('_', ' ')} traffic to ${targetName}${
          connector.protocol ? ` over ${connector.protocol}` : ''
        }.`,
      })
      if (target && isJourneyComponent(target) && !visitedComponents.has(target.id)) {
        visitedComponents.add(target.id)
        walk(target)
      }
    }
  }

  pushComponentStep(start)
  walk(start)

  if (steps.length === 1 && design.connectors.length) {
    const firstConnector = design.connectors[0]
    const source = componentById.get(firstConnector.fromComponentId)
    const target = componentById.get(firstConnector.toComponentId)
    steps.push({
      id: `step_${crypto.randomUUID()}`,
      componentId: target?.id,
      connectorId: firstConnector.id,
      kind: inferJourneyStepKind(firstConnector),
      fromComponentId: firstConnector.fromComponentId,
      toComponentId: firstConnector.toComponentId,
      title: `${source?.name ?? 'Source'} to ${target?.name ?? 'Target'}`,
      description: `${source?.name ?? 'Source'} sends ${firstConnector.type.replaceAll('_', ' ')} traffic to ${
        target?.name ?? 'target'
      }.`,
    })
  }

  return {
    id: `journey_${crypto.randomUUID()}`,
    title: design.requirementBrief.useCase.trim() ? `${design.requirementBrief.useCase.trim()} journey` : 'Primary request journey',
    description: 'Ordered walkthrough generated from the current component graph. Refine the step text as the design matures.',
    entryComponentId: start.id,
    steps,
    createdAt: now,
    updatedAt: now,
  }
}

export function stripHtml(value: string) {
  return value
    .replace(/<style[\s\S]*?<\/style>/gi, '')
    .replace(/<script[\s\S]*?<\/script>/gi, '')
    .replace(/<[^>]+>/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

export function isBackendOfflineMessage(message: string) {
  const normalized = message.toLowerCase()
  return normalized.includes('backend unavailable') || normalized.includes('load failed') || normalized.includes('failed to fetch')
}

export function normalizedCatalogLabel(value: string) {
  return value.trim().toLowerCase().replace(/\s+/g, ' ')
}

export function analysisReadinessLabel(score: number) {
  if (score <= 0) return 'Not ready'
  if (score < 40) return 'Foundation missing'
  if (score < 70) return 'Needs definition'
  if (score < 85) return 'Reviewable'
  return 'Strong foundation'
}

export function analysisFindingMatchesFocus(focus: string, suite: string) {
  if (focus === 'full') return true
  const suitesByFocus: Record<string, string[]> = {
    security: ['security'],
    scalability: ['traffic', 'topology'],
    reliability: ['availability', 'topology', 'integrity'],
    data: ['data', 'consistency'],
    operability: ['availability', 'topology', 'integrity'],
    cost: ['traffic', 'data', 'topology'],
  }
  return (suitesByFocus[focus] ?? []).includes(suite)
}

export function isPotentialCatalogMatch(left: string, right: string) {
  const normalizedLeft = normalizedCatalogLabel(left)
  const normalizedRight = normalizedCatalogLabel(right)
  return Boolean(
    normalizedLeft &&
      normalizedRight &&
      (normalizedLeft === normalizedRight || normalizedLeft.includes(normalizedRight) || normalizedRight.includes(normalizedLeft)),
  )
}

export function catalogAssetUsageLabel(count: number) {
  return `${count} design${count === 1 ? '' : 's'}`
}
