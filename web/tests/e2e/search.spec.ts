import { test, expect, type Page as BrowserPage } from '@playwright/test'
import type { Page, Transaction } from '../../app/types'

const api = 'http://127.0.0.1:18080/api/v1'
let transaction: Transaction

async function openSearch(page: BrowserPage) {
  await page.getByRole('button', { name: 'Open search', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Search indexed Testnet', exact: true })
  const input = dialog.getByRole('combobox', {
    name: 'Search transactions, accounts, contracts or ledgers',
  })
  await expect(dialog).toBeVisible()
  await expect(input).toBeFocused()
  return { dialog, input }
}

test.beforeAll(async ({ request }) => {
  const response = await request.get(`${api}/transactions?limit=1`)
  transaction = ((await response.json()) as Page<Transaction>).items[0]!
})

test('empty search stays compact and shows search types in the input', async ({ page }) => {
  await page.goto('/')
  const queries: string[] = []
  page.on('request', (request) => {
    if (request.url().includes('/api/v1/search/suggestions')) {
      queries.push(request.url())
    }
  })
  const { dialog, input } = await openSearch(page)
  await expect(input).toHaveAttribute('placeholder', 'Tx hash / account / contract / ledger…')
  await expect(input).toHaveAttribute('aria-expanded', 'false')
  await expect(dialog.getByRole('listbox')).toHaveCount(0)
  await expect(dialog.getByRole('button', { name: 'Search ↵', exact: true })).toHaveCount(0)

  for (const width of [320, 390, 768, 1440]) {
    await page.setViewportSize({ width, height: 700 })
    await expect
      .poll(async () => {
        const bounds = await dialog.boundingBox()
        return Math.round(bounds!.width)
      })
      .toBe(Math.min(640, width - 32))
    const bounds = await dialog.boundingBox()
    expect(bounds!.height).toBeLessThanOrEqual(72)
    expect(Math.round(bounds!.y + bounds!.height / 2)).toBe(350)
    await expect(input).toBeInViewport()
    await expect(dialog.getByRole('button', { name: 'Close search', exact: true })).toBeInViewport()
  }

  await input.fill('abc')
  await expect(dialog).toContainText('at least 4')
  await page.getByRole('button', { name: 'Clear search', exact: true }).click()
  await expect(input).toBeFocused()
  await expect(dialog.locator('.search-results')).toHaveCount(0)
  expect((await dialog.boundingBox())!.height).toBeLessThanOrEqual(72)
  expect(queries).toEqual([])
})

test('live search previews partial indexed matches and opens them with Enter', async ({ page }) => {
  await page.goto('/')
  const { dialog, input } = await openSearch(page)
  await input.fill(transaction.hash.slice(0, 12))
  const results = page.getByRole('listbox', { name: 'Search results' })
  await expect(results.getByRole('option')).toContainText([transaction.hash])
  await expect(page).toHaveURL('/')
  await expect(input).toHaveAttribute('aria-expanded', 'true')
  await input.press('ArrowDown')
  await input.press('Enter')
  await expect(page).toHaveURL(`/tx/${transaction.hash}`)
  await expect(dialog).not.toBeVisible()
  await expect(page.getByRole('button', { name: 'Open search', exact: true })).toHaveAttribute(
    'aria-expanded',
    'false',
  )
})

test('live search handles typed categories, clear, Escape and outside clicks', async ({ page }) => {
  await page.goto('/')
  const { dialog, input } = await openSearch(page)
  await input.fill(transaction.source_account)
  const option = page.getByRole('option')
  await expect(option).toContainText('Account')
  await expect(option).toContainText(transaction.source_account)
  await input.press('Escape')
  await expect(dialog).not.toBeVisible()
  await openSearch(page)
  await expect(input).toHaveValue('')
  await input.fill(transaction.source_account.toLowerCase())
  await expect(page.getByRole('option')).toContainText(transaction.source_account)
  await input.press('Escape')
  await expect(dialog).not.toBeVisible()
  await openSearch(page)
  await input.fill(transaction.source_account)
  await page.getByRole('button', { name: 'Clear search', exact: true }).click()
  await expect(input).toHaveValue('')
  await expect(dialog.locator('.search-results')).toHaveCount(0)
  await expect(input).toHaveAttribute('aria-expanded', 'false')
  expect((await dialog.boundingBox())!.height).toBeLessThanOrEqual(72)
  await input.fill(String(transaction.ledger_sequence).slice(0, 4))
  await expect(page.getByRole('option').first()).toContainText('Ledger')
  await page.mouse.click(4, 4)
  await expect(dialog).not.toBeVisible()
  await expect(page.getByRole('button', { name: 'Open search', exact: true })).toBeFocused()
})

test('search debounces input and ignores an older response after the query changes', async ({
  page,
}) => {
  await page.goto('/')
  const queries: string[] = []
  await page.route('**/api/v1/search/suggestions*', async (route) => {
    const q = new URL(route.request().url()).searchParams.get('q')!
    queries.push(q)
    if (q === 'aaaa') {
      await new Promise((resolve) => setTimeout(resolve, 450))
    }
    await route.fulfill({
      json: {
        network: 'testnet',
        query: q,
        limit: 10,
        items: [
          {
            type: 'transaction',
            key: q === 'bbbb' ? transaction.hash : 'old-result',
            path: `/tx/${transaction.hash}`,
          },
        ],
      },
    })
  })
  const { input } = await openSearch(page)
  await input.fill('abcd')
  await input.fill('aaaa')
  await expect.poll(() => queries).toEqual(['aaaa'])
  await input.fill('bbbb')
  await expect(page.getByRole('option')).toContainText(transaction.hash)
  await page.waitForTimeout(500)
  await expect(page.getByRole('option')).toContainText(transaction.hash)
  await expect(page.getByText('old-result', { exact: true })).toHaveCount(0)
  expect(queries).toEqual(['aaaa', 'bbbb'])
})

test('live search shows empty and unavailable states with an explicit retry', async ({ page }) => {
  await page.goto('/')
  let attempts = 0
  await page.route('**/api/v1/search/suggestions*', (route) => {
    attempts++
    return attempts === 1
      ? route.fulfill({ status: 503, json: { error: { message: 'Unavailable' } } })
      : route.fulfill({ json: { network: 'testnet', query: 'ffff', limit: 10, items: [] } })
  })
  const { input } = await openSearch(page)
  await input.fill('ffff')
  await expect(page.locator('.search-popup')).toContainText('temporarily unavailable')
  await page.getByRole('button', { name: 'Retry search', exact: true }).click()
  await expect(page.locator('.search-popup')).toContainText('No matches in indexed history')
})

test('search dialog stays centered inside narrow and short screens with usable controls', async ({
  page,
}) => {
  await page.goto('/')
  const { dialog, input } = await openSearch(page)
  await input.fill(transaction.hash.slice(0, 8))
  await expect(page.getByRole('option')).toBeVisible()
  for (const viewport of [
    { width: 320, height: 700 },
    { width: 390, height: 700 },
    { width: 768, height: 700 },
    { width: 1440, height: 900 },
    { width: 568, height: 320 },
    { width: 390, height: 300 },
  ]) {
    await page.setViewportSize(viewport)
    await expect
      .poll(async () => {
        const bounds = await dialog.boundingBox()
        return {
          x: Math.round(bounds!.x + bounds!.width / 2),
          y: Math.round(bounds!.y + bounds!.height / 2),
        }
      })
      .toEqual({ x: viewport.width / 2, y: viewport.height / 2 })
    const bounds = await dialog.boundingBox()
    expect(bounds!.x).toBeGreaterThanOrEqual(16)
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(viewport.width - 16)
    expect(bounds!.y).toBeGreaterThanOrEqual(16)
    expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(viewport.height - 16)
    const option = await page.getByRole('option').first().boundingBox()
    expect(option!.height).toBeGreaterThanOrEqual(44)
    await expect(dialog.getByRole('button', { name: 'Close search', exact: true })).toBeInViewport()
    await expect(dialog.getByRole('button', { name: 'Search ↵', exact: true })).toBeInViewport()
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    ).toBeTruthy()
  }
})

test('search blurs the backdrop, traps focus and restores it when Escape closes the dialog', async ({
  page,
}) => {
  await page.goto('/')
  const { dialog, input } = await openSearch(page)
  expect(await dialog.evaluate((element) => element.matches(':modal'))).toBeTruthy()
  expect(
    await dialog.evaluate((element) => getComputedStyle(element, '::backdrop').backdropFilter),
  ).toBe('blur(6px)')
  await expect(dialog).toHaveAttribute('aria-modal', 'true')
  await input.fill(transaction.hash.slice(0, 12))
  await expect(page.getByRole('option')).toBeVisible()
  await input.press('Tab')
  await expect(dialog.getByRole('button', { name: 'Clear search', exact: true })).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(dialog.getByRole('button', { name: 'Close search', exact: true })).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(dialog.getByRole('button', { name: 'Search ↵', exact: true })).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(input).toBeFocused()
  await input.press('Shift+Tab')
  await expect(dialog.getByRole('button', { name: 'Search ↵', exact: true })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(dialog).not.toBeVisible()
  await expect(page.getByRole('button', { name: 'Open search', exact: true })).toBeFocused()
})

test('search locks background scrolling and restores the previous page position on close', async ({
  page,
}) => {
  await page.goto('/transactions')
  await page.evaluate(() => window.scrollTo(0, 400))
  const before = await page.evaluate(() => ({
    scroll: window.scrollY,
    overflow: document.documentElement.style.overflow,
  }))
  expect(before.scroll).toBeGreaterThan(0)
  await page.keyboard.press('/')
  const dialog = page.getByRole('dialog', { name: 'Search indexed Testnet', exact: true })
  const input = dialog.getByRole('combobox')
  await expect(input).toBeFocused()
  expect(await page.evaluate(() => getComputedStyle(document.documentElement).overflow)).toBe(
    'hidden',
  )
  await page.locator('#main-content').evaluate((element: HTMLElement) => element.focus())
  await expect(input).toBeFocused()
  await page.mouse.move(4, 4)
  await page.mouse.wheel(0, 600)
  await page.waitForTimeout(100)
  expect(await page.evaluate(() => window.scrollY)).toBe(before.scroll)
  await dialog.getByRole('button', { name: 'Close search', exact: true }).click()
  await expect(dialog).not.toBeVisible()
  expect(await page.evaluate(() => document.documentElement.style.overflow)).toBe(before.overflow)
  expect(await page.evaluate(() => window.scrollY)).toBe(before.scroll)
})

test('long result lists scroll inside the palette while the input and controls stay visible', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 400 })
  await page.goto('/')
  await page.route('**/api/v1/search/suggestions*', (route) =>
    route.fulfill({
      json: {
        network: 'testnet',
        query: 'ffff',
        limit: 10,
        items: Array.from({ length: 10 }, (_, index) => ({
          type: 'transaction',
          key: String(index).padStart(64, 'f'),
          path: `/tx/${transaction.hash}`,
        })),
      },
    }),
  )
  const { dialog, input } = await openSearch(page)
  await input.fill('ffff')
  await expect(dialog.getByRole('option')).toHaveCount(10)
  await input.press('ArrowUp')
  const last = dialog.getByRole('option').last()
  await expect(last).toHaveAttribute('aria-selected', 'true')
  await expect(last).toBeInViewport()
  expect(
    await dialog.locator('.search-results').evaluate((element) => element.scrollTop),
  ).toBeGreaterThan(0)
  await expect(input).toBeInViewport()
  await expect(dialog.getByRole('button', { name: 'Close search', exact: true })).toBeInViewport()
  await expect(dialog.getByRole('button', { name: 'Search ↵', exact: true })).toBeInViewport()
  await last.click()
  await expect(page).toHaveURL(`/tx/${transaction.hash}`)
  await expect(dialog).not.toBeVisible()
})

