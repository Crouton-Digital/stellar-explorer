<script setup lang="ts">
import type { Overview } from '~/types'

const { data, pending, error, refresh } = await useLazyFetch<Overview>('/api/v1/overview')
let timer: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  timer = setInterval(() => refresh(), 10000)
})

onBeforeUnmount(() => clearInterval(timer))

useHead({ title: 'Network overview · Crouton' })
</script>

<template>
  <section class="overview-hero">
    <div class="page-heading">
      <div>
        <div class="eyebrow">STELLAR / TESTNET</div>
        <h1>Network overview</h1>
        <p>Ledgers, transactions, and Soroban activity. All in one place.</p>
      </div>
      <button
        class="subtle-button"
        @click="refresh()"
      >
        ↻ Refresh
      </button>
    </div>
    <div
      v-if="data"
      class="stats-grid"
    >
      <div class="stat">
        <span>NETWORK LEDGER</span>
        <strong>{{ data.sync.latest_network_ledger?.toLocaleString('en-US') ?? '—' }}</strong>
        <small>Last observed RPC tip</small>
      </div>
      <div class="stat">
        <span>INDEXED LEDGER</span>
        <strong>{{ data.sync.last_indexed_ledger?.toLocaleString('en-US') ?? '—' }}</strong>
        <small>Atomically committed</small>
      </div>
      <div class="stat">
        <span>INDEXER LAG</span>
        <strong> {{ data.sync.lag ?? '—' }} <em>ledgers</em> </strong>
        <small :class="data.sync.last_error ? 'text-error' : 'text-success'">
          {{
            data.sync.last_error
              ? 'Indexer needs attention'
              : data.sync.lag === null
                ? 'Waiting for indexer'
                : data.sync.lag === 0
                  ? 'Caught up at last observation'
                  : 'Catching up'
          }}
        </small>
      </div>
      <div class="stat">
        <span>INDEXED FROM</span>
        <strong>{{ data.sync.first_indexed_ledger?.toLocaleString('en-US') ?? '—' }}</strong>
        <small>Partial Testnet history</small>
      </div>
    </div>
  </section>
  <StateView
    :pending="pending && !data"
    :error="error"
    @retry="refresh"
  >
    <template v-if="data">
      <div
        v-if="data.sync.last_error"
        class="notice error"
        role="alert"
      >
        Indexer: {{ data.sync.last_error }}
      </div>
      <div class="notice">
        <span aria-hidden="true">ⓘ</span> This explorer shows indexed Testnet data. Account activity
        and contract history may be incomplete.<span v-if="data.sync.observed_at">
          RPC observation: <TimeValue :value="data.sync.observed_at" />.
        </span>
      </div>
      <ActivitySummary
        kind="transactions"
        title="Recent transaction activity"
        :refresh-key="data.sync.last_indexed_ledger"
      />
      <section class="panel">
        <div class="panel-heading">
          <h2><span class="section-symbol">▤</span> Latest ledgers</h2>
          <NuxtLink to="/ledgers">All ledgers ↗</NuxtLink>
        </div>
        <StateView :empty="!data.ledgers.items.length">
          <LedgerTable :items="data.ledgers.items" />
        </StateView>
      </section>
      <section class="panel">
        <div class="panel-heading">
          <h2><span class="section-symbol">⇄</span> Recent transactions</h2>
          <NuxtLink to="/transactions">All transactions ↗</NuxtLink>
        </div>
        <StateView :empty="!data.transactions.items.length">
          <TxTable :items="data.transactions.items" />
        </StateView>
      </section>
    </template>
  </StateView>
</template>
