import { expect, test, type Page as BrowserPage } from '@playwright/test'
import type { Page as ApiPage, Transaction } from '../../app/types'

const api = 'http://127.0.0.1:18080/api/v1'
let successful: Transaction
let failed: Transaction
let soroban: Transaction

function deferred() {
  let resolve!: () => void
  const promise = new Promise<void>((complete) => {
    resolve = complete
  })

  return { promise, resolve }
}

async function searchFor(page: BrowserPage, hash: string) {
  await page.getByRole('button', { name: 'Open search', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Search indexed Testnet', exact: true })
  const input = dialog.getByRole('combobox')

  await expect(input).toBeFocused()
  await input.fill(hash)
  await expect(dialog.getByRole('option')).toContainText(hash)
  await input.press('Enter')
  await expect(page).toHaveURL(`/tx/${hash}`)
  await expect(dialog).not.toBeVisible()
}

async function chooseStatus(page: BrowserPage, status: 'Success' | 'Failed') {
  await page.getByRole('combobox', { name: 'Status', exact: true }).click()
  await page.getByRole('option', { name: status, exact: true }).click()
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
}

async function mockClipboard(page: BrowserPage, initialMode = 'success') {
  await page.addInitScript((mode) => {
    sessionStorage.setItem('clipboard.mode', mode)

    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      get() {
        if (sessionStorage.getItem('clipboard.mode') === 'missing') {
          return undefined
        }

        return {
          async writeText(value: string) {
            if (sessionStorage.getItem('clipboard.mode') === 'denied') {
              throw new DOMException('Clipboard permission denied', 'NotAllowedError')
            }

            sessionStorage.setItem('clipboard.value', value)
          },
        }
      },
    })
  }, initialMode)
}

test.beforeAll(async ({ request }) => {
  const response = await request.get(`${api}/transactions?limit=100`)
  expect(response.ok()).toBeTruthy()
  const transactions = ((await response.json()) as ApiPage<Transaction>).items

  const success = transactions.find((transaction) => transaction.successful)
  const failure = transactions.find((transaction) => !transaction.successful)
  const invocation = transactions.find((transaction) => transaction.calls.length > 0)
  expect(success).toBeDefined()
  expect(failure).toBeDefined()
  expect(invocation).toBeDefined()

  successful = success!
  failed = failure!
  soroban = invocation!
})

test('copy preserves the complete hash and announces success', async ({ page }) => {
  await mockClipboard(page)
  await page.goto(`/tx/${successful.hash}`)
  const button = page.getByRole('button', { name: `Copy ${successful.hash}`, exact: true })

  await button.click()
  await expect(button).toHaveText('Copied')
  await expect(button.locator('..').locator('[aria-live="polite"]')).toHaveText('Copied')
  expect(await page.evaluate(() => sessionStorage.getItem('clipboard.value'))).toBe(successful.hash)
})

for (const mode of ['missing', 'denied']) {
  test(`copy recovers when the clipboard is ${mode}`, async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', (error) => {
      errors.push(error.message)
    })

    await mockClipboard(page, mode)
    await page.goto(`/tx/${successful.hash}`)
    const button = page.getByRole('button', { name: `Copy ${successful.hash}`, exact: true })
    await button.click()
    await expect(button).toHaveText('Copy unavailable')
    await expect(button.locator('..').locator('[aria-live="polite"]')).toHaveText(
      'Copy unavailable',
    )
    expect(await page.evaluate(() => sessionStorage.getItem('clipboard.value'))).toBeNull()

    await page.evaluate(() => sessionStorage.setItem('clipboard.mode', 'success'))
    await button.click()
    await expect(button).toHaveText('Copied')
    expect(await page.evaluate(() => sessionStorage.getItem('clipboard.value'))).toBe(
      successful.hash,
    )
    expect(errors).toEqual([])
  })
}

test('transaction navigation and browser history replace the previous details and copy state', async ({
  page,
}) => {
  await mockClipboard(page)
  await page.goto(`/tx/${successful.hash}`)
  const original = page.getByRole('button', { name: `Copy ${successful.hash}`, exact: true })
  await original.click()
  await expect(original).toHaveText('Copied')

  await searchFor(page, failed.hash)
  const current = page.getByRole('button', { name: `Copy ${failed.hash}`, exact: true })
  await expect(current).toHaveText('Copy')
  await expect(original).toHaveCount(0)
  await expect(page.locator('.page-heading .pill')).toHaveText('Failed')
  await expect(page.locator('.detail-panel')).toContainText(failed.result_code)
  await current.click()
  expect(await page.evaluate(() => sessionStorage.getItem('clipboard.value'))).toBe(failed.hash)

  await page.goBack()
  await expect(page).toHaveURL(`/tx/${successful.hash}`)
  await expect(original).toHaveText('Copy')
  await expect(current).toHaveCount(0)
  await expect(page.locator('.page-heading .pill')).toHaveText('Success')
  await expect(page.locator('.detail-panel')).toContainText(successful.result_code)
})

