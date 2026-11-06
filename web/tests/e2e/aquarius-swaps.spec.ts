import { test, expect } from '@playwright/test'
import type { Page as ApiPage, Action, AquariusSwap, Asset, Transaction } from '../../app/types'

const api = 'http://127.0.0.1:18080/api/v1'
const router = 'CBCFTQSPDBAIZ6R6PJQKSQWKNKWH2QIV3I4J72SHWBIK3ADRRAM5A6GD'
const trader = 'CDP3HMUH6SMS3S7NPGNDJLULCOXXEPSHY4JKUKMBNQMATHDHWXRRJTBY'
const intermediateToken = 'CCJUD55AG6W5HAI5LRVNKAE5WDP5XGZBUDS5WNTIVDU7O264UZZE7BRD'
const nativeToken = 'CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC'
const maximum = '170141183460469231731687303715884105727'
let transaction: Transaction
let asset: Asset

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
  asset = details
    .find((item) => item.successful && item.interpretation?.actions.some((action) => action.asset))!
    .interpretation!.actions.find((action) => action.asset)!.asset!
  expect(transaction).toBeTruthy()
  expect(asset).toBeTruthy()
})

function swap(routing: 'direct' | 'chained'): AquariusSwap {
  const inputAsset: Asset = {
    ...asset,
    contract_id: trader,
    kind: 'contract',
    verification: 'event_shape',
    decimals: null,
    classic_asset: null,
    code: null,
    issuer: null,
  }
  const intermediateAsset = {
    ...inputAsset,
    contract_id: intermediateToken,
    code: 'MID',
    decimals: 7,
  }
  const outputAsset = {
    ...asset,
    contract_id: nativeToken,
    kind: 'sac' as const,
    verification: 'contract_id' as const,
    classic_asset: 'native',
    code: 'XLM',
    issuer: null,
    decimals: 7,
  }
  const input = { asset: inputAsset, amount: maximum }
  const intermediate = { asset: intermediateAsset, amount: '30000000' }
  const output = { asset: outputAsset, amount: '10000000' }
  const chained = routing === 'chained'

  return {
    protocol: 'aquarius',
    adapter_version: 1,
    verification: 'registered_router_events_and_transfers',
    router_contract: router,
    router_wasm_hash: null,
    mode: 'exact_input',
    routing,
    trader,
    input_limit: maximum,
    output_limit: '9999999',
    path: chained ? [trader, intermediateToken, nativeToken] : [trader, nativeToken],
    input,
    output,
    hops: [
      {
        index: 0,
        pool_tokens: [trader, chained ? intermediateToken : nativeToken],
        pool_index: 'ab'.repeat(32),
        pool_contract: transaction.calls[0]!.contract_id,
        input,
        output: chained ? intermediate : output,
        input_event_index: chained ? 1 : 0,
        output_event_index: chained ? 2 : 1,
        trade_event_index: chained ? 3 : 2,
        router_event_index: chained ? 4 : 3,
      },
      ...(chained
        ? [
            {
              index: 1,
              pool_tokens: [intermediateToken, nativeToken],
              pool_index: 'cd'.repeat(32),
              pool_contract: intermediateToken,
              input: intermediate,
              output,
              input_event_index: 5,
              output_event_index: 6,
              trade_event_index: 7,
              router_event_index: 8,
            },
          ]
        : []),
    ],
    transfer_event_indexes: chained ? [0, 1, 2, 5, 6, 9] : [0, 1],
  }
}

function interpretedAction(evidence?: AquariusSwap, functionName = 'swap'): Action {
  return {
    id: 'operation:0',
    kind: evidence ? 'swap' : 'contract_call',
    title: evidence ? 'Aquarius exchange' : `Call ${functionName}`,
    description: evidence
      ? 'Exchange tokens through the registered Aquarius router.'
      : 'Invoke a contract function. No supported Aquarius exchange was established.',
    status: 'applied',
    source: 'operation',
    op_index: 0,
    event_indexes: evidence ? evidence.transfer_event_indexes : [],
    from: evidence ? trader : transaction.source_account,
    contract_id: router,
    function: evidence?.routing === 'chained' ? 'swap_chained' : functionName,
    amount: null,
    asset: null,
    result_code: null,
    ...(evidence
      ? { aquarius_swap: evidence, source_account: transaction.source_account, to: trader }
      : {}),
  }
}

