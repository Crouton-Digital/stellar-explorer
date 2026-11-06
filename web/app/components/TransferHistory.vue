<script setup lang="ts">
import type { Page, TokenTransfer } from '~/types'

const props = defineProps<{ address: string; kind: 'accounts' | 'contracts' }>()
const cursor = ref<string | undefined>()

watch(
  () => [props.address, props.kind],
  () => {
    cursor.value = undefined
  },
)

const { data, pending, error, refresh } = await useLazyFetch<Page<TokenTransfer>>(
  computed(() => `/api/v1/${props.kind}/${encodeURIComponent(props.address)}/transfers`),
  { query: computed(() => ({ cursor: cursor.value })) },
)
</script>

<template>
  <section class="panel">
    <div class="panel-heading"><h2>Token movements</h2></div>
    <div class="token-note muted small">
      Committed events in the indexed range. Token balances and total supply are not calculated.
    </div>
    <StateView
      :pending="pending"
      :error="error"
      @retry="refresh"
    >
      <TransferTable
        v-if="data?.items.length"
        :items="data.items"
      />
      <p
        v-else
        class="empty-inline padded"
      >
        No decoded token movements in this indexed history.
      </p>
    </StateView>
    <div
      v-if="cursor || data?.next_cursor"
      class="page-nav"
    >
      <button
        v-if="cursor"
        class="subtle-button"
        @click="cursor = undefined"
      >
        ↺ Newest
      </button>
      <button
        v-if="data?.next_cursor"
        class="button"
        @click="cursor = data.next_cursor || undefined"
      >
        Older movements →
      </button>
    </div>
  </section>
</template>
