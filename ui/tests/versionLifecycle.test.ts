import { describe, expect, it } from 'vitest'
import {
  canTransitionVersionStatus,
  nextVersionStatusOptions,
  normalizeVersionStatus,
  statusAfterDesignChange,
  statusAfterReviewRequest,
  versionStatusLabel,
  versionTransitionLabel,
} from '../src/app/versionLifecycle'

describe('version lifecycle', () => {
  it('normalizes statuses and labels', () => {
    expect(normalizeVersionStatus(' pending_review ')).toBe('pending_review')
    expect(normalizeVersionStatus('unknown')).toBe('draft')
    expect(versionStatusLabel('pending_review')).toBe('Pending review')
  })

  it('allows only explicit transitions', () => {
    expect(nextVersionStatusOptions('draft')).toEqual(['pending_review'])
    expect(canTransitionVersionStatus('draft', 'pending_review')).toBe(true)
    expect(canTransitionVersionStatus('pending_review', 'reviewed')).toBe(true)
    expect(canTransitionVersionStatus('reviewed', 'live')).toBe(true)
    expect(canTransitionVersionStatus('draft', 'live')).toBe(false)
    expect(canTransitionVersionStatus('live', 'live')).toBe(true)
  })

  it('keeps action labels and side effects deterministic', () => {
    expect(versionTransitionLabel('draft', 'pending_review')).toBe('Ready for review')
    expect(versionTransitionLabel('pending_review', 'reviewed')).toBe('Mark reviewed')
    expect(versionTransitionLabel('reviewed', 'live')).toBe('Make live')
    expect(versionTransitionLabel('live', 'draft')).toBe('Move back to draft')
    expect(statusAfterReviewRequest('draft')).toBe('pending_review')
    expect(statusAfterReviewRequest('reviewed')).toBe('reviewed')
    expect(statusAfterDesignChange('live')).toBe('live')
    expect(statusAfterDesignChange('reviewed')).toBe('draft')
  })
})