for (const routing of ['direct', 'chained'] as const) {
  test(`Aquarius ${routing} shows exact limits, trader and settlement provenance`, async ({
    page,
  }) => {
    await page.goto('/transactions')
    const evidence = swap(routing)
    const tokenAction: Action = {
      ...interpretedAction(),
      id: 'event:0',
      kind: 'transfer',
      title: 'Token transfer',
      description: 'The same input funding transfer used by the interpreted exchange.',
      source: 'token_event',
      from: trader,
      event_indexes: [0],
      amount: maximum,
      asset: evidence.input.asset,
    }
    await page.route(`**/api/v1/transactions/${transaction.hash}`, (route) =>
      route.fulfill({
        json: {
          ...transaction,
          interpretation: {
            ...transaction.interpretation,
            version: 4,
            actions: [interpretedAction(evidence), tokenAction],
          },
        },
      }),
    )
    await page.locator(`a[href="/tx/${transaction.hash}"]`).first().click()
    const action = page.getByRole('article', { name: 'Aquarius exchange', exact: true })
    const panel = action.getByRole('region', { name: 'Aquarius execution', exact: true })
    await expect(panel.getByText('Amount sent', { exact: true })).toHaveCount(1)
    await expect(panel.getByText('Amount received', { exact: true })).toHaveCount(1)
    await expect(panel).toContainText(maximum)
    await expect(panel).toContainText('base units')
    await expect(panel.getByText('Requested input', { exact: true })).toBeVisible()
    await expect(panel.getByText('Funded input', { exact: true })).toHaveCount(0)
    await expect(
      panel.locator('dt', { hasText: /^Amount received$/ }).locator('+ dd'),
    ).toContainText('1.0000000 XLM')
    await expect(
      panel.locator('dt', { hasText: /^Minimum output$/ }).locator('+ dd'),
    ).toContainText('0.9999999 XLM')
    await expect(panel.getByText('Deadline', { exact: true })).toHaveCount(0)
    await expect(panel.getByText('Pool fee', { exact: true })).toHaveCount(0)
    await expect(panel.locator('dt', { hasText: /^Trader$/ }).locator('+ dd a')).toHaveAttribute(
      'href',
      `/contracts/${trader}`,
    )
    await expect(
      action.locator('dt', { hasText: /^Operation source$/ }).locator('+ dd a'),
    ).toHaveAttribute('href', `/accounts/${transaction.source_account}`)
    await expect(page.getByRole('article', { name: 'Token transfer', exact: true })).toBeVisible()
    if (routing === 'chained') {
      await expect(panel).toContainText('Router funding and settlement transfers are included once')
      await expect(panel).toContainText('Intermediate hop amounts are not added')
    } else {
      await expect(panel).toContainText('Direct pool')
    }
    await panel
      .getByText(
        `Aquarius route (${evidence.hops.length} ${routing === 'direct' ? 'pool' : 'pools'})`,
        {
          exact: true,
        },
      )
      .click()
    await expect(panel.locator('ol > li')).toHaveCount(evidence.hops.length)
    await expect(
      panel
        .locator('dt', { hasText: /^Pool index$/ })
        .first()
        .locator('+ dd'),
    ).toHaveText('ab'.repeat(32))
    await expect(
      panel
        .locator('dt', { hasText: /^Pool trade event$/ })
        .first()
        .locator('+ dd'),
    ).toHaveText(`#${evidence.hops[0]!.trade_event_index}`)
    await expect(
      panel
        .locator('dt', { hasText: /^Router event$/ })
        .first()
        .locator('+ dd'),
    ).toHaveText(`#${evidence.hops[0]!.router_event_index}`)
    await panel.getByText('Aquarius router and evidence', { exact: true }).click()
    await expect(panel).toContainText('The router WASM hash was not observed')
    await expect(panel).toContainText(
      'not derived or independently verified against a pool registry',
    )
    await expect(panel).toContainText('do not establish a match to pinned or audited code')
    await expect(panel.locator('dt', { hasText: /^Transfer events$/ }).locator('+ dd')).toHaveText(
      evidence.transfer_event_indexes.map((index) => `#${index}`).join(', '),
    )
    for (const width of [320, 390, 768, 1440]) {
      await page.setViewportSize({ width, height: 900 })
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
      ).toBeTruthy()
      if (width > 600) {
        const gap = await panel.getByText('Requested input', { exact: true }).evaluate((label) => {
          const value = label.nextElementSibling!
          return value.getBoundingClientRect().left - label.getBoundingClientRect().right
        })
        expect(gap).toBeGreaterThanOrEqual(12)
      }
    }
    await panel.getByText('Aquarius assets, pools and source evidence', { exact: true }).click()
    await expect(panel.locator('pre')).toContainText('"registered_router_events_and_transfers"')
    await expect(panel.locator('pre')).toContainText('"router_wasm_hash": null')
  })
}

