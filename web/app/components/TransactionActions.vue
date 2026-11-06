<script setup lang="ts">
import type { Action, Interpretation } from '~/types'
import { tokenAmount } from '~/utils/format.mjs'

defineProps<{ interpretation?: Interpretation }>()

function statusLabel(status: Action['status']) {
  return { applied: 'Applied', failed: 'Failed', not_applied: 'Not applied' }[status]
}

function actionAmount(action: Action) {
  if (action.amount === null) {
    return '—'
  }

  const negative = action.amount.startsWith('-')
  const quantity = negative ? action.amount.slice(1) : action.amount
  const formatted = tokenAmount(quantity, action.asset?.decimals ?? null)
  return `${negative ? '-' : ''}${formatted}`
}
</script>

<template>
  <section
    class="panel padded"
    aria-label="Transaction actions"
  >
    <h2>What happened</h2>
    <template v-if="interpretation?.version">
      <p class="muted">{{ interpretation.summary }}</p>
      <div
        v-if="interpretation.failure"
        class="notice error execution-failure"
        aria-label="Execution failure"
      >
        <h3>Why it failed</h3>
        <p>{{ interpretation.failure.message }}</p>
        <ul v-if="interpretation.failure.operations.length > 1">
          <li
            v-for="failure in interpretation.failure.operations"
            :key="failure.op_index"
          >
            Operation {{ failure.op_index + 1 }}: {{ failure.message }}
          </li>
        </ul>
        <JsonView
          :value="interpretation.failure"
          label="Failure codes and operation results"
        />
      </div>
      <article
        v-for="action in interpretation.actions"
        :key="action.id"
        class="activity-card interpreted-action"
        :aria-label="action.title"
      >
        <div class="section-heading">
          <h3>{{ action.title }}</h3>
          <span
            class="pill"
            :class="{ success: action.status === 'applied', failed: action.status === 'failed' }"
          >
            {{ statusLabel(action.status) }}
          </span>
        </div>
        <p class="muted small">{{ action.description }}</p>
        <PathPaymentDetails
          v-if="action.path_payment"
          :payment="action.path_payment"
        />
        <ContractSwapDetails
          v-if="action.contract_swap"
          :swap="action.contract_swap"
        />
        <AquariusSwapDetails
          v-if="action.aquarius_swap"
          :swap="action.aquarius_swap"
        />
        <dl class="action-fields">
          <template v-if="action.amount !== null">
            <dt>{{ action.status === 'applied' ? 'Amount' : 'Requested amount' }}</dt>
            <dd class="mono">
              {{ actionAmount(action) }}
              <span class="muted small">
                {{ action.asset?.decimals == null ? 'base units' : action.asset.code || 'tokens' }}
              </span>
            </dd>
          </template>
          <template v-if="action.source_account">
            <dt>Operation source</dt>
            <dd>
              <TokenAddress
                :address="action.source_account"
                empty="Unknown"
              />
            </dd>
          </template>
          <template v-if="action.from && !action.contract_swap && !action.aquarius_swap">
            <dt>{{ action.source === 'operation' ? 'Source' : 'From' }}</dt>
            <dd>
              <TokenAddress
                :address="action.from"
                empty="Unknown"
              />
            </dd>
          </template>
          <template v-if="action.to && !action.contract_swap && !action.aquarius_swap">
            <dt>To</dt>
            <dd>
              <TokenAddress
                :address="action.to"
                empty="Unknown"
              />
            </dd>
          </template>
          <template v-if="action.contract_id">
            <dt>Contract</dt>
            <dd>
              <EntityLink
                kind="contracts"
                :value="action.contract_id"
              />
            </dd>
          </template>
        </dl>
        <p class="muted small action-provenance">
          <a :href="`#operation-${action.op_index}`">Operation {{ action.op_index + 1 }}</a>
          <span v-if="action.source === 'token_event'">
            · Committed event #{{ action.event_indexes[0] }}
          </span>
        </p>
        <JsonView
          v-if="action.to_muxed"
          :value="action.to_muxed"
          label="Destination routing information"
        />
      </article>
      <p
        v-if="!interpretation.actions.length"
        class="empty-inline"
      >
        No supported actions were found in this transaction.
      </p>
      <p class="muted small">
        Descriptions cover supported operations and committed token movements. Full contract effects
        may extend beyond the actions shown.
      </p>
      <details
        v-if="interpretation.diagnostic_errors.length || interpretation.diagnostics_truncated"
      >
        <summary>Diagnostic evidence ({{ interpretation.diagnostic_errors.length }})</summary>
        <p class="muted small">
          These traces can include handled errors. Transaction and operation result codes determine
          the execution outcome.
        </p>
        <ul class="diagnostic-evidence">
          <li
            v-for="(diagnostic, index) in interpretation.diagnostic_errors"
            :key="index"
          >
            {{ diagnostic.message }}
            <span class="muted small">· Diagnostic event #{{ diagnostic.event_index }}</span>
          </li>
        </ul>
        <p
          v-if="interpretation.diagnostics_truncated"
          class="muted small"
        >
          Additional diagnostic data remains available in the raw transaction view.
        </p>
      </details>
    </template>
    <p
      v-else
      class="muted small"
    >
      No plain-language interpretation is available for this stored transaction.
    </p>
  </section>
</template>
