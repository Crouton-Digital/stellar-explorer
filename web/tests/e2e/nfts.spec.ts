import { test, expect, type Page as BrowserPage } from '@playwright/test'
import type { NFTCollection, NFTTransfer, Overview, Page, Transaction } from '../../app/types'

const api = 'http://127.0.0.1:18080/api/v1'
const maximumID = '115792089237316195423570985008687907853269984665640564039457584007913129639935'
const recipient = 'CAPR6HY7D4PR6HY7D4PR6HY7D4PR6HY7D4PR6HY7D4PR6HY7D4PR77LP'
let transaction: Transaction
let failed: Transaction
let collection: NFTCollection
let movements: NFTTransfer[]
let overview: Overview

test.beforeAll(async ({ request }) => {
  const summaries = (await (
    await request.get(`${api}/transactions?limit=100`)
  ).json()) as Page<Transaction>
  const source = summaries.items.find((item) => item.successful && item.calls.length > 0)!
  const failure = summaries.items.find((item) => !item.successful)!
  transaction = (await (
    await request.get(`${api}/transactions/${source.hash}`)
  ).json()) as Transaction
  failed = (await (await request.get(`${api}/transactions/${failure.hash}`)).json()) as Transaction
  overview = (await (await request.get(`${api}/overview`)).json()) as Overview
  collection = {
    network: 'testnet',
    contract_id: transaction.calls[0]!.contract_id,
    verification: 'event_shape',
    first_observed_ledger: transaction.ledger_sequence,
    observed_token_count: 3,
    transfer_count: 3,
  }
  const movement: NFTTransfer = {
    transaction_hash: transaction.hash,
    ledger_sequence: transaction.ledger_sequence,
    tx_index: transaction.tx_index,
    event_index: 100,
    op_index: 0,
    kind: 'transfer',
    contract_id: collection.contract_id,
    token_id: maximumID,
    token_id_type: 'u256',
    from: transaction.source_account,
    to: recipient,
    created_at: transaction.created_at,
    verification: 'event_shape',
    event_standard: 'sep50',
    format: 'scalar',
  }
  movements = [
    movement,
    {
      ...movement,
      event_index: 101,
      kind: 'mint',
      token_id: '4294967295',
      token_id_type: 'u32',
      format: 'token_id_map',
      from: undefined,
    },
    {
      ...movement,
      event_index: 102,
      kind: 'burn',
      token_id: '0',
      token_id_type: 'u32',
      format: 'token_id_map',
      event_standard: 'openzeppelin_burnable',
      to: undefined,
    },
  ]
})

async function mockNFTs(page: BrowserPage) {
  await page.route('**/api/v1/nfts/collections', (route) =>
    route.fulfill({ json: { items: [collection], next_cursor: null } }),
  )
  const range = { ...overview.sync, last_indexed_ledger: transaction.ledger_sequence + 100 }
  await page.route(`**/api/v1/nfts/collections/${collection.contract_id}`, (route) =>
    route.fulfill({
      json: {
        collection,
        transfers: { items: movements, next_cursor: null },
        range,
        snapshot_ledger: transaction.ledger_sequence,
      },
    }),
  )
  await page.route(
    `**/api/v1/nfts/collections/${collection.contract_id}/tokens/${maximumID}`,
    (route) =>
      route.fulfill({
        json: {
          item: {
            network: 'testnet',
            contract_id: collection.contract_id,
            token_id: maximumID,
            first_observed_ledger: transaction.ledger_sequence,
            transfer_count: 1,
            verification: 'event_shape',
          },
          transfers: { items: [movements[0]], next_cursor: null },
          range,
          snapshot_ledger: transaction.ledger_sequence,
        },
      }),
  )
}

test('NFT collection and item pages preserve exact IDs and snapshot scope on mobile', async ({
  page,
}) => {
  await page.goto('/ledgers')
  await mockNFTs(page)
  await page
    .getByRole('navigation', { name: 'Main navigation' })
    .getByRole('link', { name: 'NFTs', exact: true })
    .click()
  await expect(page.getByRole('table', { name: 'Observed NFT collections' })).toContainText(
    'Event format match',
  )
  await page.locator(`a[href="/nfts/${collection.contract_id}"]`).click()
  await expect(
    page.getByRole('heading', { name: 'NFT collection activity', exact: true }),
  ).toBeVisible()
  await expect(
    page.getByText(`through snapshot ledger ${transaction.ledger_sequence}`, { exact: false }),
  ).toBeVisible()
  await expect(
    page.getByText('Movement filters apply only to the history below', { exact: false }),
  ).toBeVisible()
  const table = page.getByRole('table', { name: 'NFT movements', exact: true })
  await expect(table).toContainText(maximumID)
  await expect(table).toContainText('4294967295')
  await expect(table).toContainText('Burnable extension')
  await expect(table.getByRole('link', { name: '0', exact: true })).toBeVisible()
  for (const width of [320, 390, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 })
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBeTruthy()
  }
  await table.getByRole('link', { name: maximumID, exact: true }).click()
  await expect(page).toHaveURL(`/nfts/${collection.contract_id}/${maximumID}`)
  await expect(page.getByRole('button', { name: `Copy ${maximumID}`, exact: true })).toBeVisible()
  await expect(
    page.getByText('Unknown · indexed events do not establish current ownership'),
  ).toBeVisible()
  await expect(page.locator('main img')).toHaveCount(0)
  for (const width of [320, 390, 768]) {
    await page.setViewportSize({ width, height: 900 })
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBeTruthy()
  }
})

