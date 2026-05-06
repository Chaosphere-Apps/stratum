import type { ComponentType, DesignComponent, DesignDocument, RequirementBrief } from './types'
import { defaultRedisMetadata } from './catalog'

export function createEmptyRequirementBrief(input: Partial<RequirementBrief> = {}): RequirementBrief {
  return {
    useCase: input.useCase ?? '',
    functionalRequirements: input.functionalRequirements ?? '',
    nonFunctionalRequirements: input.nonFunctionalRequirements ?? '',
    targetRps: input.targetRps ?? null,
    availabilityRequirement: input.availabilityRequirement ?? '',
    sla: input.sla ?? '',
    problemStatement: input.problemStatement ?? '',
    primaryActors: input.primaryActors ?? '',
    trafficNotes: input.trafficNotes ?? '',
    consistencyNotes: input.consistencyNotes ?? '',
    openQuestions: input.openQuestions ?? '',
  }
}

export function createEmptyDesign(input: { id?: string; title?: string; requirementBrief?: Partial<RequirementBrief> } = {}): DesignDocument {
  return {
    schemaVersion: 'sde-ui/v0.1',
    id: input.id ?? `design_${crypto.randomUUID()}`,
    title: input.title ?? 'Untitled system design',
    requirementBrief: createEmptyRequirementBrief(input.requirementBrief),
    components: [],
    connectors: [],
    journeys: [],
    updatedAt: new Date().toISOString(),
  }
}

export function createComponent(input: {
  shapeId: string
  type: ComponentType
  name: string
}): DesignComponent {
  const metadata = input.type === 'data.redis' ? { redis: defaultRedisMetadata() } : {}
  return {
    id: `cmp_${crypto.randomUUID()}`,
    shapeId: input.shapeId,
    type: input.type,
    name: input.name,
    purpose: '',
    owner: '',
    criticality: 'medium',
    metadata,
    notes: [],
  }
}

export function touchDesign(design: DesignDocument): DesignDocument {
  return {
    ...design,
    requirementBrief: createEmptyRequirementBrief(design.requirementBrief),
    journeys: design.journeys ?? [],
    updatedAt: new Date().toISOString(),
  }
}

export function toExportableDesign(design: DesignDocument) {
  return {
    ...touchDesign(design),
    canvas: {
      provider: 'react-flow',
    },
  }
}
