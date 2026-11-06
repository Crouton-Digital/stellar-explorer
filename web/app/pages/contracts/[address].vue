<script setup lang="ts">
import type { ContractDetail, Event, Page } from '~/types'

const route = useRoute()
const url = computed(() => `/api/v1/contracts/${encodeURIComponent(String(route.params.address))}`)
const query = computed(() => ({
  cursor: typeof route.query.cursor === 'string' ? route.query.cursor : undefined,
}))

const { data, pending, error, refresh } = await useLazyFetch<ContractDetail>(url, { query })
const activityFilters = computed(() => ({
  contract: data.value?.address,
  max_ledger: String(data.value?.transactions.items[0]?.ledger_sequence || ''),
}))
const eventCursor = ref<string | undefined>()

watch(
  () => route.params.address,
  () => {
    eventCursor.value = undefined
  },
)
const {
  data: eventPage,
  pending: eventPending,
  error: eventError,
  refresh: refreshEvents,
} = await useLazyFetch<Page<Event>>(
  computed(() => `${url.value}/events`),
  { query: computed(() => ({ cursor: eventCursor.value })) },
)
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow">EXPLORER / SOROBAN</div>
      <h1>Contract activity</h1>
      <p>Observed contract metadata, invocations and committed events.</p>
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
          This view does not establish the current contract state or full invocation history.
        </div>
      </section>
      <section
        class="panel padded"
        aria-labelledby="contract-metadata-heading"
      >
        <h2 id="contract-metadata-heading">Contract metadata</h2>
        <dl>
          <dt>Executable</dt>
          <dd>
            {{
              data.contract.kind === 'wasm'
                ? 'WASM'
                : data.contract.kind === 'sac'
                  ? 'Stellar Asset Contract (SAC)'
                  : 'Unknown'
            }}
          </dd>
          <dt>WASM hash</dt>
          <dd>
            <CopyValue
              v-if="data.contract.wasm_hash"
              :value="data.contract.wasm_hash"
            />
            <span v-else>{{ data.contract.kind === 'sac' ? 'Not applicable' : 'Unknown' }}</span>
          </dd>
          <dt>Executable evidence</dt>
          <dd>
            {{
              data.contract.executable_source === 'instance_entry'
                ? 'Contract instance entry'
                : data.contract.executable_source === 'asset_contract_id'
                  ? 'Verified asset contract ID'
                  : 'Unknown'
            }}
            <template v-if="data.contract.executable_ledger">
              · ledger
              <EntityLink
                :value="String(data.contract.executable_ledger)"
                kind="ledgers"
              />
            </template>
          </dd>
          <dt>First observed</dt>
          <dd>
            <EntityLink
              :value="String(data.contract.first_observed_ledger)"
              kind="ledgers"
            />
          </dd>
          <dt>Last observed</dt>
          <dd>
            <EntityLink
              :value="String(data.contract.last_observed_ledger)"
              kind="ledgers"
            />
          </dd>
          <dt>Creation ledger</dt>
          <dd>
            <EntityLink
              v-if="data.contract.creation_ledger"
              :value="String(data.contract.creation_ledger)"
              kind="ledgers"
            />
            <span v-else>Unknown · creation not observed</span>
          </dd>
          <dt>Creation transaction</dt>
          <dd>
            <EntityLink
              v-if="data.contract.creation_transaction"
              :value="data.contract.creation_transaction"
            />
            <span v-else>Unknown</span>
          </dd>
          <dt>Creator</dt>
          <dd>
            <TokenAddress
              :address="data.contract.creator || undefined"
              empty="Unknown"
            />
          </dd>
        </dl>
        <p class="notice">
          Metadata reflects the latest evidence in indexed history. First observed is not the
          creation date. The creator is shown only when a matching creation preimage identifies the
          deployer.
        </p>
      </section>
      <ActivitySummary
        v-if="data.transactions.items.length"
        kind="transactions"
        :query="activityFilters"
        variant="line"
        title="Contract transaction activity"
        caption="The window ends at the newest transaction on this page. It counts transactions related to this contract."
      />
      <section class="panel padded">
        <h2>Recent calls</h2>
        <template
          v-for="tx in data.transactions.items"
          :key="tx.hash"
        >
          <div
            v-for="call in tx.calls.filter((c) => c.contract_id === data?.address)"
            :key="`${tx.hash}:${call.op_index}`"
            class="activity-card"
          >
            <div class="section-heading">
              <strong class="function-name">{{ call.function_name }}(…)</strong>
              <span
                class="pill"
                :class="tx.successful ? 'success' : 'failed'"
              >
                {{ tx.successful ? 'Success' : 'Failed' }}
              </span>
            </div>
            <p><EntityLink :value="tx.hash" /></p>
            <JsonView
              :value="call.arguments"
              label="Typed arguments"
            />
          </div>
        </template>
        <p
          v-if="
            !data.transactions.items.some((t) =>
              t.calls.some((c) => c.contract_id === data?.address),
            )
          "
          class="empty-inline"
        >
          No top-level calls in these recent transactions.
        </p>
      </section>
      <section class="panel padded">
        <div class="section-heading">
          <h2>Contract events</h2>
          <NuxtLink :to="{ path: '/events', query: { contract: data.address } }">
            Filter events →
          </NuxtLink>
        </div>
        <StateView
          :pending="eventPending"
          :error="eventError"
          @retry="refreshEvents"
        >
          <EventList :items="eventPage?.items || []" />
        </StateView>
        <div class="page-nav">
          <button
            v-if="eventCursor"
            class="subtle-button"
            @click="eventCursor = undefined"
          >
            ↺ Newest
          </button>
          <button
            v-if="eventPage?.next_cursor"
            class="button"
            @click="eventCursor = eventPage?.next_cursor || undefined"
          >
            Older events →
          </button>
        </div>
      </section>
      <section class="panel">
        <div class="panel-heading"><h2>Related transactions</h2></div>
        <TxTable :items="data.transactions.items" />
        <PageNav :cursor="data.transactions.next_cursor" />
      </section>
      <TransferHistory
        :address="data.address"
        kind="contracts"
      />
      <NFTHistory
        :address="data.address"
        kind="contracts"
      />
    </template>
  </StateView>
</template>
