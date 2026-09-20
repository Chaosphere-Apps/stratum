import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { loadTsModule } from './loadTsModule.mjs'

const governance = loadTsModule('src/app/catalogGovernance.ts')

describe('catalog governance', () => {
  it('normalizes user-facing labels', () => {
    assert.equal(governance.catalogAssetKindLabel('system'), 'System')
    assert.equal(governance.catalogAssetKindLabel('component'), 'Component')
    assert.equal(governance.catalogAssetKindLabel(undefined), 'Component')
    assert.equal(governance.catalogAssetStatusLabel(' deprecated '), 'Deprecated')
    assert.equal(governance.catalogAssetStatusLabel('retired'), 'Retired')
    assert.equal(governance.catalogAssetStatusLabel('unknown'), 'Active')
  })

  it('shows advisories only for deprecated and retired assets', () => {
    assert.equal(governance.isCatalogAssetActionable({ status: 'active' }), false)
    assert.equal(governance.isCatalogAssetActionable({ status: 'proposed' }), false)
    assert.equal(governance.isCatalogAssetActionable({ status: 'deprecated' }), true)
    assert.equal(
      governance.catalogAssetAdvisoryText({ status: 'deprecated', updateMessage: 'Move to Payments v2.' }),
      'Move to Payments v2.',
    )
    assert.match(
      governance.catalogAssetAdvisoryText({ status: 'retired', replacementAssetId: 'asset_new' }),
      /asset_new/,
    )
  })
})
