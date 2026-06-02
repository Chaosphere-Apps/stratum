import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { loadTsModule } from './loadTsModule.mjs'

const journeyModel = loadTsModule('src/app/journeyModel.ts')

describe('journey model', () => {
  it('classifies async, batch, signal, and callback steps', () => {
    assert.equal(
      journeyModel.inferJourneyStepKind({ type: 'asynchronous_event', fromComponentId: 'api', toComponentId: 'queue' }),
      'async',
    )
    assert.equal(
      journeyModel.inferJourneyStepKind({ type: 'batch_transfer', fromComponentId: 'worker', toComponentId: 'store' }),
      'batch',
    )
    assert.equal(
      journeyModel.inferJourneyStepKind({ type: 'observability_signal', fromComponentId: 'api', toComponentId: 'logs' }),
      'signal',
    )
    assert.equal(
      journeyModel.inferJourneyStepKind(
        { type: 'synchronous', fromComponentId: 'webhook', toComponentId: 'api' },
        new Set(['api']),
      ),
      'callback',
    )
  })

  it('keeps step labels stable for UI rendering', () => {
    assert.equal(journeyModel.journeyStepKindLabel('async'), 'Async')
    assert.equal(journeyModel.journeyStepKindLabel(undefined, 'asynchronous_event'), 'Async')
    assert.match(journeyModel.journeyStepSummary('callback'), /Reverse/)
  })
})