test('an observed Aquarius executable is displayed without a pinned-code claim', async ({
  page,
}) => {
  await page.goto('/transactions')
  const evidence = { ...swap('direct'), router_wasm_hash: 'ab'.repeat(32) }
  await page.route(`**/api/v1/transactions/${transaction.hash}`, (route) =>
    route.fulfill({
      json: {
        ...transaction,
        interpretation: {
          ...transaction.interpretation,
          version: 4,
          actions: [interpretedAction(evidence)],
        },
      },
    }),
  )
  await page.locator(`a[href="/tx/${transaction.hash}"]`).first().click()
  const panel = page.getByRole('region', { name: 'Aquarius execution', exact: true })
  await panel.getByText('Aquarius router and evidence', { exact: true }).click()
  await expect(panel.locator('dt', { hasText: /^Observed WASM hash$/ }).locator('+ dd')).toHaveText(
    evidence.router_wasm_hash,
  )
  await expect(panel).toContainText('do not establish a match to pinned or audited code')
  await expect(
    panel.getByText('The router WASM hash was not observed', { exact: false }),
  ).toHaveCount(0)
})

for (const functionName of ['swap_chained_strict_receive', 'swap_split']) {
  test(`Aquarius ${functionName} without reconciled evidence remains a contract call`, async ({
    page,
  }) => {
    await page.goto('/transactions')
    await page.route(`**/api/v1/transactions/${transaction.hash}`, (route) =>
      route.fulfill({
        json: {
          ...transaction,
          interpretation: {
            ...transaction.interpretation,
            version: 4,
            actions: [interpretedAction(undefined, functionName)],
          },
        },
      }),
    )
    await page.locator(`a[href="/tx/${transaction.hash}"]`).first().click()
    const action = page.getByRole('article', { name: `Call ${functionName}`, exact: true })
    await expect(action).toBeVisible()
    await expect(action).toContainText('No supported Aquarius exchange was established')
    await expect(page.getByRole('region', { name: 'Aquarius execution', exact: true })).toHaveCount(
      0,
    )
    await expect(action.getByText('Amount received', { exact: true })).toHaveCount(0)
  })
}

