<script setup lang="ts">
import type { Page, TokenTransfer } from '~/types'

const route = useRoute()
const { data, pending, error, refresh } = await useLazyFetch<Page<TokenTransfer>>(
  '/api/v1/transfers',
  {
    query: computed(() => route.query),
  },
)

useHead({ title: 'Token movements · Crouton' })
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow">EXPLORER / TOKEN MOVEMENTS</div>
      <h1>Token movements</h1>
      <p>Decoded committed movements with exact amounts and explicit asset identification.</p>
    </div>
  </div>
  <FilterForm kind="transfers" />
  <ActivitySummary
    kind="transfers"
    :query="route.query"
  />
  <section class="panel">
    <StateView
      :pending="pending"
      :error="error"
      :empty="!data?.items.length"
      empty-title="No token movements match this view"
      empty-message="Adjust or reset the filters. Balances and total supply are not calculated."
      @retry="refresh"
    >
      <TransferTable :items="data?.items || []" />
    </StateView>
    <PageNav :cursor="data?.next_cursor" />
  </section>
</template>
