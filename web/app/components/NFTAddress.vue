<script setup lang="ts">
import { short } from '~/utils/format.mjs'

defineProps<{ address?: string; direction: 'from' | 'to'; empty: string }>()
</script>

<template>
  <NuxtLink
    v-if="address"
    class="entity mono"
    :to="{ path: '/nfts/transfers', query: { [direction]: address } }"
    :title="`${address} · NFT movement history`"
  >
    {{ short(address) }}
  </NuxtLink>
  <span
    v-else
    class="muted small"
  >
    {{ empty }}
  </span>
</template>
