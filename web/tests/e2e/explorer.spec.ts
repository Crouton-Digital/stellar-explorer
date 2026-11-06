import { test, expect, type Page as BrowserPage } from '@playwright/test'
import type { ContractDetail, Page as ApiPage, Transaction, TokenTransfer } from '../../app/types'

const api = 'http://127.0.0.1:18080/api/v1'
let transactions: Transaction[]
let tokenTransfers: TokenTransfer[]

async function chooseFilter(page: BrowserPage, name: string, label: string) {
  const control = page.getByRole('combobox', { name, exact: true })
  await control.click()
  await page.getByRole('option', { name: label, exact: true }).click()
  await expect(control).toHaveText(label)
}

test.beforeAll(async ({ request }) => {
  const response = await request.get(`${api}/transactions?limit=100`)
  expect(response.ok()).toBeTruthy()
  transactions = ((await response.json()) as ApiPage<Transaction>).items
  expect(transactions.length).toBeGreaterThan(0)

  const movementResponse = await request.get(`${api}/transfers?limit=100`)
  expect(movementResponse.ok()).toBeTruthy()
  tokenTransfers = ((await movementResponse.json()) as ApiPage<TokenTransfer>).items
  expect(tokenTransfers.length).toBeGreaterThan(0)
})

test('overview links real data and fits the viewport', async ({ page }, testInfo) => {
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Network overview', exact: true })).toBeVisible()
  await expect(page.getByText('Testnet', { exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: /Recent transactions/ })).toBeVisible()
  await expect(page.locator('table tbody tr').first()).toBeVisible()
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
  ).toBeTruthy()
  expect(errors).toEqual([])
  await page.screenshot({ path: testInfo.outputPath('overview.png'), fullPage: true })
})

test('Crouton branding loads and header navigation returns home', async ({ page }) => {
  await page.goto('/')

  const logo = page.getByRole('img', { name: 'Crouton Digital', exact: true })
  const navigation = page.getByRole('navigation', { name: 'Main navigation' })

  await expect(logo).toBeVisible()
  await expect(logo).toHaveJSProperty('complete', true)
  await expect
    .poll(() => logo.evaluate((element: HTMLImageElement) => element.naturalWidth))
    .toBeGreaterThan(0)
  await expect(page.locator('link[rel="icon"]')).toHaveAttribute('href', '/brand/crouton-logo.svg')

  await navigation.getByRole('link', { name: 'Transactions', exact: true }).click()
  await expect(page).toHaveURL('/transactions')
  await expect(navigation.getByRole('link', { name: 'Transactions', exact: true })).toHaveAttribute(
    'aria-current',
    'page',
  )

  await page.getByRole('link', { name: 'Crouton home', exact: true }).click()
  await expect(page).toHaveURL('/')
  await expect(navigation.getByRole('link', { name: 'Overview', exact: true })).toHaveAttribute(
    'aria-current',
    'page',
  )
})

test('search opens a real transaction and exposes XDR', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Open search', exact: true }).click()
  await page
    .getByLabel('Search transactions, accounts, contracts or ledgers')
    .fill(transactions[0]!.hash)
  await page.getByRole('button', { name: 'Search ↵' }).click()
  await expect(page).toHaveURL(`/tx/${transactions[0]!.hash}`)
  await expect(page.getByRole('heading', { name: 'Transaction', exact: true })).toBeVisible()
  await page.getByText('Transaction metadata XDR', { exact: true }).click()
  await expect(page.locator('details[open] pre')).not.toBeEmpty()
})

test('invalid search and absent transaction show clear errors', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Open search', exact: true }).click()
  await page
    .getByLabel('Search transactions, accounts, contracts or ledgers')
    .fill('not-an-address')
  await page.getByRole('button', { name: 'Search ↵' }).click()
  await expect(page.getByRole('alert')).toContainText('Enter a transaction hash')
  await page.goto(`/tx/${'0'.repeat(64)}`)
  await expect(page.getByRole('alert')).toContainText('Entity not found in indexed data')
})

