<script setup lang="ts">
import type { TokenTransfer } from '~/types'
import { short } from '~/utils/format.mjs'

withDefaults(defineProps<{ items: TokenTransfer[]; showTransaction?: boolean }>(), {
  showTransaction: true,
})
</script>

<template>
  <div class="table-scroll">
    <table
      class="responsive-table"
      aria-label="Token movements"
    >
      <thead>
        <tr>
          <th>Action</th>
          <th>Asset</th>
          <th>From</th>
          <th>To</th>
          <th class="numeric">Amount</th>
          <th v-if="showTransaction">Transaction</th>
          <th>Ledger</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="item in items"
          :key="`${item.transaction_hash}:${item.event_index}`"
        >
          <td data-label="Action">
            <span class="protocol">{{ item.kind }}</span>
          </td>
          <td data-label="Asset">
            <div>
              <NuxtLink
                class="entity mono"
                :to="`/assets/${item.asset.contract_id}`"
                :title="item.asset.contract_id"
              >
                {{ item.asset.code || short(item.asset.contract_id) }}
              </NuxtLink>
              <div class="muted small">
                {{ item.asset.kind === 'sac' ? 'Verified SAC' : 'Event format match' }}
              </div>
            </div>
          </td>
          <td data-label="From">
            <TokenAddress
              :address="item.from"
              empty="Mint"
            />
          </td>
          <td data-label="To">
            <TokenAddress
              :address="item.to"
              empty="Removed"
            />
          </td>
          <td
            class="numeric"
            data-label="Amount"
          >
            <TokenAmount
              :amount="item.amount"
              :asset="item.asset"
            />
          </td>
          <td
            v-if="showTransaction"
            data-label="Transaction"
          >
            <EntityLink :value="item.transaction_hash" />
          </td>
          <td data-label="Ledger">
            <EntityLink
              kind="ledgers"
              :value="String(item.ledger_sequence)"
            />
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
