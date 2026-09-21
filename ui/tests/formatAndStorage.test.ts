import { describe, expect, it } from 'vitest'
import { formatCompactDateTime, initialsFor, roleLabel, userDisplayName } from '../src/app/format'
import { clearSavedDesign, loadDesignDocument, saveDesignDocument } from '../src/storage'

describe('display formatting', () => {
  const users = [{ id: 'user-1', displayName: 'Ada Lovelace' }]

  it('formats identity data with safe fallbacks', () => {
    expect(userDisplayName(users as never, 'user-1')).toBe('Ada Lovelace')
    expect(userDisplayName(users as never, 'missing-user')).toBe('missing user')
    expect(initialsFor('Ada Lovelace')).toBe('AL')
    expect(initialsFor('')).toBe('U')
    expect(roleLabel('admin')).toBe('Admin')
    expect(roleLabel('architect')).toBe('Architect')
    expect(roleLabel('reviewer')).toBe('Reviewer')
    expect(roleLabel('custom')).toBe('Member')
    expect(formatCompactDateTime()).toBe('Never')
    expect(formatCompactDateTime('invalid')).toBe('Never')
  })
})

describe('local draft storage', () => {
  it('round-trips and clears the exact structured design', () => {
    const design = { id: 'design', title: 'Payments', components: [], connectors: [], journeys: [] } as never
    expect(loadDesignDocument()).toBeNull()
    saveDesignDocument(design)
    expect(loadDesignDocument()).toEqual(design)
    clearSavedDesign()
    expect(loadDesignDocument()).toBeNull()
  })
})
