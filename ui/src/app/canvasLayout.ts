export const CANVAS_LAYOUT_BREAKPOINTS = {
  compactPalette: 1220,
  overlayContext: 1100,
} as const

export function initialCanvasPanelState(viewportWidth: number) {
  const width = Number.isFinite(viewportWidth) ? viewportWidth : Number.POSITIVE_INFINITY
  return {
    leftRailOpen: width > CANVAS_LAYOUT_BREAKPOINTS.compactPalette,
    rightRailOpen: width > CANVAS_LAYOUT_BREAKPOINTS.overlayContext,
  }
}
