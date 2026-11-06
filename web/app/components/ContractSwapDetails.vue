<script setup lang="ts">
import type { ContractSwap } from '~/types'

defineProps<{ swap: ContractSwap }>()
</script>

<template>
  <section
    class="contract-swap"
    aria-label="Soroswap execution"
  >
    <dl class="action-fields">
      <dt>Protocol</dt>
      <dd>Soroswap · {{ swap.mode === 'exact_input' ? 'Exact input' : 'Exact output' }}</dd>
      <dt>Amount sent</dt>
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
      <dt>{{ swap.mode === 'exact_input' ? 'Requested input' : 'Maximum input' }}</dt>
      <dd>
        <TokenAmount
          :amount="swap.input_limit"
          :asset="swap.input.asset"
        />
      </dd>
      <dt>{{ swap.mode === 'exact_input' ? 'Minimum output' : 'Requested output' }}</dt>
      <dd>
        <TokenAmount
          :amount="swap.output_limit"
          :asset="swap.output.asset"
        />
      </dd>
      <dt>Trader</dt>
      <dd>
        <TokenAddress
          :address="swap.trader"
          empty="Unknown"
        />
      </dd>
      <dt>Deadline</dt>
      <dd class="mono">{{ swap.deadline }} <span class="muted small">Unix seconds</span></dd>
    </dl>
    <p class="muted small">
      The trader owns the exchanged tokens and can differ from the operation source. Amounts exclude
      transaction fees. Token movement actions below retain the same transfer evidence.
    </p>
    <details>
      <summary>
        Swap route ({{ swap.hops.length }} {{ swap.hops.length === 1 ? 'pool' : 'pools' }})
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
          <p class="small">
            Pool {{ hop.index + 1 }} ·
            <EntityLink
              kind="contracts"
              :value="hop.pool_contract"
            />
          </p>
          <p class="muted small">
            Committed input event #{{ hop.input_event_index }} · Output event #{{
              hop.output_event_index
            }}
          </p>
        </li>
      </ol>
    </details>
    <details>
      <summary>Router and evidence</summary>
      <p class="muted small">
        Matched the registered Testnet router, request limits, return amounts, execution result
        commitment, pool route and committed token transfers. This does not certify arbitrary
        contracts or their current code.
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
        <dt>Router event</dt>
        <dd>#{{ swap.router_event_index }}</dd>
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
        The router WASM hash was not observed in this transaction. Router registration and transfer
        evidence do not establish historical code identity.
      </p>
    </details>
    <JsonView
      :value="swap"
      label="Swap assets, route and source evidence"
    />
  </section>
</template>
