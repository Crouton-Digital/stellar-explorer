import { test, expect } from '@playwright/test'
import type { Page as ApiPage, Transaction } from '../../app/types'

const api = 'http://127.0.0.1:18080/api/v1'
let transactions: Transaction[]

test.beforeAll(async ({ request }) => {
  const response = await request.get(`${api}/transactions?limit=100`)
  expect(response.ok()).toBeTruthy()
  transactions = ((await response.json()) as ApiPage<Transaction>).items
})

test('real successful and failed transactions expose reconciled fees on every viewport', async ({
  page,
  request,
}, testInfo) => {
  for (const successful of [true, false]) {
    const transaction = transactions.find((item) => item.successful === successful)!
    expect(transaction).toBeTruthy()
    expect(transaction.fees.status).toBe('complete')
    expect(transaction.fees.net).toBe(transaction.fee_charged)

    await page.goto(`/tx/${transaction.hash}`)
    const panel = page.getByRole('region', { name: 'Fees and refunds', exact: true })
    await expect(panel.getByRole('heading', { name: 'Fees and refunds' })).toBeVisible()

    for (const [index, amount] of [
      transaction.fees.charged,
      transaction.fees.refunded,
      transaction.fees.net,
    ].entries()) {
      await expect(
        panel.locator('dl > div').nth(index).getByText(`${amount} stroops`, { exact: true }),
      ).toBeVisible()
    }

    const response = await request.get(`${api}/transactions/${transaction.hash}`)
    const detail = (await response.json()) as Transaction
    expect(detail.events.some((event) => event.event_scope === 'transaction')).toBeTruthy()
    if (!successful) {
      expect(detail.token_transfers).toEqual([])
      expect(detail.events.every((event) => event.op_index === null)).toBeTruthy()
    }

    for (const width of [320, 768]) {
      await page.setViewportSize({ width, height: 850 })
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
      ).toBeTruthy()
      await expect(panel.getByText('Net fee', { exact: true })).toBeVisible()
    }
  }

  await page.screenshot({ path: testInfo.outputPath('fees-mobile.png'), fullPage: true })
})

test('event source filter separates transaction fees from operation events', async ({ page }) => {
  await page.goto('/events')
  const source = page.getByRole('combobox', { name: 'Event source', exact: true })
  await source.click()
  await page.getByRole('option', { name: 'Transaction events', exact: true }).click()
  await page.getByLabel('Event name', { exact: true }).fill('fee')
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect(page).toHaveURL(/event_scope=transaction/)
  await expect(page.getByText('Before all transactions', { exact: true }).first()).toBeVisible()
  await expect(page.locator('.activity-card').first()).toContainText('/ transaction')

  await page.reload()
  await expect(source).toHaveText('Transaction events')
  await source.click()
  await page.getByRole('option', { name: 'Operation events', exact: true }).click()
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect(page).toHaveURL(/event_scope=operation/)
  await expect(page.getByText('No events match this view', { exact: true })).toBeVisible()
})

test('missing and inconsistent fee data stay unknown instead of displaying zero', async ({
  page,
  request,
}) => {
  const transaction = transactions[0]!
  const response = await request.get(`${api}/transactions/${transaction.hash}`)
  const detail = (await response.json()) as Transaction
  await page.goto('/transactions')

  for (const status of ['unavailable', 'invalid'] as const) {
    await page.route(`**/api/v1/transactions/${transaction.hash}`, (route) =>
      route.fulfill({
        json: {
          ...detail,
          fees: {
            ...detail.fees,
            status,
            charged: null,
            refunded: null,
            net: null,
            reason:
              status === 'invalid' ? 'fee_events_do_not_match_result' : 'fee_events_not_provided',
          },
        },
      }),
    )
    await page.locator(`a[href="/tx/${transaction.hash}"]`).first().click()
    const panel = page.getByRole('region', { name: 'Fees and refunds', exact: true })
    await expect(panel).toContainText(
      status === 'invalid' ? 'could not be reconciled' : 'initial charge and refund are unknown',
    )
    await expect(panel.locator('dl')).toHaveCount(0)
    await page
      .getByRole('navigation')
      .getByRole('link', { name: 'Transactions', exact: true })
      .click()
    await expect(page).toHaveURL('/transactions')
  }
})
