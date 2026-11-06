<script setup lang="ts">
import { apiError } from '~/utils/format.mjs'
withDefaults(
  defineProps<{
    pending?: boolean
    error?: any
    empty?: boolean
    emptyTitle?: string
    emptyMessage?: string
  }>(),
  {
    emptyTitle: 'No indexed activity yet',
    emptyMessage:
      'Start the indexer or wait for the next ledger. This view covers the indexed range only.',
  },
)
defineEmits<{ retry: [] }>()
</script>

<template>
  <div
    v-if="pending"
    class="state"
    role="status"
  >
    <span class="spinner" /> Loading indexed data…
  </div>
  <div
    v-else-if="error"
    class="state error"
    role="alert"
  >
    <strong>Unable to load this view</strong>
    <p>{{ apiError(error) }}</p>
    <button
      class="button"
      @click="$emit('retry')"
    >
      Try again
    </button>
  </div>
  <div
    v-else-if="empty"
    class="state"
  >
    <strong>{{ emptyTitle }}</strong>
    <p>{{ emptyMessage }}</p>
  </div>
  <slot v-else />
</template>
