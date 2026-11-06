<script setup lang="ts">
import type { NFTCollectionDetail } from '~/types'

const route = useRoute()
const address = computed(() => String(route.params.address))
const { data, pending, error, refresh } = await useLazyFetch<NFTCollectionDetail>(
  computed(() => `/api/v1/nfts/collections/${encodeURIComponent(address.value)}`),
  { query: computed(() => route.query) },
)

useHead({ title: 'NFT collection activity · Crouton' })
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow"><NuxtLink to="/nfts">NFTS</NuxtLink> / COLLECTION</div>
      <h1>NFT collection activity</h1>
      <p>Observed token IDs and committed NFT movement evidence.</p>
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
          <dt>Collection contract</dt>
          <dd><CopyValue :value="data.collection.contract_id" /></dd>
          <dt>Identification</dt>
          <dd>Supported NFT event format match</dd>
          <dt>Observed token IDs</dt>
          <dd>{{ data.collection.observed_token_count.toLocaleString('en-US') }}</dd>
          <dt>Recorded movements</dt>
          <dd>{{ data.collection.transfer_count.toLocaleString('en-US') }}</dd>
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
              :value="String(data.collection.first_observed_ledger)"
            />
          </dd>
          <dt>Contract evidence</dt>
          <dd>
            <EntityLink
              kind="contracts"
              :value="data.collection.contract_id"
            />
          </dd>
        </dl>
      </section>
      <div class="notice">
        Counts cover this entire collection through snapshot ledger {{ data.snapshot_ledger }}.
        Movement filters apply only to the history below. Observed IDs and movements are not current
        ownership or total supply. Collection names, artwork and metadata are not inferred or
        fetched.
      </div>
      <section class="panel">
        <div class="panel-heading">
          <h2>NFT movements</h2>
          <NuxtLink :to="{ path: '/nfts/transfers', query: { collection: address } }">
            Filter NFT movements →
          </NuxtLink>
        </div>
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
