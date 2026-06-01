import type { ComponentType } from '../types'

export type Point = { x: number; y: number }
export type Size = { width: number; height: number }

export function calculateCenteredDropPosition(cursorPosition: Point, size: Size): Point {
  return {
    x: cursorPosition.x - size.width / 2,
    y: cursorPosition.y - size.height / 2,
  }
}

export function defaultComponentSize(type: ComponentType): Size {
  return type === 'frame.cloud' ? { width: 540, height: 340 } : { width: 230, height: 78 }
}

export function isValidCanvasConnection(source?: string | null, target?: string | null) {
  return Boolean(source && target && source !== target)
}

export function isMutatingNodeChange(changeType: string) {
  return changeType === 'remove' || changeType === 'position' || changeType === 'dimensions'
}

export function sameStringSet(left: string[], right: string[]) {
  if (left.length !== right.length) return false
  const rightIds = new Set(right)
  return left.every((id) => rightIds.has(id))
}
