<script setup lang="ts">
import type { NFTCollection, Page } from '~/types'

const route = useRoute()
const { data, pending, error, refresh } = await useLazyFetch<Page<NFTCollection>>(
  '/api/v1/nfts/collections',
  { query: computed(() => route.query) },
)

useHead({ title: 'NFT collections · Crouton' })
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow">EXPLORER / NFTS</div>
      <h1>NFT collections</h1>
      <p>Contracts observed in supported NFT movement events.</p>
    </div>
    <NuxtLink
      class="subtle-button"
      to="/nfts/transfers"
    >
      NFT movements →
    </NuxtLink>
  </div>
  <div class="notice">
    Event format matches do not certify a collection. Observed token IDs and recorded movements
    cover indexed history at this snapshot; they are not total supply or current ownership.
  </div>
  <section class="panel">
    <StateView
      :pending="pending"
      :error="error"
      :empty="!data?.items.length"
      empty-title="No NFT collections observed yet"
      empty-message="Supported committed NFT events will appear here as they are indexed."
      @retry="refresh"
    >
      <div class="table-scroll">
        <table
          class="responsive-table"
          aria-label="Observed NFT collections"
        >
          <thead>
            <tr>
              <th>Collection contract</th>
              <th>Identification</th>
              <th>Observed token IDs</th>
              <th>Recorded movements</th>
              <th>First observed ledger</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="collection in data?.items"
              :key="collection.contract_id"
            >
              <td data-label="Collection">
                <EntityLink
                  kind="nfts"
                  :value="collection.contract_id"
                />
              </td>
              <td data-label="Identification">Event format match</td>
              <td
                class="mono"
                data-label="Observed IDs"
              >
                {{ collection.observed_token_count.toLocaleString('en-US') }}
              </td>
              <td
                class="mono"
                data-label="Movements"
              >
                {{ collection.transfer_count.toLocaleString('en-US') }}
              </td>
              <td data-label="First observed">
                <EntityLink
                  kind="ledgers"
                  :value="String(collection.first_observed_ledger)"
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
