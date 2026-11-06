<script setup lang="ts">
import type { Transaction } from '~/types'
import { xlm } from '~/utils/format.mjs'
defineProps<{ items: Transaction[] }>()
</script>

<template>
  <div class="table-scroll">
    <table
      class="responsive-table"
      aria-label="Indexed transactions"
    >
      <thead>
        <tr>
          <th>Transaction</th>
          <th>Status</th>
          <th>Ledger</th>
          <th>Source account</th>
          <th class="numeric">Fee · XLM</th>
          <th>Closed at</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="tx in items"
          :key="tx.hash"
        >
          <td data-label="Transaction"><EntityLink :value="tx.hash" /></td>
          <td data-label="Status">
            <span
              class="pill"
              :class="tx.successful ? 'success' : 'failed'"
            >
              {{ tx.successful ? 'Success' : 'Failed' }}
            </span>
          </td>
          <td data-label="Ledger">
            <EntityLink
              kind="ledgers"
              :value="String(tx.ledger_sequence)"
            />
          </td>
          <td data-label="Source account">
            <EntityLink
              kind="accounts"
              :value="tx.source_account"
            />
          </td>
          <td
            class="numeric mono"
            data-label="Fee · XLM"
          >
            {{ xlm(tx.fee_charged) }}
          </td>
          <td
            class="muted small"
            data-label="Closed at"
          >
            <TimeValue :value="tx.created_at" />
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
