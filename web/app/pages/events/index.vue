<script setup lang="ts">
import type { Event, Page } from '~/types'

const route = useRoute()
const { data, pending, error, refresh } = await useLazyFetch<Page<Event>>('/api/v1/events', {
  query: computed(() => route.query),
})

useHead({ title: 'Events · Crouton' })
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow">EXPLORER / EVENTS</div>
      <h1>Events</h1>
      <p>
        Committed operation and transaction events, including fees and refunds. Diagnostic traces
        remain on transaction pages.
      </p>
    </div>
  </div>
  <FilterForm kind="events" />
  <section class="panel padded">
    <StateView
      :pending="pending"
      :error="error"
      :empty="!data?.items.length"
      empty-title="No events match this view"
      empty-message="Adjust or reset the filters. Only committed indexed events are included."
      @retry="refresh"
    >
      <EventList :items="data?.items || []" />
    </StateView>
  </section>
  <PageNav :cursor="data?.next_cursor" />
</template>
