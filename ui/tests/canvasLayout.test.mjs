import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { loadTsModule } from './loadTsModule.mjs'

const layout = loadTsModule('src/app/canvasLayout.ts')

describe('canvas panel layout', () => {
  it('keeps both panels open on wide screens', () => {
    const state = layout.initialCanvasPanelState(1440)
    assert.equal(state.leftRailOpen, true)
    assert.equal(state.rightRailOpen, true)
  })

  it('compacts the component palette on laptop screens', () => {
    const state = layout.initialCanvasPanelState(1180)
    assert.equal(state.leftRailOpen, false)
    assert.equal(state.rightRailOpen, true)
  })

  it('auto-hides context when it becomes an overlay', () => {
    const state = layout.initialCanvasPanelState(1024)
    assert.equal(state.leftRailOpen, false)
    assert.equal(state.rightRailOpen, false)
  })
})
