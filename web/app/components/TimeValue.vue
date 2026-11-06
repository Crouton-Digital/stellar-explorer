<script setup lang="ts">
const props = defineProps<{ value: string }>()
const local = ref('')

onMounted(() => {
  local.value = new Date(props.value).toLocaleString()
})

watch(
  () => props.value,
  (value) => {
    local.value = new Date(value).toLocaleString()
  },
)
</script>

<template>
  <time
    :datetime="value"
    :title="local ? `Local: ${local}` : value"
  >
    {{ new Date(value).toISOString().replace('T', ' ').slice(0, 19) }} UTC
  </time>
</template>
