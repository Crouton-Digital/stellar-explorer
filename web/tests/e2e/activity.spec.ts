import { test, expect } from '@playwright/test'
import type { Activity, AddressDetail, Page, Transaction } from '../../app/types'

const api = 'http://127.0.0.1:18080/api/v1'

function activity(): Activity {
  return {
    network: 'testnet',
    kind: 'transactions',
    window_limit: 120,
    first_ledger: 100,
    last_ledger: 219,
    ledger_count: 120,
    from: '2026-11-05T00:00:00Z',
    to: '2026-11-05T00:09:55Z',
    total: 30,
    buckets: Array.from({ length: 12 }, (_, index) => ({
      first_ledger: 100 + index * 10,
      last_ledger: 109 + index * 10,
      closed_at: new Date(Date.UTC(2026, 10, 5, 0, 0, (index * 10 + 9) * 5)).toISOString(),
      count: index === 0 ? 10 : index === 11 ? 20 : 0,
    })),
    breakdown: [
      { key: 'success', count: 20 },
      { key: 'failed', count: 10 },
    ],
  }
}

test('overview statistics agree with the bounded indexed activity API', async ({
  page,
  request,
}) => {
  const response = await request.get(`${api}/activity/transactions`)
  expect(response.ok()).toBeTruthy()
  const data = (await response.json()) as Activity
  await page.goto('/')
  const panel = page.getByRole('region', { name: 'Recent transaction activity', exact: true })
  await expect(panel.locator('.activity-total')).toHaveText(
    `${data.total.toLocaleString('en-US')} matching`,
  )
  await expect(panel).toContainText(`${data.first_ledger}–${data.last_ledger}`)
  await expect(panel).toContainText('not complete network history')
  await expect(panel.locator('.activity-bar')).toHaveCount(data.buckets.length)
})

test('activity charts expose zeros, readable scale and keyboard/touch inspection', async ({
  page,
}) => {
  await page.goto('/ledgers')
  await page.route('**/api/v1/activity/transactions*', (route) =>
    route.fulfill({ json: activity() }),
  )
  await page
    .getByRole('navigation', { name: 'Main navigation' })
    .getByRole('link', { name: 'Transactions', exact: true })
    .click()
  const panel = page.getByRole('region', { name: 'Transaction activity', exact: true })
  await expect(panel.locator('.activity-scale')).toContainText('20')
  await expect(panel).toContainText('66.7%')
  const chart = panel.getByRole('img', { name: 'Transaction activity by indexed ledger bucket' })
  await chart.focus()
  await chart.press('Home')
  await expect(panel.locator('.activity-selection')).toContainText('10 matching · Ledgers 100–109')
  await chart.press('ArrowRight')
  await expect(panel.locator('.activity-selection')).toContainText('0 matching · Ledgers 110–119')
  await expect(panel.locator('.activity-bar.selected')).toHaveAttribute('height', '0')
  await panel.getByRole('button', { name: 'Next activity bucket', exact: true }).click()
  await expect(panel.locator('.activity-selection')).toContainText('Ledgers 120–129')
  await panel.getByText('Activity data (12 buckets)', { exact: true }).click()
  await expect(panel.locator('.activity-data dt')).toHaveCount(12)
  for (const width of [320, 390, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 })

    const geometry = await chart.evaluate((element) => {
      const plot = element.getBoundingClientRect()
      const bars = Array.from(element.querySelectorAll('.activity-bar')).map((bar) =>
        bar.getBoundingClientRect(),
      )

      return {
        peakHeight: Math.max(...bars.map((bar) => bar.height)) / plot.height,
        firstBar: (bars[0]!.left - plot.left) / plot.width,
        lastBar: (bars.at(-1)!.right - plot.left) / plot.width,
      }
    })

    expect(geometry.peakHeight).toBeGreaterThan(0.8)
    expect(geometry.firstBar).toBeLessThan(0.05)
    expect(geometry.lastBar).toBeGreaterThan(0.95)

    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBeTruthy()
    const buttons = await panel.locator('.activity-selection button').evaluateAll((items) =>
      items.map((item) => ({
        width: item.getBoundingClientRect().width,
        height: item.getBoundingClientRect().height,
      })),
    )
    expect(buttons.every((button) => button.width >= 44 && button.height >= 44)).toBeTruthy()
  }
})

test('filtered activity keeps list cursor bindings and omits page-size parameters', async ({
  page,
}) => {
  await page.goto('/transactions?limit=1')
  const queries: URL[] = []
  await page.route('**/api/v1/activity/transactions*', (route) => {
    queries.push(new URL(route.request().url()))
    return route.fulfill({ json: activity() })
  })
  await page.getByRole('link', { name: 'Older results →', exact: true }).click()
  await expect.poll(() => queries.length).toBeGreaterThan(0)
  const cursor = new URL(page.url()).searchParams.get('cursor')
  expect(queries.at(-1)!.searchParams.get('cursor')).toBe(cursor)
  expect(queries.at(-1)!.searchParams.has('limit')).toBeFalsy()
  await page.getByRole('combobox', { name: 'Status', exact: true }).click()
  await page.getByRole('option', { name: 'Failed', exact: true }).click()
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect.poll(() => queries.at(-1)!.searchParams.get('status')).toBe('failed')
  expect(queries.at(-1)!.searchParams.has('cursor')).toBeFalsy()
  expect(queries.at(-1)!.searchParams.has('limit')).toBeFalsy()
})

test('statistics distinguish an empty indexed window from zero matching records', async ({
  page,
}) => {
  await page.goto('/transactions?min_ledger=4294967295')
  const panel = page.getByRole('region', { name: 'Transaction activity', exact: true })
  await expect(panel).toContainText('No indexed ledgers are available within this view')
  await expect(panel.getByRole('img')).toHaveCount(0)
  await page.getByLabel('From ledger', { exact: true }).fill('')
  await page.getByLabel('Function', { exact: true }).fill('not_present')
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect(panel).toContainText('No matching transactions in this indexed window')
  await expect(panel.getByRole('img')).toHaveCount(1)
  await expect(panel).not.toContainText('Successful')
})

for (const kind of ['accounts', 'contracts'] as const) {
  test(`${kind} show a filtered line chart ending at their visible activity`, async ({
    page,
    request,
  }) => {
    const transactionResponse = await request.get(`${api}/transactions?limit=100`)
    const transactions = ((await transactionResponse.json()) as Page<Transaction>).items
    const address =
      kind === 'accounts'
        ? transactions[0]!.source_account
        : transactions.find((transaction) => transaction.calls.length)!.calls[0]!.contract_id
    const response = await request.get(`${api}/${kind}/${address}`)
    const detail = (await response.json()) as AddressDetail
    const latest = detail.transactions.items[0]!.ledger_sequence
    const filter = kind === 'accounts' ? 'address' : 'contract'
    const statistics = await request.get(
      `${api}/activity/transactions?${filter}=${address}&max_ledger=${latest}`,
    )
    const expected = (await statistics.json()) as Activity
    await page.goto(`/${kind}/${address}`)
    const panel = page.getByRole('region', {
      name: kind === 'accounts' ? 'Account transaction activity' : 'Contract transaction activity',
      exact: true,
    })
    await expect(panel.locator('.activity-total')).toHaveText(
      `${expected.total.toLocaleString('en-US')} matching`,
    )
    await expect(panel).toContainText('window ends at the newest transaction on this page')
    await expect(panel.locator('.activity-trend')).toHaveCount(1)
    await expect(panel.locator('.activity-bar')).toHaveCount(0)
    await expect(panel.locator('.activity-window').first()).toContainText(String(latest))
  })
}
