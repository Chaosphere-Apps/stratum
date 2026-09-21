import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createComponent, createEmptyDesign, createEmptyRequirementBrief, toExportableDesign, touchDesign } from '../src/designModel'
import { defaultRedisMetadata, getCatalogItem } from '../src/catalog'

describe('structured design model', () => {
  beforeEach(() => vi.spyOn(crypto, 'randomUUID').mockReturnValue('00000000-0000-4000-8000-000000000001'))

  it('creates complete requirement and design defaults', () => {
    expect(createEmptyRequirementBrief({ useCase: 'Pay', targetRps: 10 })).toMatchObject({
      useCase: 'Pay', targetRps: 10, functionalRequirements: '', consistencyNotes: '', openQuestions: '',
    })
    const design = createEmptyDesign({ title: 'Payments', requirementBrief: { sla: '100ms' } })
    expect(design).toMatchObject({ schemaVersion: 'sde-ui/v0.1', title: 'Payments', components: [], connectors: [], journeys: [] })
    expect(design.requirementBrief.sla).toBe('100ms')
  })

  it('creates typed components and Redis metadata', () => {
    expect(createComponent({ shapeId: 'shape', type: 'compute.service', name: 'API' })).toMatchObject({
      id: 'cmp_00000000-0000-4000-8000-000000000001', shapeId: 'shape', type: 'compute.service', criticality: 'medium', metadata: {},
    })
    expect(createComponent({ shapeId: 'redis', type: 'data.redis', name: 'Cache' }).metadata.redis).toEqual(defaultRedisMetadata())
    expect(getCatalogItem('not-real' as never).type).toBe('compute.service')
  })

  it('normalizes touched and exported documents', () => {
    const design = { ...createEmptyDesign(), journeys: undefined } as never
    expect(touchDesign(design).journeys).toEqual([])
    expect(toExportableDesign(design)).toMatchObject({ canvas: { provider: 'react-flow' }, journeys: [] })
  })
})
