import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { loadTsModule } from './loadTsModule.mjs'

const interactions = loadTsModule('src/canvas/interactions.ts')

describe('canvas interactions', () => {
  it('centers dropped components on the cursor', () => {
    const position = interactions.calculateCenteredDropPosition({ x: 500, y: 300 }, { width: 230, height: 78 })
    assert.equal(position.x, 385)
    assert.equal(position.y, 261)
    const frameSize = interactions.defaultComponentSize('frame.cloud')
    assert.equal(frameSize.width, 540)
    assert.equal(frameSize.height, 340)
  })

  it('rejects invalid connector attempts', () => {
    assert.equal(interactions.isValidCanvasConnection('a', 'b'), true)
    assert.equal(interactions.isValidCanvasConnection('a', 'a'), false)
    assert.equal(interactions.isValidCanvasConnection('a', ''), false)
  })

  it('detects read-only mutating node changes and stable selection sets', () => {
    assert.equal(interactions.isMutatingNodeChange('position'), true)
    assert.equal(interactions.isMutatingNodeChange('select'), false)
    assert.equal(interactions.sameStringSet(['a', 'b'], ['b', 'a']), true)
    assert.equal(interactions.sameStringSet(['a', 'b'], ['a']), false)
  })
})
