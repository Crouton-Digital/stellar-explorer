<script setup lang="ts">
import type { Ledger } from '~/types'
defineProps<{ items: Ledger[] }>()
</script>

<template>
  <div class="table-scroll">
    <table
      class="responsive-table"
      aria-label="Indexed ledgers"
    >
      <thead>
        <tr>
          <th>Ledger</th>
          <th>Hash</th>
          <th class="numeric">Transactions</th>
          <th>Protocol</th>
          <th>Closed at</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="ledger in items"
          :key="ledger.sequence"
        >
          <td data-label="Ledger">
            <EntityLink
              kind="ledgers"
              :value="String(ledger.sequence)"
            />
          </td>
          <td
            class="mono muted"
            data-label="Hash"
          >
            {{ ledger.hash.slice(0, 12) }}…{{ ledger.hash.slice(-8) }}
          </td>
          <td
            class="numeric mono"
            data-label="Transactions"
          >
            {{ ledger.transaction_count }}
          </td>
          <td data-label="Protocol">
            <span class="protocol">v{{ ledger.protocol_version }}</span>
          </td>
          <td
            class="muted small"
            data-label="Closed at"
          >
            <TimeValue :value="ledger.closed_at" />
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
