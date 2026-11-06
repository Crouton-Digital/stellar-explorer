<script setup lang="ts">
import type { NFTTransfer, Page } from '~/types'

const route = useRoute()
const { data, pending, error, refresh } = await useLazyFetch<Page<NFTTransfer>>(
  '/api/v1/nfts/transfers',
  { query: computed(() => route.query) },
)

useHead({ title: 'NFT movements · Crouton' })
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow"><NuxtLink to="/nfts">NFTS</NuxtLink> / MOVEMENTS</div>
      <h1>NFT movements</h1>
      <p>Committed transfers, mints and supported burn-extension events with exact token IDs.</p>
    </div>
  </div>
  <div class="notice">
    Indexed event evidence does not establish the current owner, total supply or collection
    compliance. NFT IDs are identifiers, not fungible token amounts.
  </div>
  <FilterForm kind="nfts" />
  <section class="panel">
    <StateView
      :pending="pending"
      :error="error"
      :empty="!data?.items.length"
      empty-title="No NFT movements match this view"
      empty-message="Adjust the filters or inspect another indexed range."
      @retry="refresh"
    >
      <NFTTransferTable :items="data?.items || []" />
    </StateView>
    <PageNav :cursor="data?.next_cursor" />
  </section>
</template>