test('NFT filtering and pagination retain full decimal IDs and party links use NFT history', async ({
  page,
}) => {
  await page.goto('/ledgers')
  await mockNFTs(page)
  const queries: URL[] = []
  const genericContractRequests: string[] = []
  page.on('request', (request) => {
    if (request.url().includes(`/api/v1/contracts/${recipient}`)) {
      genericContractRequests.push(request.url())
    }
  })
  await page.route('**/api/v1/nfts/transfers*', (route) => {
    const url = new URL(route.request().url())
    queries.push(url)
    const items = movements.filter((movement) =>
      ['kind', 'token_id', 'from', 'to'].every(
        (key) =>
          !url.searchParams.has(key) || movement[key as 'kind'] === url.searchParams.get(key),
      ),
    )
    return route.fulfill({
      json: {
        items: items.slice(0, 1),
        next_cursor: url.searchParams.has('cursor') ? null : 'nft-page-2',
      },
    })
  })
  await page
    .getByRole('navigation', { name: 'Main navigation' })
    .getByRole('link', { name: 'NFTs', exact: true })
    .click()
  await page.getByRole('link', { name: 'NFT movements →', exact: true }).click()
  await page.getByLabel('Collection contract', { exact: true }).fill(collection.contract_id)
  await page.getByLabel('Token ID', { exact: true }).fill(maximumID)
  await page.getByRole('combobox', { name: 'NFT action', exact: true }).click()
  await page.getByRole('option', { name: 'Transfer', exact: true }).click()
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click()
  await expect.poll(() => queries.at(-1)?.searchParams.get('token_id')).toBe(maximumID)
  expect(queries.at(-1)!.searchParams.get('kind')).toBe('transfer')
  await page.getByRole('link', { name: 'Older results →', exact: true }).click()
  await expect(page).toHaveURL(/cursor=nft-page-2/)
  await expect(page.getByLabel('Token ID', { exact: true })).toHaveValue(maximumID)
  expect(new URL(page.url()).searchParams.get('collection')).toBe(collection.contract_id)
  await page.locator(`a[href="/nfts/transfers?to=${recipient}"]`).click()
  await expect(page).toHaveURL(`/nfts/transfers?to=${recipient}`)
  await expect(page.getByLabel('To address', { exact: true })).toHaveValue(recipient)
  expect(genericContractRequests).toEqual([])
})

test('transaction NFT evidence stays separate from fungible movements and failed calls', async ({
  page,
}) => {
  await page.goto('/ledgers')
  await page.route(`**/api/v1/transactions/${transaction.hash}`, (route) =>
    route.fulfill({ json: { ...transaction, nft_transfers: movements } }),
  )
  await page.getByRole('button', { name: 'Open search', exact: true }).click()
  await page
    .getByLabel('Search transactions, accounts, contracts or ledgers')
    .fill(transaction.hash)
  await page.getByRole('button', { name: 'Search ↵' }).click()
  const nftTable = page.getByRole('table', { name: 'NFT movements', exact: true })
  await expect(nftTable.locator('tbody tr')).toHaveCount(3)
  await expect(nftTable).toContainText(maximumID)
  await expect(nftTable.getByRole('columnheader', { name: 'Amount', exact: true })).toHaveCount(0)
  await expect(page.getByRole('heading', { name: /^Token movements/ })).toBeVisible()
  await page.route(`**/api/v1/transactions/${failed.hash}`, (route) =>
    route.fulfill({ json: { ...failed, nft_transfers: [] } }),
  )
  await page.getByRole('button', { name: 'Open search', exact: true }).click()
  await page.getByLabel('Search transactions, accounts, contracts or ledgers').fill(failed.hash)
  await page.getByRole('button', { name: 'Search ↵' }).click()
  await expect(
    page.getByText('No decoded NFT movements in this transaction.', { exact: true }),
  ).toBeVisible()
  await expect(page.getByRole('table', { name: 'NFT movements', exact: true })).toHaveCount(0)
})

test('NFT collection navigation handles loading, empty and API failures', async ({ page }) => {
  await page.goto('/ledgers')
  let release: (() => void) | undefined
  const hold = new Promise<void>((resolve) => {
    release = resolve
  })
  await page.route('**/api/v1/nfts/collections', async (route) => {
    await hold
    await route.fulfill({ json: { items: [], next_cursor: null } })
  })
  const navigation = page.getByRole('navigation', { name: 'Main navigation' })
  await navigation.getByRole('link', { name: 'NFTs', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('Loading indexed data')
  release!()
  await expect(page.getByText('No NFT collections observed yet', { exact: true })).toBeVisible()
  await navigation.getByRole('link', { name: 'Ledgers', exact: true }).click()
  await page.route('**/api/v1/nfts/collections', (route) =>
    route.fulfill({
      status: 503,
      json: { error: { code: 'storage_unavailable', message: 'NFT index unavailable' } },
    }),
  )
  await navigation.getByRole('link', { name: 'NFTs', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('NFT index unavailable')
})
