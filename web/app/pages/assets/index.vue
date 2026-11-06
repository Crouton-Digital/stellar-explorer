<script setup lang="ts">
import type { Asset, Page } from '~/types'
import { short } from '~/utils/format.mjs'

const route = useRoute()
const query = computed(() => ({
  cursor: typeof route.query.cursor === 'string' ? route.query.cursor : undefined,
}))
const { data, pending, error, refresh } = await useLazyFetch<Page<Asset>>('/api/v1/assets', {
  query,
})

useHead({ title: 'Assets · Crouton' })
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow">EXPLORER / TOKENS</div>
      <h1>Assets</h1>
      <p>Token contracts observed in decoded movement events.</p>
    </div>
  </div>
  <div class="notice">
    SAC identity is checked against the network-specific contract ID. Other contracts are recognized
    by event format; this does not certify their implementation or metadata.
  </div>
  <section class="panel">
    <StateView
      :pending="pending"
      :error="error"
      :empty="!data?.items.length"
      @retry="refresh"
    >
      <div class="table-scroll">
        <table
          class="responsive-table"
          aria-label="Observed assets"
        >
          <thead>
            <tr>
              <th>Asset</th>
              <th>Contract</th>
              <th>Identification</th>
              <th>Decimals</th>
              <th>First observed ledger</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="asset in data?.items"
              :key="asset.contract_id"
            >
              <td data-label="Asset">
                <NuxtLink
                  class="entity"
                  :to="`/assets/${asset.contract_id}`"
                >
                  {{ asset.code || short(asset.contract_id) }}
                </NuxtLink>
              </td>
              <td data-label="Contract">
                <EntityLink
                  kind="assets"
                  :value="asset.contract_id"
                />
              </td>
              <td data-label="Identification">
                <span class="protocol">{{
                  asset.kind === 'sac' ? 'Verified SAC' : 'Event format match'
                }}</span>
              </td>
              <td
                class="mono"
                data-label="Decimals"
              >
                {{ asset.decimals ?? 'Unknown' }}
              </td>
              <td data-label="First observed">
                <EntityLink
                  kind="ledgers"
                  :value="String(asset.first_observed_ledger)"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </StateView>
    <PageNav :cursor="data?.next_cursor" />
  </section>
</template>
