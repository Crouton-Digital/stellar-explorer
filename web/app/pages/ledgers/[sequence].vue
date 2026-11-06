<script setup lang="ts">
import type { Ledger, Page, Transaction } from '~/types'

const route = useRoute()
const url = computed(() => `/api/v1/ledgers/${encodeURIComponent(String(route.params.sequence))}`)
const query = computed(() => ({
  cursor: typeof route.query.cursor === 'string' ? route.query.cursor : undefined,
}))

const { data, pending, error, refresh } = await useLazyFetch<{
  ledger: Ledger
  transactions: Page<Transaction>
}>(url, { query })
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow"><NuxtLink to="/ledgers">LEDGERS</NuxtLink> / DETAIL</div>
      <h1>Ledger #{{ route.params.sequence }}</h1>
      <p>A closed ledger on Stellar Testnet.</p>
    </div>
  </div>
  <StateView
    :pending="pending"
    :error="error"
    @retry="refresh"
  >
    <template v-if="data">
      <section class="panel detail-panel">
        <dl>
          <dt>Hash</dt>
          <dd><CopyValue :value="data.ledger.hash" /></dd>
          <dt>Closed at</dt>
          <dd><TimeValue :value="data.ledger.closed_at" /></dd>
          <dt>Protocol version</dt>
          <dd>{{ data.ledger.protocol_version }}</dd>
          <dt>Transactions</dt>
          <dd>{{ data.ledger.transaction_count }}</dd>
          <dt>Previous hash</dt>
          <dd><CopyValue :value="data.ledger.previous_hash" /></dd>
        </dl>
      </section>
      <section class="panel">
        <div class="panel-heading"><h2>Transactions</h2></div>
        <StateView :empty="!data.transactions.items.length">
          <TxTable :items="data.transactions.items" />
        </StateView>
        <PageNav :cursor="data.transactions.next_cursor" />
      </section>
    </template>
  </StateView>
</template>
