import { describe, expect, it } from 'vitest'
import { initialCanvasPanelState } from '../src/app/canvasLayout'

describe('initial canvas panels', () => {
  it.each([
    [1600, { leftRailOpen: true, rightRailOpen: true }],
    [1160, { leftRailOpen: false, rightRailOpen: true }],
    [900, { leftRailOpen: false, rightRailOpen: false }],
  ])('uses an ergonomic layout at %dpx', (width, expected) => {
    expect(initialCanvasPanelState(width)).toEqual(expected)
  })

  it('uses the full layout for an invalid width', () => {
    expect(initialCanvasPanelState(Number.NaN)).toEqual({ leftRailOpen: true, rightRailOpen: true })
  })
})
