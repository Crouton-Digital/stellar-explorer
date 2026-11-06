import { test, expect } from '@playwright/test'
import type { Page as ApiPage, Transaction, StateChange, PathPayment } from '../../app/types'

const api = 'http://127.0.0.1:18080/api/v1'
let transaction: Transaction
let listed: Transaction

test.beforeAll(async ({ request }) => {
  const response = await request.get(`${api}/transactions?limit=100`)
  expect(response.ok()).toBeTruthy()
  const summaries = ((await response.json()) as ApiPage<Transaction>).items
  const details = await Promise.all(
    summaries.map(async (item) => {
      const response = await request.get(`${api}/transactions/${item.hash}`)
      expect(response.ok()).toBeTruthy()
      return (await response.json()) as Transaction
    }),
  )
  transaction = details.find((item) => item.interpretation?.state_changes?.length)!
  listed = details.find(
    (item) => item.successful && item.interpretation?.actions.some((action) => action.asset),
  )!
  expect(transaction).toBeTruthy()
  expect(listed).toBeTruthy()
})

test('real metadata shows committed changes and raw key evidence', async ({ page }) => {
  await page.goto(`/tx/${transaction.hash}`)
  const panel = page.getByRole('region', { name: 'State changes', exact: true })
  const changes = transaction.interpretation!.state_changes!
  await expect(panel.getByRole('article')).toHaveCount(Math.min(10, changes.length))
  const first = panel.getByRole('article').first()
  await expect(first).toContainText(changes[0]!.kind)
  await expect(first).toContainText(`Metadata change #${changes[0]!.change_index}`)
  await first.getByText('Ledger key XDR', { exact: true }).click()
  await expect(first).toContainText(changes[0]!.key_xdr)
  await expect(panel).toContainText('they do not describe current balances')
})

test('unknown and archived state remain explicit with accessible pagination', async ({ page }) => {
  await page.goto('/transactions')
  const baseline: StateChange = {
    id: 'change:0',
    kind: 'updated',
    entry_type: 'Account',
    scope: 'operation',
    op_index: 0,
    change_index: 1,
    before_index: null,
    subject: listed.source_account,
    key: null,
    key_xdr: 'AAAAAA==',
    before_status: 'unknown',
    after_status: 'present',
    before: null,
    after: {
      last_modified_ledger: listed.ledger_sequence,
      projection: 'supported_fields',
      fields: {
        balance_stroops: '9223372036854775807',
        home_domain: { encoding: 'hex', value_hex: '6100ff' },
      },
    },
  }
  const changes = Array.from({ length: 11 }, (_, index) => ({
    ...baseline,
    id: `change:${index}`,
    ...(index === 10 ? { kind: 'restored' as const, before_status: 'archived' as const } : {}),
  }))
  await page.route(`**/api/v1/transactions/${listed.hash}`, (route) =>
    route.fulfill({
      json: { ...listed, interpretation: { ...listed.interpretation, state_changes: changes } },
    }),
  )
  await page.locator(`a[href="/tx/${listed.hash}"]`).first().click()
  const panel = page.getByRole('region', { name: 'State changes', exact: true })
  await expect(panel.getByRole('article')).toHaveCount(10)
  await expect(
    panel.getByText('A matching before value was not provided', { exact: false }),
  ).toHaveCount(10)
  await expect(panel).toContainText('9223372036854775807')
  await expect(panel.getByRole('article').first()).toContainText('6100ff')
  await expect(panel.getByRole('button', { name: 'Previous', exact: true })).toBeDisabled()
  await panel.getByRole('button', { name: 'Next', exact: true }).click()
  await expect(panel.getByRole('article')).toHaveCount(1)
  await expect(panel).toContainText('Archived')
  await expect(panel).toContainText('Page 2 of 2')
  await expect(panel.getByRole('button', { name: 'Next', exact: true })).toBeDisabled()
  await panel.getByRole('button', { name: 'Previous', exact: true }).click()
  await expect(panel).toContainText('Page 1 of 2')
  for (const width of [320, 390, 768]) {
    await page.setViewportSize({ width, height: 900 })
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBeTruthy()
  }
})

function payment(executed: boolean): PathPayment {
  const asset = listed.interpretation!.actions.find((action) => action.asset)!.asset!
  const output = { ...asset, code: 'USD', classic_asset: null }
  const inputLeg = { asset, amount: '20000000' }
  const outputLeg = { asset: output, amount: '10000000' }
  return {
    mode: 'strict_receive',
    send_asset: asset,
    destination_asset: output,
    send_limit: executed ? '30000000' : '-30000000',
    destination_limit: '10000000',
    path: [],
    execution: executed
      ? {
          protocol: 'stellar_dex',
          input: inputLeg,
          output: outputLeg,
          trades: [{ claim_index: 0, venue: 'liquidity_pool', input: inputLeg, output: outputLeg }],
        }
      : null,
  }
}

for (const executed of [true, false]) {
  test(`path payment ${executed ? 'shows actual swap amounts separately from limits' : 'keeps uncommitted limits separate from execution'}`, async ({
    page,
  }) => {
    await page.goto('/transactions')
    const original = listed.interpretation!.actions[0]!
    await page.route(`**/api/v1/transactions/${listed.hash}`, (route) =>
      route.fulfill({
        json: {
          ...listed,
          interpretation: {
            ...listed.interpretation,
            actions: [
              {
                ...original,
                kind: executed ? 'swap' : 'path_payment',
                title: executed ? 'Swap assets' : 'Path payment',
                status: executed ? 'applied' : 'not_applied',
                amount: null,
                asset: null,
                path_payment: payment(executed),
              },
            ],
          },
        },
      }),
    )
    await page.locator(`a[href="/tx/${listed.hash}"]`).first().click()
    const panel = page.getByRole('region', { name: 'Transaction actions', exact: true })
    await expect(panel).toContainText('Maximum send')
    if (executed) {
      await expect(panel.getByText('Amount sent', { exact: true })).toBeVisible()
      await expect(panel.getByText('Amount received', { exact: true })).toBeVisible()
      await panel.getByText('Executed trades (1)', { exact: true }).click()
      await expect(panel).toContainText('Liquidity pool · Result claim #0')
    } else {
      await expect(panel.getByText('Amount sent', { exact: true })).toHaveCount(0)
      await expect(panel).toContainText('These amounts are request limits')
      await expect(panel).toContainText('-3.0000000')
    }
    for (const width of [320, 390, 768]) {
      await page.setViewportSize({ width, height: 900 })
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
      ).toBeTruthy()
      if (width > 600) {
        const gap = await panel.getByText('Maximum send', { exact: true }).evaluate((label) => {
          const value = label.nextElementSibling!
          return value.getBoundingClientRect().left - label.getBoundingClientRect().right
        })
        expect(gap).toBeGreaterThanOrEqual(12)
      }
    }
  })
}
