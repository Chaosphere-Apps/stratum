import type { BackendWorkspace } from '../backendSync'

export type AdminSection =
  | 'overview'
  | 'users'
  | 'access'
  | 'workspaces'
  | 'catalog'
  | 'identity'
  | 'ai'
  | 'integrations'
  | 'security'
  | 'settings'

export type AppRoute =
  | { screen: 'home' }
  | { screen: 'admin'; section?: AdminSection }
  | { screen: 'workspace'; workspaceId: string }
  | { screen: 'design'; workspaceId: string; designId: string }

export const adminSectionOrder: AdminSection[] = [
  'overview',
  'users',
  'access',
  'workspaces',
  'catalog',
  'identity',
  'ai',
  'integrations',
  'security',
  'settings',
]

export const adminSectionCopy: Record<AdminSection, { label: string; description: string; group: string }> = {
  overview: {
    label: 'Dashboard',
    description: 'Deployment health, configuration state, and setup shortcuts.',
    group: 'Command',
  },
  users: {
    label: 'Users and roles',
    description: 'Manage people, passwords, roles, and account state.',
    group: 'Identity',
  },
  access: {
    label: 'Access center',
    description: 'Workspace and design ACL policy surface.',
    group: 'Identity',
  },
  workspaces: {
    label: 'Workspaces',
    description: 'Govern workspace ownership, deletion, and design creation.',
    group: 'Content',
  },
  catalog: {
    label: 'Enterprise catalog',
    description: 'Canonical services and infrastructure reused across designs.',
    group: 'Content',
  },
  identity: {
    label: 'Sign-in and SSO',
    description: 'Local sign-in, Okta OIDC, claims, and provisioning.',
    group: 'Security',
  },
  ai: {
    label: 'AI suite',
    description: 'Central provider for analysis, vision, and future copilots.',
    group: 'Intelligence',
  },
  integrations: {
    label: 'Integrations',
    description: 'MCP readiness and future enterprise integration endpoints.',
    group: 'Platform',
  },
  security: {
    label: 'Security and audit',
    description: 'Policy posture, audit readiness, and security controls.',
    group: 'Security',
  },
  settings: {
    label: 'System settings',
    description: 'Deployment defaults and platform-level configuration.',
    group: 'Platform',
  },
}

export const defaultWorkspace: BackendWorkspace = {
  id: 'guest-workspace',
  name: 'Guest Workspace',
}

export function parseAdminSection(value: string | undefined): AdminSection {
  if (
    value === 'users' ||
    value === 'access' ||
    value === 'workspaces' ||
    value === 'catalog' ||
    value === 'identity' ||
    value === 'ai' ||
    value === 'integrations' ||
    value === 'security' ||
    value === 'settings'
  ) {
    return value
  }
  return 'overview'
}

export function parseRoute(pathname: string): AppRoute {
  const segments = pathname.split('/').filter(Boolean).map(decodeURIComponent)
  if (segments[0] === 'workspaces' && segments[1] && segments[2] === 'designs' && segments[3]) {
    return { screen: 'design', workspaceId: segments[1], designId: segments[3] }
  }
  if (segments[0] === 'workspaces' && segments[1]) {
    return { screen: 'workspace', workspaceId: segments[1] }
  }
  if (segments[0] === 'admin') {
    return { screen: 'admin', section: parseAdminSection(segments[1]) }
  }
  return { screen: 'home' }
}

export function routePath(route: AppRoute) {
  if (route.screen === 'admin') return route.section && route.section !== 'overview' ? `/admin/${route.section}` : '/admin'
  if (route.screen === 'workspace') return `/workspaces/${encodeURIComponent(route.workspaceId)}`
  if (route.screen === 'design') {
    return `/workspaces/${encodeURIComponent(route.workspaceId)}/designs/${encodeURIComponent(route.designId)}`
  }
  return '/'
}
