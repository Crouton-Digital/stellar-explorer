<script setup lang="ts">
import type { FeeSummary } from '~/types'
import { xlm } from '~/utils/format.mjs'

defineProps<{ fees?: FeeSummary }>()
</script>

<template>
  <section
    class="panel padded"
    aria-label="Fees and refunds"
  >
    <h2>Fees and refunds</h2>
    <template v-if="fees?.status === 'complete'">
      <dl class="fee-breakdown">
        <div>
          <dt>Initial charge</dt>
          <dd class="mono">{{ xlm(fees.charged!) }} XLM</dd>
          <dd class="muted small">{{ fees.charged }} stroops</dd>
        </div>
        <div>
          <dt>Refund</dt>
          <dd class="mono">{{ xlm(fees.refunded!) }} XLM</dd>
          <dd class="muted small">{{ fees.refunded }} stroops</dd>
        </div>
        <div>
          <dt>Net fee</dt>
          <dd class="mono">{{ xlm(fees.net!) }} XLM</dd>
          <dd class="muted small">{{ fees.net }} stroops</dd>
        </div>
      </dl>
      <p class="muted small">
        Calculated from committed fee events and reconciled with the result.
      </p>
    </template>
    <p
      v-else-if="fees?.status === 'invalid'"
      class="notice"
      role="status"
    >
      Fee events could not be reconciled. The initial charge, refund, and net fee are unverified.
      Inspect the raw events below for details.
    </p>
    <p
      v-else
      class="muted small"
    >
      Fee/refund events were not provided. The transaction result fee is shown above; the initial
      charge and refund are unknown.
    </p>
  </section>
</template>
