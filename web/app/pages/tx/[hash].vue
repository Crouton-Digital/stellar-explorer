<script setup lang="ts">
import type { Transaction } from '~/types'
import { xlm } from '~/utils/format.mjs'

const route = useRoute()
const url = computed(() => `/api/v1/transactions/${encodeURIComponent(String(route.params.hash))}`)

const { data, pending, error, refresh } = await useLazyFetch<Transaction>(url)
</script>

<template>
  <div class="page-heading">
    <div>
      <div class="eyebrow"><NuxtLink to="/transactions">TRANSACTIONS</NuxtLink> / DETAIL</div>
      <h1>Transaction</h1>
      <p>Operations, contract calls, and committed events.</p>
    </div>
    <span
      v-if="data"
      class="pill"
      :class="data.successful ? 'success' : 'failed'"
    >
      {{ data.successful ? 'Success' : 'Failed' }}
    </span>
  </div>
  <StateView
    :pending="pending"
    :error="error"
    @retry="refresh"
  >
    <template v-if="data">
      <TransactionActions :interpretation="data.interpretation" />
      <section class="panel detail-panel">
        <dl>
          <dt>Transaction hash</dt>
          <dd><CopyValue :value="data.hash" /></dd>
          <dt>Ledger</dt>
          <dd>
            <EntityLink
              kind="ledgers"
              :value="String(data.ledger_sequence)"
            />
          </dd>
          <dt>Source account</dt>
          <dd>
            <EntityLink
              kind="accounts"
              :value="data.source_account"
              full
            />
          </dd>
          <dt>Fee account</dt>
          <dd>
            <EntityLink
              kind="accounts"
              :value="data.fee_account"
              full
            />
          </dd>
          <dt>Result fee charged</dt>
          <dd class="mono">
            {{ xlm(data.fee_charged) }} XLM
            <span class="muted">({{ data.fee_charged }} stroops)</span>
          </dd>
          <dt>Result</dt>
          <dd class="mono">{{ data.result_code }}</dd>
          <dt>Closed at</dt>
          <dd><TimeValue :value="data.created_at" /></dd>
        </dl>
      </section>
      <FeeBreakdown :fees="data.fees" />
      <TransactionState :changes="data.interpretation?.state_changes" />
      <section class="panel padded">
        <h2>
          Operations <span class="count">{{ data.operations.length }}</span>
        </h2>
        <div
          v-for="op in data.operations"
          :id="`operation-${op.op_index}`"
          :key="op.op_index"
          class="activity-card"
        >
          <div class="section-heading">
            <strong>#{{ op.op_index }} · {{ op.type.replace('OperationType', '') }}</strong>
            <span class="protocol">{{ op.decode_status }}</span>
          </div>
          <p class="muted small">
            Source
            <EntityLink
              kind="accounts"
              :value="op.source_account"
            />
          </p>
          <pre class="decoded">{{ JSON.stringify(op.details, null, 2) }}</pre>
          <JsonView
            :value="op.raw_xdr"
            label="Operation XDR"
          />
        </div>
      </section>
      <section class="panel padded">
        <h2>
          Soroban calls <span class="count">{{ data.calls.length }}</span>
        </h2>
        <p class="muted small">Top-level invocations. Internal call trees are outside this MVP.</p>
        <div
          v-for="call in data.calls"
          :key="call.op_index"
          class="activity-card"
        >
          <strong class="function-name">{{ call.function_name }}(…)</strong>
          <p>
            <EntityLink
              kind="contracts"
              :value="call.contract_id"
              full
            />
          </p>
          <JsonView
            :value="call.arguments"
            label="Typed arguments"
          />
        </div>
        <p
          v-if="!data.calls.length"
          class="empty-inline"
        >
          No decoded top-level contract invocation.
        </p>
        <JsonView
          :value="data.return_value"
          label="Return value (when provided)"
        />
      </section>
      <section class="panel padded">
        <h2>
          Events <span class="count">{{ data.events.length }}</span>
        </h2>
        <EventList :items="data.events" />
      </section>
      <section class="panel">
        <div class="panel-heading">
          <h2>
            Token movements <span class="count">{{ data.token_transfers?.length || 0 }}</span>
          </h2>
        </div>
        <TransferTable
          v-if="data.token_transfers?.length"
          :items="data.token_transfers"
          :show-transaction="false"
        />
        <p
          v-else
          class="empty-inline padded"
        >
          No decoded token movements in this transaction.
        </p>
      </section>
      <section class="panel">
        <div class="panel-heading">
          <h2>
            NFT movements <span class="count">{{ data.nft_transfers?.length || 0 }}</span>
          </h2>
        </div>
        <p class="token-note muted small">
          Event format matches with exact token IDs. These records do not establish current
          ownership or collection compliance. Burn records use the burnable extension.
        </p>
        <NFTTransferTable
          v-if="data.nft_transfers?.length"
          :items="data.nft_transfers"
          :show-transaction="false"
        />
        <p
          v-else
          class="empty-inline padded"
        >
          No decoded NFT movements in this transaction.
        </p>
      </section>
      <section class="panel padded">
        <h2>Raw transaction</h2>
        <JsonView
          :value="data.envelope_xdr"
          label="Envelope XDR"
        />
        <JsonView
          :value="data.result_xdr"
          label="Result XDR"
        />
        <JsonView
          :value="data.meta_xdr"
          label="Transaction metadata XDR"
        />
        <JsonView
          :value="data.diagnostics"
          label="Diagnostics (provider dependent)"
        />
      </section>
    </template>
  </StateView>
</template>