for (const hasRefund of [true, false]) {
  test(`Aquarius strict receive separates net spending from funding ${hasRefund ? 'with' : 'without'} a return`, async ({
    page,
  }) => {
    await page.goto('/transactions')
    const baseline = swap('chained')
    const spent = hasRefund ? '170141183460469231731687303715884105726' : maximum
    const evidence: AquariusSwap = {
      ...baseline,
      adapter_version: 2,
      mode: 'exact_output',
      output_limit: baseline.output.amount,
      input: { ...baseline.input, amount: spent },
      hops: baseline.hops.map((hop, index) =>
        index === 0 ? { ...hop, input: { ...hop.input, amount: spent } } : hop,
      ),
      settlement: {
        funded_amount: maximum,
        refunded_amount: hasRefund ? '1' : '0',
        funding_event_index: 0,
        refund_event_index: hasRefund ? 10 : null,
        payout_event_index: 9,
      },
      transfer_event_indexes: [...baseline.transfer_event_indexes, ...(hasRefund ? [10] : [])],
    }
    await page.route(`**/api/v1/transactions/${transaction.hash}`, (route) =>
      route.fulfill({
        json: {
          ...transaction,
          interpretation: {
            ...transaction.interpretation,
            version: 5,
            actions: [
              {
                ...interpretedAction(evidence),
                function: 'swap_chained_strict_receive',
              },
            ],
          },
        },
      }),
    )
    await page.locator(`a[href="/tx/${transaction.hash}"]`).first().click()
    const panel = page.getByRole('region', { name: 'Aquarius execution', exact: true })
    await expect(panel).toContainText('Aquarius · Exact output')
    await expect(panel.locator('dt', { hasText: /^Amount spent$/ }).locator('+ dd')).toContainText(
      spent,
    )
    await expect(panel.locator('dt', { hasText: /^Maximum input$/ }).locator('+ dd')).toContainText(
      maximum,
    )
    await expect(panel.locator('dt', { hasText: /^Funded input$/ }).locator('+ dd')).toContainText(
      maximum,
    )
    await expect(
      panel.locator('dt', { hasText: /^Returned input$/ }).locator('+ dd'),
    ).toContainText(`${hasRefund ? '1' : '0'} base units`)
    await expect(
      panel.locator('dt', { hasText: /^Requested output$/ }).locator('+ dd'),
    ).toContainText('1.0000000 XLM')
    await expect(panel.getByText('Minimum output', { exact: true })).toHaveCount(0)
    await expect(panel).toContainText('This token return is separate from transaction fee refunds')
    await panel.getByText('Router settlement evidence', { exact: true }).click()
    await expect(panel.locator('dt', { hasText: /^Funding event$/ }).locator('+ dd')).toHaveText(
      '#0',
    )
    await expect(panel.locator('dt', { hasText: /^Payout event$/ }).locator('+ dd')).toHaveText(
      '#9',
    )
    await expect(
      panel.locator('dt', { hasText: /^Input return event$/ }).locator('+ dd'),
    ).toHaveText(hasRefund ? '#10' : 'No input return event')
    for (const width of [320, 390, 768, 1440]) {
      await page.setViewportSize({ width, height: 900 })
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
      ).toBeTruthy()
    }
    await panel.getByText('Aquarius assets, pools and source evidence', { exact: true }).click()
    await expect(panel.locator('pre')).toContainText(
      `"refund_event_index": ${hasRefund ? '10' : 'null'}`,
    )
  })
}

