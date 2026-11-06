<script setup lang="ts">
type FilterField = {
  key: string
  label: string
  placeholder?: string
  options?: { value: string; label: string }[]
}

const props = defineProps<{ kind: 'transactions' | 'events' | 'transfers' | 'nfts' }>()
const route = useRoute()
const draft = reactive<Record<string, string | number>>({})
const formError = ref('')
const fields = computed<FilterField[]>(() => {
  const bounds = [
    { key: 'min_ledger', label: 'From ledger', placeholder: 'Inclusive minimum' },
    { key: 'max_ledger', label: 'To ledger', placeholder: 'Inclusive maximum' },
  ]

  if (props.kind === 'transactions') {
    return [
      { key: 'address', label: 'Participant address', placeholder: 'G… or C…' },
      { key: 'contract', label: 'Contract', placeholder: 'C…' },
      { key: 'function', label: 'Function', placeholder: 'Exact function name' },
      {
        key: 'status',
        label: 'Status',
        options: [
          { value: '', label: 'All results' },
          { value: 'success', label: 'Success' },
          { value: 'failed', label: 'Failed' },
        ],
      },
      ...bounds,
    ]
  }

  if (props.kind === 'events') {
    return [
      { key: 'contract', label: 'Contract', placeholder: 'C…' },
      { key: 'event_name', label: 'Event name', placeholder: 'First symbol topic, e.g. transfer' },
      {
        key: 'event_type',
        label: 'Event type',
        options: [
          { value: '', label: 'All committed event types' },
          { value: 'contract', label: 'Contract' },
          { value: 'system', label: 'System' },
          { value: 'diagnostic', label: 'Diagnostic' },
        ],
      },
      {
        key: 'event_scope',
        label: 'Event source',
        options: [
          { value: '', label: 'Operations and transactions' },
          { value: 'operation', label: 'Operation events' },
          { value: 'transaction', label: 'Transaction events' },
        ],
      },
      ...bounds,
    ]
  }

  if (props.kind === 'nfts') {
    return [
      { key: 'collection', label: 'Collection contract', placeholder: 'NFT contract C…' },
      { key: 'token_id', label: 'Token ID', placeholder: 'Exact unsigned decimal ID' },
      {
        key: 'kind',
        label: 'NFT action',
        options: [
          { value: '', label: 'All NFT movements' },
          { value: 'transfer', label: 'Transfer' },
          { value: 'mint', label: 'Mint' },
          { value: 'burn', label: 'Burn' },
        ],
      },
      { key: 'from', label: 'From address', placeholder: 'G… or C…' },
      { key: 'to', label: 'To address', placeholder: 'G… or C…' },
      ...bounds,
    ]
  }

  return [
    { key: 'asset', label: 'Asset contract', placeholder: 'Token contract C…' },
    { key: 'from', label: 'From address', placeholder: 'G… or C…' },
    { key: 'to', label: 'To address', placeholder: 'G… or C…' },
    ...bounds,
  ]
})

watch(
  () => [route.query, props.kind],
  () => {
    for (const field of fields.value) {
      const value = route.query[field.key]
      draft[field.key] = typeof value === 'string' ? value : ''
    }

    formError.value = ''
  },
  { immediate: true },
)

function baseQuery() {
  const query: Record<string, string> = {}
  for (const key of ['network', 'limit']) {
    const value = route.query[key]
    if (typeof value === 'string' && value) {
      query[key] = value
    }
  }

  return query
}

async function apply() {
  const minimum = String(draft.min_ledger ?? '').trim()
  const maximum = String(draft.max_ledger ?? '').trim()
  if (minimum && maximum && Number(minimum) > Number(maximum)) {
    formError.value = 'From ledger must be less than or equal to To ledger.'
    return
  }

  formError.value = ''
  const query = baseQuery()
  for (const field of fields.value) {
    const value = String(draft[field.key] ?? '').trim()
    if (value) {
      query[field.key] = value
    }
  }

  await navigateTo({ path: route.path, query })
}

async function reset() {
  for (const field of fields.value) {
    draft[field.key] = ''
  }

  formError.value = ''
  await navigateTo({ path: route.path, query: baseQuery() })
}
</script>

<template>
  <form
    class="panel padded filter-form"
    aria-label="Activity filters"
    @submit.prevent="apply"
  >
    <h2>
      Filter
      {{ kind === 'transfers' ? 'token movements' : kind === 'nfts' ? 'NFT movements' : kind }}
    </h2>
    <div class="filter-grid">
      <div
        v-for="field in fields"
        :key="field.key"
        class="filter-field"
      >
        <label
          :id="`filter-${kind}-${field.key}-label`"
          :for="`filter-${kind}-${field.key}`"
          >{{ field.label }}</label
        >
        <CustomSelect
          v-if="field.options"
          :id="`filter-${kind}-${field.key}`"
          :model-value="String(draft[field.key] ?? '')"
          :name="field.key"
          :options="field.options"
          @update:model-value="draft[field.key] = $event"
        />
        <input
          v-else
          :id="`filter-${kind}-${field.key}`"
          v-model="draft[field.key]"
          :name="field.key"
          :placeholder="field.placeholder"
          :type="field.key.endsWith('_ledger') ? 'number' : 'text'"
          :min="field.key.endsWith('_ledger') ? 1 : undefined"
          :max="field.key.endsWith('_ledger') ? 4294967295 : undefined"
          :step="field.key.endsWith('_ledger') ? 1 : undefined"
          :maxlength="
            field.key === 'token_id' ? 78 : ['function', 'event_name'].includes(field.key) ? 32 : 56
          "
          :inputmode="field.key === 'token_id' ? 'numeric' : undefined"
          autocomplete="off"
          spellcheck="false"
        />
      </div>
    </div>
    <p
      v-if="formError"
      class="notice error"
      role="alert"
    >
      {{ formError }}
    </p>
    <div class="filter-actions">
      <p class="muted small">Exact matches in indexed history. All conditions apply together.</p>
      <button
        class="subtle-button"
        type="button"
        @click="reset"
      >
        Reset filters
      </button>
      <button
        class="button"
        type="submit"
      >
        Apply filters
      </button>
    </div>
  </form>
</template>
