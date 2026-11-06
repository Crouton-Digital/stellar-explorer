<script setup lang="ts">
import type { Transaction, Page } from '~/types'

const route = useRoute()
const query = computed(() => route.query)
const filtered = computed(() =>
  ['address', 'contract', 'function', 'status', 'min_ledger', 'max_ledger'].some(
    (key) => route.query[key],
  ),
)

const { data, pending, error, refresh } = await useLazyFetch<Page<Transaction>>(
  '/api/v1/transactions',
  { query },
)

useHead({ title: 'Transactions · Crouton' })
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow">EXPLORER / TRANSACTIONS</div>
      <h1>Transactions</h1>
      <p>Classic operations and Soroban invocations in the indexed range.</p>
    </div>
  </div>
  <FilterForm kind="transactions" />
  <ActivitySummary
    kind="transactions"
    :query="query"
  />
  <section class="panel">
    <StateView
      :pending="pending"
      :error="error"
      :empty="!data?.items.length"
      :empty-title="filtered ? 'No transactions match these filters' : undefined"
      :empty-message="
        filtered ? 'Adjust or reset the filters to explore more indexed history.' : undefined
      "
      @retry="refresh"
    >
      <TxTable :items="data?.items || []" />
    </StateView>
    <PageNav :cursor="data?.next_cursor" />
  </section>
</template>
