<script setup lang="ts">
const props = defineProps<{ value: string }>()
const status = ref('Copy')

async function copy() {
  try {
    await navigator.clipboard.writeText(props.value)
    status.value = 'Copied'
  } catch {
    status.value = 'Copy unavailable'
  }
}
</script>

<template>
  <div class="copy-value">
    <code>{{ value }}</code>
    <button
      class="subtle-button"
      :aria-label="`Copy ${value}`"
      @click="copy"
    >
      {{ status }}
    </button>
    <span
      class="sr-only"
      aria-live="polite"
    >
      {{ status }}
    </span>
  </div>
</template>
