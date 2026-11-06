<script setup lang="ts">
import type { Ledger, Page } from '~/types'

const route = useRoute()
const query = computed(() => ({
  cursor: typeof route.query.cursor === 'string' ? route.query.cursor : undefined,
}))

const { data, pending, error, refresh } = await useLazyFetch<Page<Ledger>>('/api/v1/ledgers', {
  query,
})

useHead({ title: 'Ledgers · Crouton' })
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow">EXPLORER / LEDGERS</div>
      <h1>Ledgers</h1>
      <p>Closed ledgers in application order. Newest first.</p>
    </div>
  </div>
  <section class="panel">
    <StateView
      :pending="pending"
      :error="error"
      :empty="!data?.items.length"
      @retry="refresh"
    >
      <LedgerTable :items="data?.items || []" />
    </StateView>
    <PageNav :cursor="data?.next_cursor" />
  </section>
</template>
