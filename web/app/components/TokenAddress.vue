<script setup lang="ts">
defineProps<{ address?: string; empty: string }>()
</script>

<template>
  <EntityLink
    v-if="address?.startsWith('G') || address?.startsWith('C')"
    :value="address"
    :kind="address.startsWith('G') ? 'accounts' : 'contracts'"
  />
  <span
    v-else
    class="muted small"
    :title="address"
    >{{ address || empty }}</span
  >
</template>
