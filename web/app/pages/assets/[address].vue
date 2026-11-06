<script setup lang="ts">
import type { AssetDetail } from '~/types'

const route = useRoute()
const url = computed(() => `/api/v1/assets/${encodeURIComponent(String(route.params.address))}`)
const query = computed(() => ({
  cursor: typeof route.query.cursor === 'string' ? route.query.cursor : undefined,
}))
const { data, pending, error, refresh } = await useLazyFetch<AssetDetail>(url, { query })

useHead({ title: 'Asset activity · Crouton' })
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow"><NuxtLink to="/assets">ASSETS</NuxtLink> / DETAIL</div>
      <h1>Asset activity</h1>
      <p>Observed token movements on Stellar Testnet.</p>
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
          <dt>Token contract</dt>
          <dd><CopyValue :value="data.asset.contract_id" /></dd>
          <dt>Identification</dt>
          <dd>
            {{
              data.asset.kind === 'sac'
                ? 'Verified Stellar Asset Contract'
                : 'SEP-41-compatible event format'
            }}
          </dd>
          <dt>Asset code</dt>
          <dd>{{ data.asset.code || 'Unknown' }}</dd>
          <dt>Decimals</dt>
          <dd>{{ data.asset.decimals ?? 'Unknown · amounts shown in base units' }}</dd>
          <dt>Issuer</dt>
          <dd>
            <code v-if="data.asset.issuer">{{ data.asset.issuer }}</code>
            <span
              v-else
              class="muted"
              >{{ data.asset.classic_asset === 'native' ? 'Native XLM' : 'Unknown' }}</span
            >
          </dd>
          <dt>Contract events</dt>
          <dd>
            <EntityLink
              kind="contracts"
              :value="data.asset.contract_id"
            />
          </dd>
        </dl>
      </section>
      <div class="notice">
        Partial indexed history: {{ data.range.first_indexed_ledger }}–{{
          data.range.last_indexed_ledger
        }}. Balances, total supply, and contract compliance are not established by these events.
      </div>
      <section class="panel">
        <div class="panel-heading">
          <h2>Token movements</h2>
          <NuxtLink :to="{ path: '/transfers', query: { asset: data.asset.contract_id } }">
            Filter movements →
          </NuxtLink>
        </div>
        <TransferTable
          v-if="data.transfers.items.length"
          :items="data.transfers.items"
        />
        <p
          v-else
          class="empty-inline padded"
        >
          No decoded movements on this page.
        </p>
        <PageNav :cursor="data.transfers.next_cursor" />
      </section>
    </template>
  </StateView>
</template>
