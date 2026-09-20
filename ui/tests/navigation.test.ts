import { describe, expect, it } from 'vitest'
import { parseAdminSection, parseRoute, routePath } from '../src/app/navigation'

describe('application routes', () => {
  it.each([
    ['/', { screen: 'home' }],
    ['/reset-password', { screen: 'reset-password' }],
    ['/admin/access', { screen: 'admin', section: 'access' }],
    ['/workspaces/platform', { screen: 'workspace', workspaceId: 'platform' }],
    ['/workspaces/platform/designs/payments', { screen: 'design', workspaceId: 'platform', designId: 'payments' }],
  ])('parses %s', (path, route) => expect(parseRoute(path)).toEqual(route))

  it('falls back safely and encodes generated paths', () => {
    expect(parseAdminSection('not-real')).toBe('overview')
    expect(routePath({ screen: 'admin', section: 'overview' })).toBe('/admin')
    expect(routePath({ screen: 'workspace', workspaceId: 'space / one' })).toBe('/workspaces/space%20%2F%20one')
    expect(routePath({ screen: 'design', workspaceId: 'a/b', designId: 'd e' })).toBe('/workspaces/a%2Fb/designs/d%20e')
  })
})
