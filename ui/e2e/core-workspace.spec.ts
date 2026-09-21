import { expect, test } from '@playwright/test'

test('creates a private workspace and persists a component', async ({ page }) => {
  await page.goto('/')

  await page.getByLabel('Name').fill('Test Admin')
  await page.getByLabel('Work email').fill('admin@stratum.test')
  await page.getByLabel('Password').fill('local-test-password')
  await page.getByRole('button', { name: 'Create admin' }).click()

  await expect(page.getByRole('heading', { name: 'Make system design a shared source of truth.' })).toBeVisible()
  await page.getByRole('button', { name: 'New workspace' }).click()
  await expect(page.getByText('Private by default')).toBeVisible()
  await page.getByLabel('Workspace name').fill('Payments Test')
  await page.getByRole('button', { name: 'Create workspace' }).click()

  await expect(page.getByRole('heading', { name: 'Payments Test' })).toBeVisible()
  await page.getByRole('button', { name: 'New design' }).click()
  await page.getByLabel('Design name').fill('Payment Flow')
  await page.getByRole('button', { name: 'Create design' }).click()

  const contextTabs = page.locator('.context-tabs button')
  await expect(contextTabs).toHaveCount(4)
  expect(await contextTabs.evaluateAll((buttons) => buttons.every((button) => button.scrollWidth <= button.clientWidth))).toBe(true)

  await page.locator('.user-menu summary').click()
  const userMenu = page.locator('.user-menu-popover')
  await expect(userMenu).toBeVisible()
  const menuBox = await userMenu.boundingBox()
  expect(menuBox).not.toBeNull()
  expect(await page.evaluate(({ x, y }) => {
    const topElement = document.elementFromPoint(x, y)
    return Boolean(topElement?.closest('.user-menu-popover'))
  }, { x: menuBox!.x + menuBox!.width / 2, y: menuBox!.y + Math.min(24, menuBox!.height / 2) })).toBe(true)
  await page.locator('.user-menu summary').click()

  const serviceButton = page.locator('.catalog-item').filter({ hasText: 'Service' })
  await expect(serviceButton).toHaveCount(1)
  await serviceButton.click()
  const node = page.locator('.react-flow__node')
  await expect(node).toHaveCount(1)

  await page.waitForTimeout(900)
  await page.reload()
  await expect(page.locator('.react-flow__node')).toHaveCount(1)
})
