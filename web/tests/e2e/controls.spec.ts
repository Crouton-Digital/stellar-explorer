import { expect, test } from '@playwright/test'

test('custom select supports keyboard navigation, cancellation, typeahead and Tab', async ({
  page,
}) => {
  await page.goto('/transactions')
  const control = page.getByRole('combobox', { name: 'Status', exact: true })
  await control.focus()
  await control.press('ArrowDown')
  await expect(control).toHaveAttribute('aria-expanded', 'true')
  await expect(page.getByRole('option', { name: 'All results', exact: true })).toHaveAttribute(
    'aria-selected',
    'true',
  )
  await control.press('End')
  await expect(control).toHaveAttribute(
    'aria-activedescendant',
    'filter-transactions-status-option-2',
  )
  await expect(control).toHaveText('All results')
  await control.press('Escape')
  await expect(control).toHaveAttribute('aria-expanded', 'false')
  await expect(control).toHaveText('All results')
  await expect(control).toBeFocused()

  await control.press('f')
  await control.press('Enter')
  await expect(control).toHaveText('Failed')
  await expect(page.locator('input[name="status"]')).toHaveValue('failed')
  await expect(control).toBeFocused()
  await control.press('Space')
  await control.press('Home')
  await control.press('Tab')
  await expect(control).toHaveText('All results')
  await expect(page.getByRole('listbox')).toHaveCount(0)
  await expect(page.getByLabel('From ledger', { exact: true })).toBeFocused()
  await expect(page).not.toHaveURL(/[?&]status=/)
})

test('custom select works by pointer and closes outside without changing the value', async ({
  page,
}, testInfo) => {
  await page.goto('/events')
  const control = page.getByRole('combobox', { name: 'Event type', exact: true })
  await control.click()
  await expect(page.getByRole('listbox', { name: 'Event type', exact: true })).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('custom-select.png') })
  await page.getByRole('option', { name: 'System', exact: true }).click()
  await expect(control).toHaveText('System')
  await control.click()
  await page.getByRole('heading', { name: 'Events', exact: true }).click()
  await expect(page.getByRole('listbox')).toHaveCount(0)
  await expect(control).toHaveText('System')
  await expect(page.locator('select')).toHaveCount(0)
})

test('dropdown stays inside narrow and short viewports and supports touch', async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 320, height: 568 })
  await page.goto('/events')
  const control = page.getByRole('combobox', { name: 'Event type', exact: true })
  if (testInfo.project.name === 'mobile') {
    await control.tap()
  } else {
    await control.click()
  }

  const popup = page.getByRole('listbox', { name: 'Event type', exact: true })
  await expect(popup).toBeVisible()
  const box = await popup.boundingBox()
  expect(box).toBeTruthy()
  expect(box!.x).toBeGreaterThanOrEqual(0)
  expect(box!.x + box!.width).toBeLessThanOrEqual(320)
  expect(box!.y).toBeGreaterThanOrEqual(0)
  expect(box!.y + box!.height).toBeLessThanOrEqual(568)

  if (testInfo.project.name === 'mobile') {
    await page.getByRole('option', { name: 'Contract', exact: true }).tap()
  } else {
    await page.getByRole('option', { name: 'Contract', exact: true }).click()
  }
  await expect(control).toHaveText('Contract')
  await control.click()
  await page.setViewportSize({ width: 568, height: 320 })
  await expect
    .poll(async () => {
      const current = await popup.boundingBox()
      return current ? current.y >= 0 && current.y + current.height <= 320 : true
    })
    .toBeTruthy()
  await page.getByLabel('Contract', { exact: true }).focus()
  await expect(popup).toHaveCount(0)
})
