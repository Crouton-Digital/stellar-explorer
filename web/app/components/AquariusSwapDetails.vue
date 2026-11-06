<script setup lang="ts">
import type { AquariusSwap } from '~/types'

defineProps<{ swap: AquariusSwap }>()

function routeWeight(weight: number) {
  return `${Math.floor(weight / 100)}.${String(weight % 100).padStart(2, '0')}%`
}

function routingLabel(routing: AquariusSwap['routing']) {
  return {
    direct: 'Direct pool',
    chained: 'Chained pools through router',
    split: 'Split routes through router',
  }[routing]
}
</script>

<template>
  <section
    class="contract-swap"
    aria-label="Aquarius execution"
  >
    <dl class="action-fields">
      <dt>Protocol</dt>
      <dd>Aquarius · {{ swap.mode === 'exact_output' ? 'Exact output' : 'Exact input' }}</dd>
      <dt>Routing</dt>
      <dd>{{ routingLabel(swap.routing) }}</dd>
      <dt>{{ swap.mode === 'exact_output' ? 'Amount spent' : 'Amount sent' }}</dt>
      <dd>
        <TokenAmount
          :amount="swap.input.amount"
          :asset="swap.input.asset"
        />
      </dd>
      <dt>Amount received</dt>
      <dd>
        <TokenAmount
          :amount="swap.output.amount"
          :asset="swap.output.asset"
        />
      </dd>
      <dt>{{ swap.mode === 'exact_output' ? 'Maximum input' : 'Requested input' }}</dt>
      <dd>
        <TokenAmount
          :amount="swap.input_limit"
          :asset="swap.input.asset"
        />
      </dd>
      <dt>{{ swap.mode === 'exact_output' ? 'Requested output' : 'Minimum output' }}</dt>
      <dd>
        <TokenAmount
          :amount="swap.output_limit"
          :asset="swap.output.asset"
        />
      </dd>
      <template v-if="swap.settlement">
        <dt>Funded input</dt>
        <dd>
          <TokenAmount
            :amount="swap.settlement.funded_amount"
            :asset="swap.input.asset"
          />
        </dd>
        <dt>Returned input</dt>
        <dd>
          <TokenAmount
            :amount="swap.settlement.refunded_amount"
            :asset="swap.input.asset"
          />
        </dd>
      </template>
      <dt>Trader</dt>
      <dd>
        <TokenAddress
          :address="swap.trader"
          empty="Unknown"
        />
      </dd>
    </dl>
    <p class="muted small">
      The trader owns the exchanged tokens and can differ from the operation source. Amounts exclude
      transaction fees. Token movement actions retain the same transfer evidence.
    </p>
    <p
      v-if="swap.mode === 'exact_output'"
      class="muted small"
    >
      The amount spent equals funded input minus returned input. This token return is separate from
      transaction fee refunds.
    </p>
    <p
      v-if="swap.routing !== 'direct'"
      class="muted small"
    >
      Router funding and settlement transfers are included once in the evidence. Intermediate hop
      amounts are not added to the total sent or received.
    </p>
    <details v-if="swap.settlement">
      <summary>Router settlement evidence</summary>
      <dl class="action-fields">
        <dt>Funding event</dt>
        <dd>#{{ swap.settlement.funding_event_index }}</dd>
        <dt>Input return event</dt>
        <dd>
          {{
            swap.settlement.refund_event_index === null
              ? 'No input return event'
              : `#${swap.settlement.refund_event_index}`
          }}
        </dd>
        <dt>Payout event</dt>
        <dd>#{{ swap.settlement.payout_event_index }}</dd>
      </dl>
    </details>
    <details v-if="swap.routes?.length">
      <summary>Split routes ({{ swap.routes.length }})</summary>
      <p class="muted small">
        The total sent and received sum each branch's endpoints once. Branch paths remain separate;
        the pool list below preserves the shared source ordering.
      </p>
      <ol class="trade-route aquarius-split-routes">
        <li
          v-for="route in swap.routes"
          :key="route.index"
        >
          <p class="small">
            <strong>Route {{ route.index + 1 }}</strong> · {{ routeWeight(route.weight_bps) }}
          </p>
          <div class="trade-amounts">
            <TokenAmount
              :amount="route.input.amount"
              :asset="route.input.asset"
            />
            <span aria-label="exchanged for">→</span>
            <TokenAmount
              :amount="route.output.amount"
              :asset="route.output.asset"
            />
          </div>
          <div class="aquarius-pool-tokens">
            <EntityLink
              v-for="token in route.path"
              :key="token"
              kind="assets"
              :value="token"
            />
          </div>
          <p class="muted small">
            Pool evidence {{ route.hop_indexes.map((index) => `#${index + 1}`).join(', ') }}
          </p>
        </li>
      </ol>
    </details>
    <details>
      <summary>
        {{ swap.routing === 'split' ? 'Pool evidence' : 'Aquarius route' }} ({{ swap.hops.length }}
        {{ swap.hops.length === 1 ? 'pool' : 'pools' }})
      </summary>
      <ol class="trade-route">
        <li
          v-for="hop in swap.hops"
          :key="hop.index"
        >
          <div class="trade-amounts">
            <TokenAmount
              :amount="hop.input.amount"
              :asset="hop.input.asset"
            />
            <span aria-label="exchanged for">→</span>
            <TokenAmount
              :amount="hop.output.amount"
              :asset="hop.output.asset"
            />
          </div>
          <dl class="action-fields">
            <dt>Pool {{ hop.index + 1 }}</dt>
            <dd>
              <EntityLink
                kind="contracts"
                :value="hop.pool_contract"
              />
            </dd>
            <dt>Pool index</dt>
            <dd class="mono contract-swap-hash">{{ hop.pool_index }}</dd>
            <dt>Pool tokens</dt>
            <dd>
              <div class="aquarius-pool-tokens">
                <EntityLink
                  v-for="token in hop.pool_tokens"
                  :key="token"
                  kind="assets"
                  :value="token"
                />
              </div>
            </dd>
            <dt>Input event</dt>
            <dd>#{{ hop.input_event_index }}</dd>
            <dt>Output event</dt>
            <dd>#{{ hop.output_event_index }}</dd>
            <dt>Pool trade event</dt>
            <dd>#{{ hop.trade_event_index }}</dd>
            <dt>Router event</dt>
            <dd>#{{ hop.router_event_index }}</dd>
          </dl>
        </li>
      </ol>
    </details>
    <details>
      <summary>Aquarius router and evidence</summary>
      <p class="muted small">
        Matched the registered Testnet router, request limits, return amount, execution result
        commitment, pool trade events, router events and token transfers. Pool addresses come from
        this matching evidence; they are not derived or independently verified against a pool
        registry.
      </p>
      <dl class="action-fields">
        <dt>Router</dt>
        <dd>
          <EntityLink
            kind="contracts"
            :value="swap.router_contract"
          />
        </dd>
        <dt>Adapter version</dt>
        <dd>{{ swap.adapter_version }}</dd>
        <dt>Transfer events</dt>
        <dd>{{ swap.transfer_event_indexes.map((index) => `#${index}`).join(', ') }}</dd>
        <template v-if="swap.router_wasm_hash">
          <dt>Observed WASM hash</dt>
          <dd class="mono contract-swap-hash">{{ swap.router_wasm_hash }}</dd>
        </template>
      </dl>
      <p
        v-if="!swap.router_wasm_hash"
        class="muted small"
      >
        The router WASM hash was not observed in this transaction.
      </p>
      <p class="muted small">
        Executable observations do not establish a match to pinned or audited code. These checks do
        not certify arbitrary token contracts or their balance semantics.
      </p>
    </details>
    <JsonView
      :value="swap"
      label="Aquarius assets, pools and source evidence"
    />
  </section>
</template>
