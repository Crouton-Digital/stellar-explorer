<script setup lang="ts">
defineProps<{ value: unknown; label?: string }>()
</script>

<template>
  <details class="json-view">
    <summary>{{ label || 'View raw data' }}</summary>
    <pre>{{ typeof value === 'string' ? value : JSON.stringify(value, null, 2) }}</pre>
  </details>
</template>
