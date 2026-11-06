<script setup lang="ts">
import { short } from '~/utils/format.mjs'

const props = withDefaults(
  defineProps<{
    value: string
    kind?: 'tx' | 'accounts' | 'contracts' | 'ledgers' | 'assets' | 'nfts'
    full?: boolean
  }>(),
  { kind: 'tx', full: false },
)
</script>

<template>
  <NuxtLink
    class="entity mono"
    :to="`/${props.kind}/${props.value}`"
    :title="props.value"
  >
    {{ full ? value : short(value) }}
  </NuxtLink>
</template>