test('Aquarius split routes preserve branch weights and count endpoint totals once', async ({
  page,
}) => {
  await page.goto('/transactions')
  const baseline = swap('chained')
  const input = { ...baseline.input, amount: '10000000' }
  const output = baseline.output
  const firstInput = { ...input, amount: '4000000' }
  const secondInput = { ...input, amount: '6000000' }
  const intermediate = { ...baseline.hops[0]!.output, amount: '8000000' }
  const firstOutput = { ...output, amount: '3000000' }
  const secondOutput = { ...output, amount: '7000000' }
  const evidence: AquariusSwap = {
    ...baseline,
    adapter_version: 2,
    routing: 'split',
    path: [],
    input_limit: input.amount,
    input,
    output,
    routes: [
      {
        index: 0,
        weight_bps: 4000,
        path: [trader, intermediateToken, nativeToken],
        input: firstInput,
        output: firstOutput,
        hop_indexes: [0, 1],
      },
      {
        index: 1,
        weight_bps: 6000,
        path: [trader, nativeToken],
        input: secondInput,
        output: secondOutput,
        hop_indexes: [2],
      },
    ],
    hops: [
      {
        ...baseline.hops[0]!,
        input: firstInput,
        output: intermediate,
        pool_contract: 'CAPR6HY7D4PR6HY7D4PR6HY7D4PR6HY7D4PR6HY7D4PR6HY7D4PR77LP',
      },
      {
        ...baseline.hops[1]!,
        input: intermediate,
        output: firstOutput,
        pool_contract: 'CAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAKAL',
      },
      {
        ...baseline.hops[0]!,
        index: 2,
        pool_tokens: [trader, nativeToken],
        input: secondInput,
        output: secondOutput,
        pool_index: 'ef'.repeat(32),
        pool_contract: 'CAQSCIJBEEQSCIJBEEQSCIJBEEQSCIJBEEQSCIJBEEQSCIJBEEQSDFYJ',
        input_event_index: 9,
        output_event_index: 10,
        trade_event_index: 11,
        router_event_index: 12,
      },
    ],
    settlement: {
      funded_amount: input.amount,
      refunded_amount: '0',
      funding_event_index: 0,
      refund_event_index: null,
      payout_event_index: 13,
    },
    transfer_event_indexes: [0, 1, 2, 5, 6, 9, 10, 13],
  }
  await page.route(`**/api/v1/transactions/${transaction.hash}`, (route) =>
    route.fulfill({
      json: {
        ...transaction,
        interpretation: {
          ...transaction.interpretation,
          version: 5,
          actions: [{ ...interpretedAction(evidence), function: 'swap_split' }],
        },
      },
    }),
  )
  await page.locator(`a[href="/tx/${transaction.hash}"]`).first().click()
  const panel = page.getByRole('region', { name: 'Aquarius execution', exact: true })
  await expect(panel).toContainText('Split routes through router')
  await expect(panel.getByText('Amount sent', { exact: true })).toHaveCount(1)
  await expect(panel.getByText('Amount received', { exact: true })).toHaveCount(1)
  await expect(panel.locator('dt', { hasText: /^Amount sent$/ }).locator('+ dd')).toContainText(
    '10000000 base units',
  )
  await expect(panel.locator('dt', { hasText: /^Amount received$/ }).locator('+ dd')).toContainText(
    '1.0000000 XLM',
  )
  await panel.getByText('Split routes (2)', { exact: true }).click()
  const routes = panel.locator('.aquarius-split-routes > li')
  await expect(routes).toHaveCount(2)
  await expect(routes.nth(0)).toContainText('Route 1 · 40.00%')
  await expect(routes.nth(0)).toContainText('4000000 base units')
  await expect(routes.nth(0)).toContainText('0.3000000 XLM')
  await expect(routes.nth(0)).toContainText('Pool evidence #1, #2')
  await expect(routes.nth(0).locator('a[href^="/assets/"]')).toHaveCount(3)
  await expect(routes.nth(1)).toContainText('Route 2 · 60.00%')
  await expect(routes.nth(1)).toContainText('Pool evidence #3')
  await expect(routes.nth(1).locator('a[href^="/assets/"]')).toHaveCount(2)
  await panel.getByText('Pool evidence (3 pools)', { exact: true }).click()
  await expect(panel.locator('dt', { hasText: /^Pool index$/ })).toHaveCount(3)
  await expect(panel).toContainText("The total sent and received sum each branch's endpoints once")
  for (const width of [320, 390, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 })
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBeTruthy()
  }
  await panel.getByText('Aquarius assets, pools and source evidence', { exact: true }).click()
  await expect(panel.locator('pre')).toContainText('"path": []')
  await expect(panel.locator('pre')).toContainText('"weight_bps": 4000')
})
