import { test, expect } from '@playwright/test'
import type { Page as ApiPage, Action, ContractSwap, Transaction } from '../../app/types'

const api = 'http://127.0.0.1:18080/api/v1'
const router = 'CCJUD55AG6W5HAI5LRVNKAE5WDP5XGZBUDS5WNTIVDU7O264UZZE7BRD'
const trader = 'CDP3HMUH6SMS3S7NPGNDJLULCOXXEPSHY4JKUKMBNQMATHDHWXRRJTBY'
const maximum = '170141183460469231731687303715884105727'
let transaction: Transaction
let payment: Transaction

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
  transaction = details.find((item) => item.successful && item.calls.length > 0)!
  payment = details.find(
    (item) => item.successful && item.interpretation?.actions.some((action) => action.asset),
  )!
  expect(transaction).toBeTruthy()
  expect(payment).toBeTruthy()
})

function swap(mode: ContractSwap['mode']): ContractSwap {
  const asset = payment.interpretation!.actions.find((action) => action.asset)!.asset!
  const inputAsset = { ...asset, decimals: null, code: null, classic_asset: null }
  const intermediateAsset = { ...asset, contract_id: router, code: 'MID', decimals: 7 }
  const outputAsset = {
    ...asset,
    contract_id: 'CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC',
    code: 'XLM',
    decimals: 7,
  }
  const input = {
    asset: inputAsset,
    amount: mode === 'exact_input' ? maximum : '170141183460469231731687303715884105726',
  }
  const intermediate = { asset: intermediateAsset, amount: '30000000' }
  const output = { asset: outputAsset, amount: '10000000' }

  return {
    protocol: 'soroswap',
    adapter_version: 1,
    verification: 'registered_router_and_transfers',
    router_contract: router,
    router_wasm_hash: null,
    mode,
    trader,
    input_limit: maximum,
    output_limit: mode === 'exact_input' ? '9999999' : output.amount,
    deadline: '18446744073709551615',
    path: [inputAsset.contract_id, intermediateAsset.contract_id, outputAsset.contract_id],
    amounts: [input.amount, intermediate.amount, output.amount],
    input,
    output,
    hops: [
      {
        index: 0,
        pool_contract: transaction.calls[0]!.contract_id,
        input,
        output: intermediate,
        input_event_index: 0,
        output_event_index: 1,
      },
      {
        index: 1,
        pool_contract: trader,
        input: intermediate,
        output,
        input_event_index: 1,
        output_event_index: 2,
      },
    ],
    router_event_index: 5,
    transfer_event_indexes: [0, 1, 2],
  }
}

function interpretedAction(evidence?: ContractSwap): Action {
  return {
    id: 'operation:0',
    kind: evidence ? 'swap' : 'contract_call',
    title: evidence ? 'Swap assets' : 'Call swap',
    description: evidence
      ? 'Exchange tokens through the registered Soroswap router.'
      : 'Invoke a contract function. No supported swap evidence was established.',
    status: 'applied',
    source: 'operation',
    op_index: 0,
    event_indexes: evidence ? [0, 1, 2, 5] : [],
    from: evidence ? trader : transaction.source_account,
    contract_id: evidence ? router : transaction.calls[0]!.contract_id,
    function: evidence ? 'swap_exact_tokens_for_tokens' : 'swap',
    amount: null,
    asset: null,
    result_code: null,
    ...(evidence
      ? { contract_swap: evidence, source_account: transaction.source_account, to: trader }
      : {}),
  }
}

