<script setup lang="ts">
import type { Asset } from '~/types'
import { tokenAmount } from '~/utils/format.mjs'

const props = defineProps<{ amount: string; asset: Asset }>()

const formattedAmount = computed(() => {
  const negative = props.amount.startsWith('-')
  const quantity = negative ? props.amount.slice(1) : props.amount
  const formatted = tokenAmount(quantity, props.asset.decimals)

  return formatted === '—' ? formatted : `${negative ? '-' : ''}${formatted}`
})
</script>

<template>
  <span
    class="mono token-amount"
    :title="`${amount} base units`"
  >
    {{ formattedAmount }}
    <span class="muted small">{{ asset.decimals === null ? 'base units' : asset.code || '' }}</span>
  </span>
</template>
