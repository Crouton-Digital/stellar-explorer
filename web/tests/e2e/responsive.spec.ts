import { expect, test } from '@playwright/test'
import type { Asset, Page, Transaction } from '../../app/types'

let routes: string[]
test.beforeAll(async ({ request }) => {
  const transactions = (await (
    await request.get('http://127.0.0.1:18080/api/v1/transactions?limit=100')
  ).json()) as Page<Transaction>
  const assets = (await (
    await request.get('http://127.0.0.1:18080/api/v1/assets?limit=1')
  ).json()) as Page<Asset>
  const transaction = transactions.items[0]!
  const contract = transactions.items.find((item) => item.calls.length > 0)!.calls[0]!.contract_id
  routes = [
    '/',
    '/ledgers',
    `/ledgers/${transaction.ledger_sequence}`,
    '/transactions',
    `/tx/${transaction.hash}`,
    `/accounts/${transaction.source_account}`,
    `/contracts/${contract}`,
    '/assets',
    `/assets/${assets.items[0]!.contract_id}`,
    '/nfts',
    '/nfts/transfers',
    '/events',
    '/transfers',
  ]
})

for (const width of [320, 600, 768]) {
  test(`all explorer views fit ${width}px with usable navigation and readable data`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 844 })
    const errors: string[] = []
    page.on('pageerror', (error) => errors.push(error.message))

    for (const path of routes) {
      await page.goto(path)
      await expect(page.locator('h1')).toBeVisible()
      await expect(page.getByRole('status')).toHaveCount(0)
      await expect(page.getByRole('alert')).toHaveCount(0)

      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
        path,
      ).toBeTruthy()
      if (width <= 600) {
        const navigation = page.getByRole('navigation', { name: 'Main navigation' })
        const boxes = await navigation.locator('a').evaluateAll((links) =>
          links.map((link) => {
            const rect = link.getBoundingClientRect()
            return { left: rect.left, right: rect.right, height: rect.height }
          }),
        )
        expect(boxes).toHaveLength(7)
        for (const box of boxes) {
          expect(box.left, path).toBeGreaterThanOrEqual(0)
          expect(box.right, path).toBeLessThanOrEqual(width)
          expect(box.height, path).toBeGreaterThanOrEqual(44)
        }

        const tables = await page.locator('.responsive-table').evaluateAll((items) =>
          items.map((table) => ({
            scrollWidth: table.scrollWidth,
            clientWidth: table.clientWidth,
            cells: [...table.querySelectorAll('tbody td')].map((cell) => ({
              label: cell.getAttribute('data-label'),
              scrollWidth: cell.scrollWidth,
              clientWidth: cell.clientWidth,
            })),
          })),
        )
        for (const table of tables) {
          expect(table.scrollWidth, path).toBeLessThanOrEqual(table.clientWidth + 1)
          for (const cell of table.cells) {
            expect(cell.label, path).toBeTruthy()
            expect(cell.scrollWidth, `${path}: ${cell.label}`).toBeLessThanOrEqual(
              cell.clientWidth + 1,
            )
          }
        }
      }

      if (path === '/' || path === '/transactions') {
        await page.screenshot({
          path: testInfo.outputPath(`${path === '/' ? 'overview' : 'filters'}-${width}.png`),
          fullPage: true,
        })
      }
    }

    expect(errors).toEqual([])
  })
}
