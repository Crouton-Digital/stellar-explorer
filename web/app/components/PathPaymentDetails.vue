<script setup lang="ts">
import type { PathPayment } from '~/types'

defineProps<{ payment: PathPayment }>()
</script>

<template>
  <div class="path-payment">
    <dl class="action-fields">
      <template v-if="payment.execution">
        <dt>Amount sent</dt>
        <dd>
          <TokenAmount
            :amount="payment.execution.input.amount"
            :asset="payment.execution.input.asset"
          />
        </dd>
        <dt>Amount received</dt>
        <dd>
          <TokenAmount
            :amount="payment.execution.output.amount"
            :asset="payment.execution.output.asset"
          />
        </dd>
      </template>
      <dt>{{ payment.mode === 'strict_send' ? 'Requested send' : 'Maximum send' }}</dt>
      <dd>
        <TokenAmount
          :amount="payment.send_limit"
          :asset="payment.send_asset"
        />
      </dd>
      <dt>{{ payment.mode === 'strict_receive' ? 'Requested receive' : 'Minimum receive' }}</dt>
      <dd>
        <TokenAmount
          :amount="payment.destination_limit"
          :asset="payment.destination_asset"
        />
      </dd>
    </dl>
    <p
      v-if="!payment.execution"
      class="muted small"
    >
      No reconciled execution is available. These amounts are request limits.
    </p>
    <details v-if="payment.execution?.trades.length">
      <summary>Executed trades ({{ payment.execution.trades.length }})</summary>
      <ol class="trade-route">
        <li
          v-for="trade in payment.execution.trades"
          :key="trade.claim_index"
        >
          <div class="trade-amounts">
            <TokenAmount
              :amount="trade.input.amount"
              :asset="trade.input.asset"
            />
            <span aria-label="exchanged for">→</span>
            <TokenAmount
              :amount="trade.output.amount"
              :asset="trade.output.asset"
            />
          </div>
          <p class="muted small">
            {{ trade.venue === 'liquidity_pool' ? 'Liquidity pool' : 'Order book' }} · Result claim
            #{{ trade.claim_index }}
          </p>
          <JsonView
            :value="trade"
            label="Trade amounts and asset identities"
          />
        </li>
      </ol>
    </details>
    <JsonView
      :value="payment"
      label="Path payment assets, limits and execution"
    />
  </div>
</template>
