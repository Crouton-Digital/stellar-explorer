<script setup lang="ts">
import type { NFTTransfer } from '~/types'

withDefaults(
  defineProps<{
    items: NFTTransfer[]
    showTransaction?: boolean
    showCollection?: boolean
  }>(),
  { showTransaction: true, showCollection: true },
)
</script>

<template>
  <div class="table-scroll">
    <table
      class="responsive-table"
      aria-label="NFT movements"
    >
      <thead>
        <tr>
          <th>Action</th>
          <th>Token ID</th>
          <th v-if="showCollection">Collection</th>
          <th>From</th>
          <th>To</th>
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
            <div>
              <span class="protocol">{{ item.kind }}</span>
              <div class="muted small">Event #{{ item.event_index }}</div>
              <div class="muted small">
                {{
                  item.event_standard === 'openzeppelin_burnable'
                    ? 'Burnable extension'
                    : 'SEP-50 format'
                }}
              </div>
            </div>
          </td>
          <td data-label="Token ID">
            <div>
              <NuxtLink
                class="entity mono nft-token-id"
                :to="`/nfts/${item.contract_id}/${item.token_id}`"
              >
                {{ item.token_id }}
              </NuxtLink>
              <div class="muted small">
                {{ item.token_id_type }} · {{ item.format.replaceAll('_', ' ') }}
              </div>
            </div>
          </td>
          <td
            v-if="showCollection"
            data-label="Collection"
          >
            <EntityLink
              kind="nfts"
              :value="item.contract_id"
            />
          </td>
          <td data-label="From">
            <NFTAddress
              :address="item.from"
              direction="from"
              :empty="item.kind === 'mint' ? 'Mint' : 'Unknown'"
            />
          </td>
          <td data-label="To">
            <NFTAddress
              :address="item.to"
              direction="to"
              :empty="item.kind === 'burn' ? 'Burned' : 'Unknown'"
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
