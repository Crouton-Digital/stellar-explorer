<script setup lang="ts">
import type { AddressDetail } from '~/types'

const route = useRoute()
const url = computed(() => `/api/v1/accounts/${encodeURIComponent(String(route.params.address))}`)
const query = computed(() => ({
  cursor: typeof route.query.cursor === 'string' ? route.query.cursor : undefined,
}))

const { data, pending, error, refresh } = await useLazyFetch<AddressDetail>(url, { query })

const activityFilters = computed(() => ({
  address: data.value?.address,
  max_ledger: String(data.value?.transactions.items[0]?.ledger_sequence || ''),
}))
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow">EXPLORER / ACCOUNT</div>
      <h1>Account activity</h1>
      <p>Observed activity in the indexed Testnet range.</p>
    </div>
  </div>
  <StateView
    :pending="pending"
    :error="error"
    @retry="refresh"
  >
    <template v-if="data">
      <section class="panel padded">
        <CopyValue :value="data.address" />
        <div class="notice">
          Indexed range: {{ data.range.first_indexed_ledger }}–{{ data.range.last_indexed_ledger }}.
          Balance and complete account history are not available from this index.
        </div>
        <p class="muted small">Included participants: {{ data.history_semantics }}.</p>
        <a
          :href="`https://horizon-testnet.stellar.org/accounts/${data.address}`"
          rel="noopener noreferrer"
          target="_blank"
        >
          Account on Horizon Testnet ↗
        </a>
      </section>
      <ActivitySummary
        v-if="data.transactions.items.length"
        kind="transactions"
        :query="activityFilters"
        variant="line"
        title="Account transaction activity"
        caption="The window ends at the newest transaction on this page. It counts this account's indexed activity."
      />
      <section class="panel">
        <div class="panel-heading"><h2>Indexed transactions</h2></div>
        <StateView :empty="!data.transactions.items.length">
          <TxTable :items="data.transactions.items" />
        </StateView>
        <PageNav :cursor="data.transactions.next_cursor" />
      </section>
      <TransferHistory
        :address="data.address"
        kind="accounts"
      />
      <NFTHistory
        :address="data.address"
        kind="accounts"
      />
    </template>
  </StateView>
</template>