for (const mode of ['exact_input', 'exact_output'] as const) {
  test(`Soroswap ${mode} separates trader, actual amounts and request limits`, async ({ page }) => {
    await page.goto('/transactions')
    const evidence = swap(mode)
    const tokenAction: Action = {
      ...interpretedAction(),
      id: 'event:0',
      kind: 'transfer',
      title: 'Token transfer',
      description: 'The input transfer used by the interpreted swap.',
      source: 'token_event',
      from: trader,
      event_indexes: [0],
      amount: evidence.input.amount,
      asset: evidence.input.asset,
    }
    await page.route(`**/api/v1/transactions/${transaction.hash}`, (route) =>
      route.fulfill({
        json: {
          ...transaction,
          interpretation: {
            ...transaction.interpretation,
            version: 3,
            actions: [interpretedAction(evidence), tokenAction],
          },
        },
      }),
    )
    await page.locator(`a[href="/tx/${transaction.hash}"]`).first().click()
    const action = page.getByRole('article', { name: 'Swap assets', exact: true })
    const panel = action.getByRole('region', { name: 'Soroswap execution', exact: true })
    await expect(panel.getByText('Amount sent', { exact: true })).toBeVisible()
    await expect(panel.getByText('Amount received', { exact: true })).toBeVisible()
    await expect(panel).toContainText(evidence.input.amount)
    await expect(panel).toContainText('base units')
    await expect(panel).toContainText('1.0000000 XLM')
    await expect(panel).toContainText('18446744073709551615')
    const inputLabel = mode === 'exact_input' ? 'Requested input' : 'Maximum input'
    await expect(panel.getByText(inputLabel, { exact: true })).toBeVisible()
    await expect(
      panel.getByText(mode === 'exact_input' ? 'Minimum output' : 'Requested output', {
        exact: true,
      }),
    ).toBeVisible()
    await expect(panel.locator('dt', { hasText: /^Trader$/ }).locator('+ dd a')).toHaveAttribute(
      'href',
      `/contracts/${trader}`,
    )
    await expect(
      action.locator('dt', { hasText: /^Operation source$/ }).locator('+ dd a'),
    ).toHaveAttribute('href', `/accounts/${transaction.source_account}`)
    await expect(page.getByRole('article', { name: 'Token transfer', exact: true })).toBeVisible()
    await panel.getByText('Swap route (2 pools)', { exact: true }).click()
    await expect(panel).toContainText('Committed input event #0 · Output event #1')
    await expect(panel).toContainText('Committed input event #1 · Output event #2')
    await panel.getByText('Router and evidence', { exact: true }).click()
    await expect(panel).toContainText('The router WASM hash was not observed')
    await expect(panel).toContainText('do not establish historical code identity')
    await expect(panel.getByText('Observed WASM hash', { exact: true })).toHaveCount(0)
    await expect(panel.locator('dt', { hasText: /^Router event$/ }).locator('+ dd')).toHaveText(
      '#5',
    )
    await expect(panel.locator('dt', { hasText: /^Transfer events$/ }).locator('+ dd')).toHaveText(
      '#0, #1, #2',
    )
    for (const width of [320, 390, 768, 1440]) {
      await page.setViewportSize({ width, height: 900 })
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
      ).toBeTruthy()
      if (width > 600) {
        const gap = await panel.getByText(inputLabel, { exact: true }).evaluate((label) => {
          const value = label.nextElementSibling!
          return value.getBoundingClientRect().left - label.getBoundingClientRect().right
        })
        expect(gap).toBeGreaterThanOrEqual(12)
      }
    }
    await panel.getByText('Swap assets, route and source evidence', { exact: true }).click()
    await expect(panel.locator('pre')).toContainText('"registered_router_and_transfers"')
    await expect(panel.locator('pre')).toContainText('"router_wasm_hash": null')
  })
}

test('observed router hash remains distinct from the evidence verification mode', async ({
  page,
}) => {
  await page.goto('/transactions')
  const evidence = {
    ...swap('exact_input'),
    router_wasm_hash: '4b95bbf9caec2c6e00c786f53c5f392c2fcdb8435ac0a862ab5e0645eb65824c',
  }
  await page.route(`**/api/v1/transactions/${transaction.hash}`, (route) =>
    route.fulfill({
      json: {
        ...transaction,
        interpretation: {
          ...transaction.interpretation,
          version: 3,
          actions: [interpretedAction(evidence)],
        },
      },
    }),
  )
  await page.locator(`a[href="/tx/${transaction.hash}"]`).first().click()
  const panel = page.getByRole('region', { name: 'Soroswap execution', exact: true })
  await panel.getByText('Router and evidence', { exact: true }).click()
  await expect(panel).toContainText(evidence.router_wasm_hash)
  await expect(panel).toContainText(
    'This does not certify arbitrary contracts or their current code',
  )
  await expect(
    panel.getByText('The router WASM hash was not observed', { exact: false }),
  ).toHaveCount(0)
})

test('an unsupported function named swap keeps its ordinary contract-call display', async ({
  page,
}) => {
  await page.goto('/transactions')
  await page.route(`**/api/v1/transactions/${transaction.hash}`, (route) =>
    route.fulfill({
      json: {
        ...transaction,
        interpretation: {
          ...transaction.interpretation,
          actions: [interpretedAction()],
        },
      },
    }),
  )
  await page.locator(`a[href="/tx/${transaction.hash}"]`).first().click()
  const action = page.getByRole('article', { name: 'Call swap', exact: true })
  await expect(action).toBeVisible()
  await expect(action).toContainText('No supported swap evidence was established')
  await expect(page.getByRole('region', { name: 'Soroswap execution', exact: true })).toHaveCount(0)
})
