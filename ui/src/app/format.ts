import type { BackendUser } from '../backendApi'

export function userDisplayName(users: BackendUser[], userId: string) {
  return users.find((user) => user.id === userId)?.displayName ?? userId.replaceAll('-', ' ')
}

export function initialsFor(value: string) {
  const words = value.trim().split(/[\s@._-]+/).filter(Boolean)
  return words
    .slice(0, 2)
    .map((word) => word[0]?.toUpperCase())
    .join('') || 'U'
}

export function formatCompactDateTime(value?: string) {
  if (!value) return 'Never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Never'
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

export function roleLabel(role: string) {
  if (role === 'admin') return 'Admin'
  if (role === 'architect') return 'Architect'
  if (role === 'reviewer') return 'Reviewer'
  return 'Member'
}
