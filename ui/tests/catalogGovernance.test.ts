import { describe, expect, it } from 'vitest'
import {
  catalogAssetAdvisoryText,
  catalogAssetKindLabel,
  catalogAssetStatusLabel,
  isCatalogAssetActionable,
} from '../src/app/catalogGovernance'

describe('catalog governance', () => {
  it('normalizes labels and actionable states', () => {
    expect(catalogAssetKindLabel('system')).toBe('System')
    expect(catalogAssetKindLabel('component')).toBe('Component')
    expect(catalogAssetStatusLabel('deprecated')).toBe('Deprecated')
    expect(catalogAssetStatusLabel('')).toBe('Active')
    expect(isCatalogAssetActionable({ status: 'deprecated' })).toBe(true)
    expect(isCatalogAssetActionable({ status: 'retired' })).toBe(true)
    expect(isCatalogAssetActionable({ status: 'active' })).toBe(false)
  })

  it('builds advisories only for governed assets', () => {
    expect(catalogAssetAdvisoryText({ status: 'active' })).toBe('')
    expect(catalogAssetAdvisoryText({ status: 'deprecated', updateMessage: ' Move to v2 ' })).toBe('Move to v2')
    expect(catalogAssetAdvisoryText({ status: 'deprecated', replacementAssetId: 'asset-v2' })).toContain('asset-v2')
    expect(catalogAssetAdvisoryText({ status: 'retired' })).toContain('retired')
  })
})