test('retry restores a failed list request without losing its filters or page size', async ({
  page,
}) => {
  await page.goto('/transactions?limit=1')
  let available = false
  const queries: string[] = []

  await page.route('**/api/v1/transactions?*', async (route) => {
    queries.push(new URL(route.request().url()).search)
    if (available) {
      await route.continue()
      return
    }

    await route.fulfill({
      status: 503,
      json: {
        error: { code: 'storage_unavailable', message: 'Indexed data temporarily unavailable' },
      },
    })
  })

  await chooseStatus(page, 'Failed')
  await expect(page.getByRole('alert')).toContainText('Indexed data temporarily unavailable')

  available = true
  await page.getByRole('button', { name: 'Try again', exact: true }).click()
  await expect(page.getByRole('alert')).toHaveCount(0)
  await expect(page.getByRole('combobox', { name: 'Status', exact: true })).toHaveText('Failed')
  await expect(page.locator('tbody tr')).toHaveCount(1)
  await expect(page.locator('tbody tr')).toContainText('Failed')
  expect(new URL(page.url()).searchParams.get('limit')).toBe('1')
  expect(new URL(page.url()).searchParams.get('status')).toBe('failed')
  expect(queries.filter((query) => query.includes('status=failed')).length).toBeGreaterThan(1)
  const retried = new URLSearchParams(queries.at(-1))
  expect(retried.get('limit')).toBe('1')
  expect(retried.get('status')).toBe('failed')
})

test('a delayed response cannot replace the results of newer filters', async ({ page }) => {
  await page.goto('/transactions?limit=1')
  const started = deferred()
  const release = deferred()
  const completed = deferred()

  await page.route('**/api/v1/transactions?*', async (route) => {
    const status = new URL(route.request().url()).searchParams.get('status')
    if (status === 'success') {
      started.resolve()
      await release.promise
      await route.fulfill({ json: { items: [successful], next_cursor: null } })
      completed.resolve()
    } else {
      await route.fulfill({ json: { items: [failed], next_cursor: null } })
    }
  })

  try {
    await chooseStatus(page, 'Success')
    await started.promise
    await expect(page.getByRole('status')).toContainText('Loading indexed data')
    await chooseStatus(page, 'Failed')
    await expect(page.locator('tbody tr')).toContainText('Failed')
    await expect(page.locator(`tbody a[href="/tx/${failed.hash}"]`)).toBeVisible()

    release.resolve()
    await completed.promise
    await page.evaluate(
      () =>
        new Promise<void>((resolve) => {
          requestAnimationFrame(() => {
            resolve()
          })
        }),
    )
    await expect(page.locator('tbody tr')).toContainText('Failed')
    await expect(page.locator(`tbody a[href="/tx/${successful.hash}"]`)).toHaveCount(0)
    expect(new URL(page.url()).searchParams.get('status')).toBe('failed')
  } finally {
    release.resolve()
  }
})

test('an invalid cursor can return to the newest filtered results', async ({ page }) => {
  await page.goto('/transactions?status=success&limit=1&cursor=invalid')
  await expect(page.getByRole('alert')).toBeVisible()
  await page.getByRole('link', { name: '↺ Newest', exact: true }).click()

  await expect(page.getByRole('alert')).toHaveCount(0)
  await expect(page.locator('tbody tr')).toHaveCount(1)
  await expect(page.locator('tbody tr')).toContainText('Success')
  const query = new URL(page.url()).searchParams
  expect(query.get('status')).toBe('success')
  expect(query.get('limit')).toBe('1')
  expect(query.has('cursor')).toBeFalsy()
  await expect(page.getByRole('link', { name: 'Older results →', exact: true })).toBeVisible()
})

test('IME composition does not fetch or submit a partially composed search', async ({ page }) => {
  await page.goto('/')
  const queries: string[] = []
  page.on('request', (request) => {
    if (new URL(request.url()).pathname === '/api/v1/search/suggestions') {
      queries.push(new URL(request.url()).searchParams.get('q')!)
    }
  })
  await page.getByRole('button', { name: 'Open search', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Search indexed Testnet', exact: true })
  const input = dialog.getByRole('combobox')
  await input.dispatchEvent('compositionstart')
  await input.fill(successful.hash.slice(0, 12))
  await input.press('Enter')
  await page.waitForTimeout(300)
  expect(queries).toEqual([])
  await expect(dialog).toBeVisible()
  await expect(page).toHaveURL('/')

  await input.dispatchEvent('compositionend')
  await expect(dialog.getByRole('option')).toContainText(successful.hash)
  expect(queries).toEqual([successful.hash.slice(0, 12)])
  await input.press('Enter')
  await expect(page).toHaveURL(`/tx/${successful.hash}`)
})

test('contract strings render as text instead of executable markup', async ({ page, request }) => {
  const response = await request.get(`${api}/transactions/${soroban.hash}`)
  expect(response.ok()).toBeTruthy()
  const transaction = (await response.json()) as Transaction
  const payload = "<img src=invalid onerror=document.title='injected'>"
  transaction.calls[0]!.arguments = [{ type: 'string', value: payload }]

  await page.goto('/')
  await page.route(`**/api/v1/transactions/${soroban.hash}`, (route) =>
    route.fulfill({ json: transaction }),
  )
  await searchFor(page, soroban.hash)
  const argumentsView = page.locator('details').filter({
    has: page.locator('summary', { hasText: 'Typed arguments' }),
  })
  await argumentsView.locator('summary').click()

  await expect(argumentsView.locator('pre')).toContainText(payload)
  await expect(argumentsView.locator('img')).toHaveCount(0)
  await expect(page).not.toHaveTitle('injected')
})

test('keyboard users can skip navigation and focus the page content', async ({ page }) => {
  await page.goto('/events')
  await page.keyboard.press('Tab')
  const skip = page.getByRole('link', { name: 'Skip to content', exact: true })
  await expect(skip).toBeFocused()
  await expect(skip).toBeInViewport()
  await skip.press('Enter')
  await expect(page.locator('#main-content')).toBeFocused()
  await expect(page.getByRole('heading', { name: 'Events', exact: true })).toBeVisible()
})
