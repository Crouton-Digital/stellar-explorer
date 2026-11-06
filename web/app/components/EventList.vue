<script setup lang="ts">
import type { Event } from '~/types'
defineProps<{ items: Event[] }>()

function stageLabel(stage: string | null) {
  const labels: Record<string, string> = {
    before_all_transactions: 'Before all transactions',
    after_transaction: 'After this transaction',
    after_all_transactions: 'After all transactions',
  }

  return stage ? labels[stage] || stage : 'Transaction event'
}
</script>

<template>
  <div
    v-for="event in items"
    :key="`${event.transaction_hash}:${event.event_index}`"
    class="activity-card"
  >
    <div class="section-heading">
      <strong
        >Event #{{ event.event_index }}
        <span class="muted small">
          /
          {{ event.event_scope === 'transaction' ? 'transaction' : `operation ${event.op_index}` }}
        </span>
      </strong>
      <span class="protocol">{{ event.event_type }}</span>
    </div>
    <p
      v-if="event.event_scope === 'transaction'"
      class="muted small"
    >
      {{ stageLabel(event.stage) }}
    </p>
    <p v-if="event.contract_id">
      <EntityLink
        kind="contracts"
        :value="event.contract_id"
      />
    </p>
    <p class="muted small">Transaction <EntityLink :value="event.transaction_hash" /></p>
    <JsonView
      :value="event.topics"
      label="Topics (typed SCVal)"
    />
    <JsonView
      :value="event.value"
      label="Value (typed SCVal)"
    />
    <JsonView
      :value="event.raw_xdr"
      label="Raw event XDR"
    />
  </div>
  <p
    v-if="!items.length"
    class="empty-inline"
  >
    No committed events in this indexed result.
  </p>
</template>
