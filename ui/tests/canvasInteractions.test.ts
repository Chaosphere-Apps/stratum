import { describe, expect, it } from 'vitest'
import {
  calculateCenteredDropPosition,
  defaultComponentSize,
  isMutatingNodeChange,
  isValidCanvasConnection,
  removeComponentsFromDesign,
  sameStringSet,
} from '../src/canvas/interactions'
import { connectorTypeDescription, connectorTypeLabel, connectorTypeOptions, connectorVisualProfile } from '../src/canvas/connectorVisuals'

describe('canvas interactions', () => {
  it('centers dropped components on the cursor', () => {
    expect(calculateCenteredDropPosition({ x: 500, y: 300 }, { width: 230, height: 78 })).toEqual({ x: 385, y: 261 })
    expect(defaultComponentSize('frame.cloud')).toEqual({ width: 540, height: 340 })
  })

  it('rejects incomplete and self-referencing connections', () => {
    expect(isValidCanvasConnection('a', 'b')).toBe(true)
    expect(isValidCanvasConnection('a', 'a')).toBe(false)
    expect(isValidCanvasConnection('a', '')).toBe(false)
  })

  it('classifies mutating changes and compares selection sets', () => {
    expect(isMutatingNodeChange('remove')).toBe(true)
    expect(isMutatingNodeChange('position')).toBe(true)
    expect(isMutatingNodeChange('dimensions')).toBe(true)
    expect(isMutatingNodeChange('select')).toBe(false)
    expect(sameStringSet(['a', 'b'], ['b', 'a'])).toBe(true)
    expect(sameStringSet(['a'], ['a', 'b'])).toBe(false)
  })

  it('removes components with dependent connectors and journey steps', () => {
    const design = {
      components: [{ id: 'a' }, { id: 'b' }, { id: 'c' }],
      connectors: [
        { id: 'ab', fromComponentId: 'a', toComponentId: 'b' },
        { id: 'bc', fromComponentId: 'b', toComponentId: 'c' },
      ],
      journeys: [{ id: 'journey', steps: [
        { id: 'one', componentId: 'a' },
        { id: 'two', connectorId: 'ab' },
        { id: 'three', componentId: 'c' },
      ] }],
    } as never
    const next = removeComponentsFromDesign(design, ['a'])
    expect(next.components.map((component) => component.id)).toEqual(['b', 'c'])
    expect(next.connectors.map((connector) => connector.id)).toEqual(['bc'])
    expect(next.journeys[0].steps.map((step) => step.id)).toEqual(['three'])
    expect(removeComponentsFromDesign(next, ['missing'])).toBe(next)
  })

  it('provides stable connector visual semantics', () => {
    expect(connectorVisualProfile('asynchronous_event')).toMatchObject({ tone: 'async', shouldAnimate: true })
    expect(connectorVisualProfile('synchronous')).toMatchObject({ tone: 'request', shouldAnimate: false })
    expect(connectorTypeLabel('model_call')).toBe('Model call')
    expect(connectorTypeOptions).toHaveLength(7)
    for (const type of connectorTypeOptions) {
      expect(connectorTypeDescription(type).length).toBeGreaterThan(10)
      expect(connectorVisualProfile(type).label).toBeTruthy()
    }
    expect(connectorTypeDescription('unknown' as never)).toBe('Architecture connector.')
  })
})
