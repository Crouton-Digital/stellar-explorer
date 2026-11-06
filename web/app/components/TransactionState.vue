<script setup lang="ts">
import type { StateChange, StateSnapshot } from '~/types'

const props = defineProps<{ changes?: StateChange[] }>()
const currentPage = ref(0)
const pageSize = 10
const pageCount = computed(() => Math.ceil((props.changes?.length || 0) / pageSize))
const visible = computed(() =>
  props.changes?.slice(currentPage.value * pageSize, (currentPage.value + 1) * pageSize),
)
const panel = ref<HTMLElement>()

watch(
  () => props.changes,
  () => {
    currentPage.value = 0
  },
)

function navigate(page: number) {
  currentPage.value = page
  panel.value?.scrollIntoView({ block: 'start' })
}

function fields(change: StateChange) {
  return [
    ...new Set([
      ...Object.keys(change.before?.fields || {}),
      ...Object.keys(change.after?.fields || {}),
    ]),
  ]
}

function value(snapshot: StateSnapshot | null, field: string) {
  if (!snapshot || !(field in snapshot.fields)) {
    return '—'
  }

  const content = snapshot.fields[field]
  if (content !== null && typeof content === 'object' && 'type' in content && 'value' in content) {
    const typed = content as { type: string; value: unknown }
    const text =
      typeof typed.value === 'string' ? typed.value : JSON.stringify(typed.value, null, 2)
    return `${typed.type.replace('ScValTypeScv', '')}: ${text}`
  }

  return typeof content === 'string' ? content || '(empty)' : JSON.stringify(content, null, 2)
}

function statusLabel(status: StateChange['before_status']) {
  return { present: 'Present', absent: 'Absent', archived: 'Archived', unknown: 'Unknown' }[status]
}
</script>

<template>
  <section
    ref="panel"
    class="panel padded transaction-state"
    aria-label="State changes"
  >
    <h2>
      State changes <span class="count">{{ changes?.length || 0 }}</span>
    </h2>
    <p class="muted small">
      Committed writes from this transaction's metadata. Before and after values apply to each
      metadata group; they do not describe current balances. Fee processing outside transaction
      metadata is shown separately when available.
    </p>
    <article
      v-for="change in visible"
      :key="change.id"
      class="activity-card state-change"
      :aria-label="`${change.entry_type} ${change.kind}`"
    >
      <div class="section-heading">
        <h3>{{ change.entry_type.replace(/([a-z])([A-Z])/g, '$1 $2') }}</h3>
        <span class="protocol">{{ change.kind }}</span>
      </div>
      <p class="state-subject mono">
        <TokenAddress
          v-if="change.subject.startsWith('G') || change.subject.startsWith('C')"
          :address="change.subject"
          empty="Unknown"
        />
        <span v-else>{{ change.subject }}</span>
      </p>
      <p class="muted small">
        <a
          v-if="change.op_index !== null"
          :href="`#operation-${change.op_index}`"
          >Operation {{ change.op_index + 1 }}</a
        >
        <span v-else>{{
          change.scope === 'transaction_before' ? 'Before operations' : 'After operations'
        }}</span>
        · Metadata change #{{ change.change_index }}
        <span v-if="change.before_index !== null"> · Baseline #{{ change.before_index }}</span>
      </p>
      <div class="state-comparison">
        <div class="state-comparison-heading">
          <strong>Before</strong>
          <span class="muted small">{{ statusLabel(change.before_status) }}</span>
          <span
            v-if="change.before"
            class="muted small"
            >Ledger {{ change.before.last_modified_ledger }}</span
          >
        </div>
        <div class="state-comparison-heading">
          <strong>After</strong>
          <span class="muted small">{{ statusLabel(change.after_status) }}</span>
          <span
            v-if="change.after"
            class="muted small"
            >Ledger {{ change.after.last_modified_ledger }}</span
          >
        </div>
        <template
          v-for="field in fields(change)"
          :key="field"
        >
          <div class="state-field">
            <span class="muted small">{{ field.replaceAll('_', ' ') }} · Before</span>
            <pre>{{ value(change.before, field) }}</pre>
          </div>
          <div class="state-field">
            <span class="muted small">{{ field.replaceAll('_', ' ') }} · After</span>
            <pre>{{ value(change.after, field) }}</pre>
          </div>
        </template>
      </div>
      <p
        v-if="change.before_status === 'unknown'"
        class="muted small"
      >
        A matching before value was not provided in this metadata group.
      </p>
      <p class="muted small">
        Selected fields are shown. Full entries, extensions and unsupported fields remain in the
        transaction metadata XDR.
      </p>
      <p
        v-if="!fields(change).length"
        class="muted small"
      >
        Readable fields are unavailable for this entry type. Inspect the transaction metadata XDR.
      </p>
      <JsonView
        v-if="change.key !== null"
        :value="change.key"
        label="Entry key"
      />
      <JsonView
        :value="change.key_xdr"
        label="Ledger key XDR"
      />
    </article>
    <p
      v-if="changes === undefined"
      class="empty-inline"
    >
      No state interpretation is available for this stored transaction.
    </p>
    <p
      v-else-if="!changes.length"
      class="empty-inline"
    >
      No committed state writes were provided in this transaction's metadata.
    </p>
    <nav
      v-if="pageCount > 1"
      class="page-nav"
      aria-label="State changes pages"
    >
      <button
        :disabled="currentPage === 0"
        @click="navigate(currentPage - 1)"
      >
        Previous
      </button>
      <span class="muted small">Page {{ currentPage + 1 }} of {{ pageCount }}</span>
      <button
        :disabled="currentPage >= pageCount - 1"
        @click="navigate(currentPage + 1)"
      >
        Next
      </button>
    </nav>
  </section>
</template>
