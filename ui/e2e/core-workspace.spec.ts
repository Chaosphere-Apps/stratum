import { expect, test } from '@playwright/test'

test('creates a private workspace and persists component deletion', async ({ page }) => {
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
  await page.getByLabel('Use case').fill('Process a payment safely')
  await page.getByRole('button', { name: 'Create design' }).click()

  const serviceButton = page.locator('.catalog-item').filter({ hasText: 'Service' })
  await expect(serviceButton).toHaveCount(1)
  await serviceButton.click()
  const node = page.locator('.rf-architecture-node')
  await expect(node).toHaveCount(1)

  await node.click({ button: 'right' })
  await expect(page.getByRole('menu', { name: 'Actions for Service' })).toBeVisible()
  await page.getByRole('menuitem', { name: 'Delete' }).click()
  await expect(node).toHaveCount(0)

  await page.waitForTimeout(900)
  await page.reload()
  await expect(page.locator('.rf-architecture-node')).toHaveCount(0)
})
