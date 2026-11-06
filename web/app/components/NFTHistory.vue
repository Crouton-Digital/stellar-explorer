<script setup lang="ts">
import type { NFTTransfer, Page } from '~/types'

const props = defineProps<{ address: string; kind: 'accounts' | 'contracts' }>()
const cursor = ref<string | undefined>()

watch(
  () => [props.address, props.kind],
  () => {
    cursor.value = undefined
  },
)

const { data, pending, error, refresh } = await useLazyFetch<Page<NFTTransfer>>(
  computed(() => `/api/v1/${props.kind}/${encodeURIComponent(props.address)}/nft-transfers`),
  { query: computed(() => ({ cursor: cursor.value })) },
)
</script>

<template>
  <section class="panel">
    <div class="panel-heading"><h2>NFT movements</h2></div>
    <p class="token-note muted small">
      Supported NFT events in indexed history. A recorded recipient does not establish the current
      owner.
    </p>
    <StateView
      :pending="pending"
      :error="error"
      @retry="refresh"
    >
      <NFTTransferTable
        v-if="data?.items.length"
        :items="data.items"
      />
      <p
        v-else
        class="empty-inline padded"
      >
        No decoded NFT movements in this indexed history.
      </p>
    </StateView>
    <div
      v-if="cursor || data?.next_cursor"
      class="page-nav"
    >
      <button
        v-if="cursor"
        class="subtle-button"
        type="button"
        @click="cursor = undefined"
      >
        ↺ Newest NFT movements
      </button>
      <button
        v-if="data?.next_cursor"
        class="button"
        type="button"
        @click="cursor = data.next_cursor || undefined"
      >
        Older NFT movements →
      </button>
    </div>
  </section>
</template>
