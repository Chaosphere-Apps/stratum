import type { DesignDocument } from './types'

const DESIGN_KEY = 'sde.ui.designDocument'

export function saveDesignDocument(design: DesignDocument) {
  window.localStorage.setItem(DESIGN_KEY, JSON.stringify(design))
}

export function loadDesignDocument(): DesignDocument | null {
  const raw = window.localStorage.getItem(DESIGN_KEY)
  return raw ? (JSON.parse(raw) as DesignDocument) : null
}

export function clearSavedDesign() {
  window.localStorage.removeItem(DESIGN_KEY)
}
