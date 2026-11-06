<script setup lang="ts">
defineProps<{ cursor: string | null | undefined }>()
const route = useRoute()
const filters = computed(() => {
  const { cursor: _cursor, ...query } = route.query
  return query
})
</script>

<template>
  <div
    v-if="cursor || route.query.cursor"
    class="page-nav"
  >
    <NuxtLink
      v-if="route.query.cursor"
      class="subtle-button"
      :to="{ path: route.path, query: filters }"
    >
      ↺ Newest
    </NuxtLink>
    <NuxtLink
      v-if="cursor"
      class="button"
      :to="{ path: route.path, query: { ...filters, cursor } }"
    >
      Older results →
    </NuxtLink>
  </div>
</template>
