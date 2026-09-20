import { describe, expect, it } from 'vitest'
import { inferJourneyStepKind, journeyStepKindLabel, journeyStepSummary } from '../src/app/journeyModel'

describe('journey model', () => {
  it('classifies component, request, async, batch, signal, and callback steps', () => {
    expect(inferJourneyStepKind()).toBe('component')
    expect(inferJourneyStepKind({ type: 'synchronous', fromComponentId: 'a', toComponentId: 'b' })).toBe('request')
    expect(inferJourneyStepKind({ type: 'asynchronous_event', fromComponentId: 'a', toComponentId: 'b' })).toBe('async')
    expect(inferJourneyStepKind({ type: 'batch_transfer', fromComponentId: 'a', toComponentId: 'b' })).toBe('batch')
    expect(inferJourneyStepKind({ type: 'observability_signal', fromComponentId: 'a', toComponentId: 'b' })).toBe('signal')
    expect(inferJourneyStepKind({ type: 'synchronous', fromComponentId: 'b', toComponentId: 'a' }, new Set(['a']))).toBe('callback')
  })

  it('keeps labels and descriptions stable', () => {
    expect(journeyStepKindLabel('decision')).toBe('Decision')
    expect(journeyStepKindLabel(undefined, 'asynchronous_event')).toBe('Async')
    expect(journeyStepSummary('async')).toContain('Fire-and-continue')
    expect(journeyStepSummary('callback')).toContain('Reverse')
    expect(journeyStepSummary('batch')).toContain('scheduled')
    expect(journeyStepSummary('signal')).toContain('Telemetry')
    expect(journeyStepSummary('component')).toContain('component')
    expect(journeyStepSummary('request')).toContain('Synchronous')
  })
})