test('search shortcuts open the palette without intercepting filter input', async ({ page }) => {
  await page.goto('/transactions')
  const dialog = page.getByRole('dialog', { name: 'Search indexed Testnet', exact: true })
  await expect(page.getByRole('button', { name: 'Open search', exact: true })).toBeVisible()
  for (const shortcut of ['/', 'Control+k', 'Meta+k']) {
    await page.keyboard.press(shortcut)
    await expect(dialog).toBeVisible()
    await expect(dialog.getByRole('combobox')).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(dialog).not.toBeVisible()
  }

  const filter = page.getByLabel('From ledger', { exact: true })
  await filter.focus()
  await filter.press('/')
  await filter.press('Control+k')
  await expect(dialog).not.toBeVisible()
  await expect(filter).toBeFocused()
})

test('closing the palette cancels a pending result and permits a fresh search', async ({
  page,
}) => {
  await page.goto('/')
  let release: () => void = () => {}
  const pending = new Promise<void>((resolve) => {
    release = resolve
  })
  await page.route('**/api/v1/search/suggestions*', async (route) => {
    const query = new URL(route.request().url()).searchParams.get('q')!
    if (query === 'aaaa') {
      await pending
    }
    await route.fulfill({
      json: {
        network: 'testnet',
        query,
        limit: 10,
        items: [{ type: 'transaction', key: query, path: `/tx/${transaction.hash}` }],
      },
    })
  })
  const { dialog, input } = await openSearch(page)
  const request = page.waitForRequest('**/api/v1/search/suggestions?q=aaaa')
  await input.fill('aaaa')
  await request
  await input.press('Escape')
  await expect(dialog).not.toBeVisible()
  await openSearch(page)
  await expect(input).toHaveValue('')
  await input.fill('bbbb')
  await expect(page.getByRole('option')).toContainText('bbbb')
  release()
  await expect(page.getByRole('option')).not.toContainText('aaaa')
  await expect(page).toHaveURL('/')
})