test('account warns about partial history and contract displays events', async ({ page }) => {
  await page.goto(`/accounts/${transactions[0]!.source_account}`)
  await expect(
    page.getByText(/Balance and complete account history are not available/),
  ).toBeVisible()
  const tx = transactions.find((t) => t.calls.length)
  expect(tx).toBeTruthy()
  await page.goto(`/contracts/${tx!.calls[0]!.contract_id}`)
  await expect(page.getByRole('heading', { name: 'Recent calls' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Contract events' })).toBeVisible()
  await expect(page.getByText('Topics (typed SCVal)').first()).toBeVisible()
})

test('client navigation displays loading, empty and API failure states', async ({ page }) => {
  await page.goto('/')
  let release: (() => void) | undefined
  const hold = new Promise<void>((resolve) => {
    release = resolve
  })
  await page.route('**/api/v1/transactions*', async (route) => {
    await hold
    await route.fulfill({ json: { items: [], next_cursor: null } })
  })
  await page
    .getByRole('navigation')
    .getByRole('link', { name: /Transactions/ })
    .click()
  await expect(page.getByRole('status')).toContainText('Loading indexed data')
  release!()
  await expect(page.getByText('No indexed activity yet')).toBeVisible()
  await page.route('**/api/v1/ledgers*', (route) =>
    route.fulfill({
      status: 503,
      json: {
        error: { code: 'storage_unavailable', message: 'Indexed data temporarily unavailable' },
      },
    }),
  )
  await page
    .getByRole('navigation')
    .getByRole('link', { name: /Ledgers/ })
    .click()
  await expect(page.getByRole('alert')).toContainText('Indexed data temporarily unavailable')
})

test('assets link real token movements to transactions and recipient history', async ({ page }) => {
  const movement = tokenTransfers.find(
    (item) => item.asset.kind === 'sac' && item.to?.startsWith('G'),
  )
  expect(movement).toBeTruthy()

  await page.goto('/assets')
  await expect(page.getByRole('heading', { name: 'Assets', exact: true })).toBeVisible()
  await expect(page.getByRole('table', { name: 'Observed assets' })).toContainText('Verified SAC')
  await page.locator(`a[href="/assets/${movement!.asset.contract_id}"]`).first().click()
  await expect(page.getByRole('heading', { name: 'Asset activity', exact: true })).toBeVisible()
  await expect(page.getByText('Verified Stellar Asset Contract', { exact: true })).toBeVisible()
  await expect(page.getByRole('table', { name: 'Token movements' })).toContainText(
    movement!.asset.code!,
  )

  await page.goto(`/tx/${movement!.transaction_hash}`)
  await expect(page.getByRole('heading', { name: /^Token movements/ })).toBeVisible()
  await expect(page.getByRole('table', { name: 'Token movements' })).toContainText(
    movement!.asset.code!,
  )

  await page.goto(`/accounts/${movement!.to}`)
  await expect(page.getByRole('heading', { name: 'Token movements', exact: true })).toBeVisible()
  await expect(
    page.getByRole('table', { name: 'Token movements' }).locator('tbody tr').first(),
  ).toBeVisible()
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
  ).toBeTruthy()
})

test('custom token pages keep unknown decimals and raw base units visible', async ({ page }) => {
  const movement = tokenTransfers[0]!
  const asset = {
    ...movement.asset,
    kind: 'contract',
    verification: 'event_shape',
    code: null,
    issuer: null,
    classic_asset: null,
    decimals: null,
  }

  await page.goto('/assets')
  await page.route(`**/api/v1/assets/${asset.contract_id}`, (route) =>
    route.fulfill({
      json: {
        asset,
        transfers: { items: [{ ...movement, asset }], next_cursor: null },
        range: {
          first_indexed_ledger: movement.ledger_sequence,
          last_indexed_ledger: movement.ledger_sequence,
        },
      },
    }),
  )
  await page.locator(`a[href="/assets/${asset.contract_id}"]`).first().click()
  await expect(page.getByText('SEP-41-compatible event format', { exact: true })).toBeVisible()
  await expect(
    page.getByText('Unknown · amounts shown in base units', { exact: true }),
  ).toBeVisible()
  await expect(page.getByRole('table', { name: 'Token movements' })).toContainText(movement.amount)
  await expect(page.getByRole('table', { name: 'Token movements' })).toContainText('base units')
})

test('contract metadata matches indexed evidence and fits the viewport', async ({
  page,
  request,
}) => {
  const transaction = transactions.find((item) => item.calls.length > 0)!
  const address = transaction.calls[0]!.contract_id
  const response = await request.get(`${api}/contracts/${address}`)
  expect(response.ok()).toBeTruthy()
  const detail = (await response.json()) as ContractDetail

  await page.goto(`/contracts/${address}`)
  const metadata = page.getByRole('region', { name: 'Contract metadata' })
  await expect(metadata).toBeVisible()
  await expect(metadata).toContainText(String(detail.contract.first_observed_ledger))

  if (detail.contract.wasm_hash) {
    await expect(metadata).toContainText(detail.contract.wasm_hash)
  }

  if (!detail.contract.creation_ledger) {
    await expect(metadata).toContainText('Unknown · creation not observed')
  }

  await expect(
    page.getByRole('heading', { name: 'Related transactions', exact: true }),
  ).toBeVisible()
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
  ).toBeTruthy()
})

test('contract metadata distinguishes unknown history from confirmed creation', async ({
  page,
  request,
}) => {
  const address = transactions.find((item) => item.calls.length > 0)!.calls[0]!.contract_id
  const response = await request.get(`${api}/contracts/${address}`)
  const detail = (await response.json()) as ContractDetail

  await page.goto('/')
  await page.route(`**/api/v1/contracts/${address}`, (route) =>
    route.fulfill({
      json: {
        ...detail,
        contract: {
          ...detail.contract,
          kind: 'unknown',
          wasm_hash: null,
          executable_source: null,
          executable_ledger: null,
          creation_ledger: null,
          creation_transaction: null,
          creator: null,
        },
      },
    }),
  )
  await page.getByRole('button', { name: 'Open search', exact: true }).click()
  await page.getByLabel('Search transactions, accounts, contracts or ledgers').fill(address)
  await page.getByRole('button', { name: 'Search ↵' }).click()

  const metadata = page.getByRole('region', { name: 'Contract metadata' })
  await expect(metadata.locator('dt:has-text("WASM hash") + dd')).toHaveText('Unknown')
  await expect(metadata.locator('dt:has-text("Creator") + dd')).toHaveText('Unknown')
  await expect(metadata).toContainText('Unknown · creation not observed')

  await page.unroute(`**/api/v1/contracts/${address}`)
  await page.goto('/')
  await page.route(`**/api/v1/contracts/${address}`, (route) =>
    route.fulfill({
      json: {
        ...detail,
        contract: {
          ...detail.contract,
          creation_ledger: transactions[0]!.ledger_sequence,
          creation_transaction: transactions[0]!.hash,
          creator: transactions[0]!.source_account,
        },
      },
    }),
  )
  await page.getByRole('button', { name: 'Open search', exact: true }).click()
  await page.getByLabel('Search transactions, accounts, contracts or ledgers').fill(address)
  await page.getByRole('button', { name: 'Search ↵' }).click()
  await expect(metadata.locator(`a[href="/tx/${transactions[0]!.hash}"]`)).toBeVisible()
  await expect(
    metadata.locator(`a[href="/accounts/${transactions[0]!.source_account}"]`),
  ).toBeVisible()
})

test('transaction filters survive pagination, reload and browser navigation', async ({ page }) => {
  await page.goto('/transactions?limit=1')
  await chooseFilter(page, 'Status', 'Success')
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect(page).toHaveURL(/status=success/)
  await expect(page.locator('tbody tr')).toHaveCount(1)
  await expect(page.locator('tbody tr').first()).toContainText('Success')
  const firstHash = await page.locator('tbody tr a').first().getAttribute('href')

  await page.getByRole('link', { name: 'Older results →', exact: true }).click()
  await expect(page).toHaveURL(/cursor=/)
  expect(new URL(page.url()).searchParams.get('status')).toBe('success')
  await expect(page.locator('tbody tr')).toHaveCount(1)
  await expect(page.locator('tbody tr a').first()).not.toHaveAttribute('href', firstHash!)
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Status', exact: true })).toHaveText('Success')

  await chooseFilter(page, 'Status', 'Failed')
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect(page).toHaveURL(/status=failed/)
  expect(new URL(page.url()).searchParams.has('cursor')).toBeFalsy()
  await expect(page.locator('tbody tr').first()).toContainText('Failed')
  await page.goBack()
  await expect(page.getByRole('combobox', { name: 'Status', exact: true })).toHaveText('Success')

  await page.getByRole('link', { name: '↺ Newest', exact: true }).click()
  await expect(page).not.toHaveURL(/[?&]cursor=/)
  expect(new URL(page.url()).searchParams.get('status')).toBe('success')
  await expect(page.locator('tbody tr a').first()).toHaveAttribute('href', firstHash!)
  await page.getByRole('button', { name: 'Reset filters', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Status', exact: true })).toHaveText(
    'All results',
  )
  await expect(page).not.toHaveURL(/[?&]status=/)
})

test('function and range filters show helpful empty and invalid states', async ({ page }) => {
  const transaction = transactions.find((item) => item.calls.length > 0)!
  const call = transaction.calls[0]!
  await page.goto('/transactions')
  await page.getByLabel('Contract', { exact: true }).fill(call.contract_id)
  await page.getByLabel('Function', { exact: true }).fill(call.function_name)
  await page.getByLabel('From ledger', { exact: true }).fill(String(transaction.ledger_sequence))
  await page.getByLabel('To ledger', { exact: true }).fill(String(transaction.ledger_sequence))
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect(page).toHaveURL(/[?&]function=/)
  await expect(page.locator(`tbody a[href="/tx/${transaction.hash}"]`)).toBeVisible()

  await page.getByLabel('Function', { exact: true }).fill('not_present')
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect(page).toHaveURL(/[?&]function=not_present/)
  await expect(page.getByText('No transactions match these filters', { exact: true })).toBeVisible()
  await page
    .getByLabel('From ledger', { exact: true })
    .fill(String(transaction.ledger_sequence + 1))
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('From ledger must be less than or equal')
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
  ).toBeTruthy()
})

test('events and movements can be filtered through their global pages', async ({ page }) => {
  const movement = tokenTransfers[0]!
  await page.goto('/events')
  await page.getByLabel('Contract', { exact: true }).fill(movement.asset.contract_id)
  await page.getByLabel('Event name', { exact: true }).fill('transfer')
  await chooseFilter(page, 'Event type', 'Contract')
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect(page).toHaveURL(/[?&]event_name=transfer/)
  await expect(page.locator('.activity-card').first()).toBeVisible()
  expect(new URL(page.url()).searchParams.get('event_name')).toBe('transfer')

  await page
    .getByRole('navigation', { name: 'Main navigation' })
    .getByRole('link', { name: 'Token movements', exact: true })
    .click()
  await page.getByLabel('Asset contract', { exact: true }).fill(movement.asset.contract_id)
  await page.getByLabel('From address', { exact: true }).fill(movement.from!)
  await page.getByLabel('To address', { exact: true }).fill(movement.to!)
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect(page).toHaveURL(/[?&]asset=/)
  await expect(
    page.getByRole('table', { name: 'Token movements' }).locator('tbody tr'),
  ).toHaveCount(1)
  await page.reload()
  await expect(page.getByLabel('Asset contract', { exact: true })).toHaveValue(
    movement.asset.contract_id,
  )
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
  ).toBeTruthy()
})

test('empty activity text keeps readable insets inside panels', async ({ page }) => {
  const transaction = transactions.find(
    (item) => !tokenTransfers.some((movement) => movement.transaction_hash === item.hash),
  )!
  await page.goto(`/tx/${transaction.hash}`)
  await expect(
    page.getByText('No decoded token movements in this transaction.', { exact: true }),
  ).toBeVisible()

  const insets = await page.locator('.empty-inline').evaluateAll((elements) =>
    elements.map((element) => {
      const panel = element.closest('.panel')!
      const panelBox = panel.getBoundingClientRect()
      const range = document.createRange()
      range.selectNodeContents(element)
      const textBox = range.getBoundingClientRect()
      return {
        text: element.textContent,
        left: textBox.left - panelBox.left,
        right: panelBox.right - textBox.right,
      }
    }),
  )

  expect(insets.length).toBeGreaterThan(0)
  for (const inset of insets) {
    expect(inset.left, inset.text!).toBeGreaterThanOrEqual(15)
    expect(inset.right, inset.text!).toBeGreaterThanOrEqual(15)
  }
})
