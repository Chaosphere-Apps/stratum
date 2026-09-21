import { describe, expect, it } from 'vitest'
import {
  analysisFindingMatchesFocus,
  analysisReadinessLabel,
  buildPrimaryJourney,
  catalogAssetUsageLabel,
  isBackendOfflineMessage,
  isJourneyComponent,
  isPotentialCatalogMatch,
  normalizedCatalogLabel,
  stripHtml,
} from '../src/app/designUtils'

describe('design utilities', () => {
  it('sanitizes display text and classifies messages', () => {
    expect(stripHtml('<style>x</style><p>Hello <strong>world</strong></p><script>bad</script>')).toBe('Hello world')
    expect(isBackendOfflineMessage('Backend unavailable: timeout')).toBe(true)
    expect(isBackendOfflineMessage('Failed to fetch')).toBe(true)
    expect(isBackendOfflineMessage('Validation failed')).toBe(false)
    expect(normalizedCatalogLabel('  Purchase   Service ')).toBe('purchase service')
    expect(isPotentialCatalogMatch('Purchase Service', 'purchase')).toBe(true)
    expect(isPotentialCatalogMatch('', 'purchase')).toBe(false)
    expect(catalogAssetUsageLabel(1)).toBe('1 design')
    expect(catalogAssetUsageLabel(2)).toBe('2 designs')
  })

  it('labels readiness honestly and maps review lenses to relevant suites', () => {
    expect(analysisReadinessLabel(0)).toBe('Not ready')
    expect(analysisReadinessLabel(25)).toBe('Foundation missing')
    expect(analysisReadinessLabel(55)).toBe('Needs definition')
    expect(analysisReadinessLabel(75)).toBe('Reviewable')
    expect(analysisReadinessLabel(90)).toBe('Strong foundation')

    expect(analysisFindingMatchesFocus('full', 'security')).toBe(true)
    expect(analysisFindingMatchesFocus('scalability', 'traffic')).toBe(true)
    expect(analysisFindingMatchesFocus('scalability', 'security')).toBe(false)
    expect(analysisFindingMatchesFocus('data', 'consistency')).toBe(true)
    expect(analysisFindingMatchesFocus('unknown', 'topology')).toBe(false)
  })

  it('excludes frames and notes from journeys', () => {
    expect(isJourneyComponent({ type: 'compute.service' } as never)).toBe(true)
    expect(isJourneyComponent({ type: 'frame.cloud' } as never)).toBe(false)
    expect(isJourneyComponent({ type: 'note.sticky' } as never)).toBe(false)
  })

  it('builds an ordered journey with protocol and async semantics', () => {
    const design = {
      requirementBrief: { useCase: 'Checkout' },
      components: [
        { id: 'client', type: 'client.web', name: 'Client' },
        { id: 'api', type: 'compute.service', name: 'API' },
        { id: 'queue', type: 'messaging.queue', name: 'Queue' },
      ],
      connectors: [
        { id: 'one', fromComponentId: 'client', toComponentId: 'api', type: 'synchronous', protocol: 'HTTPS' },
        { id: 'two', fromComponentId: 'api', toComponentId: 'queue', type: 'asynchronous_event', protocol: '' },
      ],
    } as never
    const journey = buildPrimaryJourney(design)
    expect(journey?.title).toBe('Checkout journey')
    expect(journey?.entryComponentId).toBe('client')
    expect(journey?.steps.map((step) => step.kind)).toEqual(['component', 'request', 'async'])
    expect(journey?.steps[1].description).toContain('HTTPS')
    expect(buildPrimaryJourney({ ...design, components: [], connectors: [] })).toBeNull()
  })
})
