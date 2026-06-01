import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { loadTsModule } from './loadTsModule.mjs'

const lifecycle = loadTsModule('src/app/versionLifecycle.ts')

describe('version lifecycle', () => {
  it('normalizes and labels statuses', () => {
    assert.equal(lifecycle.normalizeVersionStatus(' REVIEWED '), 'reviewed')
    assert.equal(lifecycle.normalizeVersionStatus('unknown'), 'draft')
    assert.equal(lifecycle.versionStatusLabel('pending_review'), 'Pending review')
  })

  it('allows only explicit version transitions', () => {
    assert.equal(lifecycle.nextVersionStatusOptions('draft').join(','), 'pending_review')
    assert.equal(lifecycle.canTransitionVersionStatus('draft', 'pending_review'), true)
    assert.equal(lifecycle.canTransitionVersionStatus('draft', 'live'), false)
    assert.equal(lifecycle.canTransitionVersionStatus('pending_review', 'reviewed'), true)
    assert.equal(lifecycle.canTransitionVersionStatus('reviewed', 'live'), true)
    assert.equal(lifecycle.canTransitionVersionStatus('live', 'pending_review'), false)
  })

  it('keeps lifecycle side effects deterministic', () => {
    assert.equal(lifecycle.statusAfterReviewRequest('draft'), 'pending_review')
    assert.equal(lifecycle.statusAfterReviewRequest('reviewed'), 'reviewed')
    assert.equal(lifecycle.statusAfterDesignChange('pending_review'), 'draft')
    assert.equal(lifecycle.statusAfterDesignChange('reviewed'), 'draft')
    assert.equal(lifecycle.statusAfterDesignChange('live'), 'live')
  })
})
