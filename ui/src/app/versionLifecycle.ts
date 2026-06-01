import type { BackendDesignVersionStatus } from '../backendApi'

export const versionStatuses: BackendDesignVersionStatus[] = ['draft', 'pending_review', 'reviewed', 'live']

export function normalizeVersionStatus(status: string): BackendDesignVersionStatus {
  return versionStatuses.includes(status.trim().toLowerCase() as BackendDesignVersionStatus)
    ? (status.trim().toLowerCase() as BackendDesignVersionStatus)
    : 'draft'
}

export function versionStatusLabel(status: BackendDesignVersionStatus) {
  if (status === 'pending_review') return 'Pending review'
  return status.charAt(0).toUpperCase() + status.slice(1)
}

export function nextVersionStatusOptions(status: BackendDesignVersionStatus): BackendDesignVersionStatus[] {
  if (status === 'draft') return ['pending_review']
  if (status === 'pending_review') return ['reviewed', 'draft']
  if (status === 'reviewed') return ['live', 'draft']
  return ['reviewed', 'draft']
}

export function canTransitionVersionStatus(from: BackendDesignVersionStatus, to: BackendDesignVersionStatus) {
  if (from === to || to === 'draft') return true
  return nextVersionStatusOptions(from).includes(to)
}

export function versionTransitionLabel(currentStatus: BackendDesignVersionStatus, nextStatus: BackendDesignVersionStatus) {
  if (currentStatus === nextStatus) return versionStatusLabel(nextStatus)
  if (nextStatus === 'pending_review') return 'Ready for review'
  if (nextStatus === 'reviewed') return 'Mark reviewed'
  if (nextStatus === 'live') return 'Make live'
  if (nextStatus === 'draft') return 'Move back to draft'
  return versionStatusLabel(nextStatus)
}

export function statusAfterReviewRequest(status: BackendDesignVersionStatus): BackendDesignVersionStatus {
  return status === 'draft' ? 'pending_review' : status
}

export function statusAfterDesignChange(status: BackendDesignVersionStatus): BackendDesignVersionStatus {
  return status === 'live' ? 'live' : 'draft'
}
