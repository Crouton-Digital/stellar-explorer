<script setup lang="ts">
import type { NFTItemDetail } from '~/types'

const route = useRoute()
const address = computed(() => String(route.params.address))
const tokenID = computed(() => String(route.params.tokenId))
const { data, pending, error, refresh } = await useLazyFetch<NFTItemDetail>(
  computed(
    () =>
      `/api/v1/nfts/collections/${encodeURIComponent(address.value)}/tokens/${encodeURIComponent(tokenID.value)}`,
  ),
  { query: computed(() => route.query) },
)

useHead({ title: 'NFT item activity · Crouton' })
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow"><NuxtLink :to="`/nfts/${address}`">NFT COLLECTION</NuxtLink> / ITEM</div>
      <h1>NFT item activity</h1>
      <p>A token identifier and its observed movement history.</p>
    </div>
  </div>
  <StateView
    :pending="pending"
    :error="error"
    @retry="refresh"
  >
    <template v-if="data">
      <section class="panel detail-panel">
        <dl>
          <dt>Token ID</dt>
          <dd><CopyValue :value="data.item.token_id" /></dd>
          <dt>Collection</dt>
          <dd>
            <EntityLink
              kind="nfts"
              :value="data.item.contract_id"
            />
          </dd>
          <dt>Identification</dt>
          <dd>Supported NFT event format match</dd>
          <dt>Recorded movements</dt>
          <dd>{{ data.item.transfer_count.toLocaleString('en-US') }}</dd>
          <dt>Count snapshot</dt>
          <dd>
            <EntityLink
              kind="ledgers"
              :value="String(data.snapshot_ledger)"
            />
          </dd>
          <dt>First observed</dt>
          <dd>
            <EntityLink
              kind="ledgers"
              :value="String(data.item.first_observed_ledger)"
            />
          </dd>
          <dt>Current owner</dt>
          <dd>Unknown · indexed events do not establish current ownership</dd>
        </dl>
      </section>
      <div class="notice">
        Counts cover this item through snapshot ledger {{ data.snapshot_ledger }}. Movement filters
        apply only to the history below. Recipients are event evidence; they do not establish the
        current owner, metadata or complete lifecycle of this item.
      </div>
      <section class="panel">
        <div class="panel-heading"><h2>NFT movements</h2></div>
        <NFTTransferTable
          v-if="data.transfers.items.length"
          :items="data.transfers.items"
          :show-collection="false"
        />
        <p
          v-else
          class="empty-inline padded"
        >
          No NFT movements on this page.
        </p>
        <PageNav :cursor="data.transfers.next_cursor" />
      </section>
    </template>
  </StateView>
</template>
