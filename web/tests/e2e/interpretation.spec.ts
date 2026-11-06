import { test, expect } from '@playwright/test'
import type { Page as ApiPage, Transaction } from '../../app/types'

const api = 'http://127.0.0.1:18080/api/v1'
let transactions: Transaction[]
let payment: Transaction
let failed: Transaction
let contract: Transaction

test.beforeAll(async ({ request }) => {
  const response = await request.get(`${api}/transactions?limit=100`)
  expect(response.ok()).toBeTruthy()
  transactions = ((await response.json()) as ApiPage<Transaction>).items
  const details = await Promise.all(
    transactions.map(async (transaction) => {
      const response = await request.get(`${api}/transactions/${transaction.hash}`)
      expect(response.ok()).toBeTruthy()
      return (await response.json()) as Transaction
    }),
  )
  payment = details.find(
    (item) => item.successful && item.interpretation?.actions.some((a) => a.kind === 'payment'),
  )!
  failed = details.find((item) => !item.successful)!
  contract = details.find((item) => item.successful && item.calls.length > 0)!
  expect(payment).toBeTruthy()
  expect(failed).toBeTruthy()
  expect(contract).toBeTruthy()
})

test('real payment explains amount and participants without duplicate actions', async ({
  page,
}) => {
  await page.goto(`/tx/${payment.hash}`)
  const panel = page.getByRole('region', { name: 'Transaction actions', exact: true })
  await expect(panel.getByRole('heading', { name: 'What happened', exact: true })).toBeVisible()
  const actions = payment.interpretation!.actions.filter((action) => action.kind === 'payment')
  await expect(panel.getByRole('article', { name: 'Payment', exact: true })).toHaveCount(
    actions.length,
  )
  const action = panel.getByRole('article', { name: 'Payment', exact: true }).first()
  await expect(action.getByText('Applied', { exact: true })).toBeVisible()
  await expect(action.locator('a[href^="/accounts/"]').first()).toBeVisible()
  await expect(action.locator(`a[href="#operation-${actions[0]!.op_index}"]`)).toBeVisible()
})

test('real failed transaction explains failure without claiming applied effects', async ({
  page,
}) => {
  await page.goto(`/tx/${failed.hash}`)
  const panel = page.getByRole('region', { name: 'Transaction actions', exact: true })
  await expect(panel.getByRole('heading', { name: 'Why it failed', exact: true })).toBeVisible()
  await expect(panel).toContainText(failed.interpretation!.failure!.message)
  await expect(panel.getByText('Applied', { exact: true })).toHaveCount(0)
  expect(failed.interpretation!.actions.every((action) => action.status !== 'applied')).toBeTruthy()
  await page.getByText('Failure codes and operation results', { exact: true }).click()
  await expect(panel.locator('details[open]')).toContainText(failed.result_code)
})

test('handled diagnostic remains evidence on a successful transaction', async ({ page }) => {
  await page.goto('/transactions')
  await page.route(`**/api/v1/transactions/${contract.hash}`, (route) =>
    route.fulfill({
      json: {
        ...contract,
        interpretation: {
          ...contract.interpretation,
          diagnostic_errors: [
            {
              event_index: 0,
              type: 'ScErrorTypeSceContract',
              code: '7',
              message: 'Contract-defined error 7. Its meaning depends on the contract.',
            },
          ],
        },
      },
    }),
  )
  await page.locator(`a[href="/tx/${contract.hash}"]`).first().click()
  const panel = page.getByRole('region', { name: 'Transaction actions', exact: true })
  await expect(panel.getByRole('heading', { name: 'Why it failed' })).toHaveCount(0)
  await panel.getByText('Diagnostic evidence (1)', { exact: true }).click()
  await expect(panel).toContainText('Its meaning depends on the contract')
  await expect(panel).toContainText('These traces can include handled errors')
})

test('unknown token decimals retain exact base units on narrow screens', async ({ page }) => {
  await page.goto('/transactions')
  const existing = payment.interpretation!.actions.find((action) => action.asset)!
  const amount = '170141183460469231731687303715884105727'
  await page.route(`**/api/v1/transactions/${contract.hash}`, (route) =>
    route.fulfill({
      json: {
        ...contract,
        interpretation: {
          ...contract.interpretation,
          actions: [
            {
              ...existing,
              id: 'event:0',
              kind: 'transfer',
              title: 'Token transfer',
              description: 'Move tokens from the sender to the recipient.',
              source: 'token_event',
              amount,
              asset: { ...existing.asset, kind: 'contract', decimals: null, code: null },
            },
          ],
        },
      },
    }),
  )
  await page.locator(`a[href="/tx/${contract.hash}"]`).first().click()
  const panel = page.getByRole('region', { name: 'Transaction actions', exact: true })
  for (const width of [320, 390, 768]) {
    await page.setViewportSize({ width, height: 900 })
    await expect(panel).toContainText(amount)
    await expect(panel).toContainText('base units')
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBeTruthy()
  }
})
