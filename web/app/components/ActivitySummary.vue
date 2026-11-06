<script setup lang="ts">
import type { Activity } from '~/types'
import {
  activityBars,
  activityLine,
  activityQuery,
  activitySuccessRate,
} from '~/utils/activity.mjs'

const props = withDefaults(
  defineProps<{
    kind: Activity['kind']
    query?: Record<string, unknown>
    title?: string
    refreshKey?: number | null
    variant?: 'bars' | 'line'
    caption?: string
  }>(),
  { query: () => ({}), variant: 'bars' },
)

const { data, pending, error, refresh } = await useLazyFetch<Activity>(
  computed(() => `/api/v1/activity/${props.kind}`),
  { query: computed(() => activityQuery(props.query)) },
)
const selected = ref(0)
const bars = computed(() => activityBars(data.value?.buckets || []))
const line = computed(() => activityLine(data.value?.buckets || []))
const maximumCount = computed(() =>
  Math.max(0, ...(data.value?.buckets.map((item) => item.count) || [])),
)
const bucket = computed(() => data.value?.buckets[selected.value])
const successRate = computed(() =>
  data.value && props.kind === 'transactions' ? activitySuccessRate(data.value) : null,
)
const title = computed(
  () => props.title || (props.kind === 'transfers' ? 'Movement activity' : 'Transaction activity'),
)
const descriptionID = `activity-${useId()}`

watch(
  () => data.value?.buckets,
  (buckets) => {
    selected.value = Math.max(0, (buckets?.length || 1) - 1)
  },
  { immediate: true },
)

watch(
  () => props.refreshKey,
  () => {
    refresh()
  },
)

function selectBucket(index: number) {
  selected.value = Math.max(0, Math.min((data.value?.buckets.length || 1) - 1, index))
}

function categoryLabel(key: string) {
  const labels: Record<string, string> = {
    success: 'Success',
    failed: 'Failed',
    transfer: 'Transfers',
    mint: 'Mints',
    burn: 'Burns',
    clawback: 'Clawbacks',
    contract: 'Contract',
    system: 'System',
    diagnostic: 'Diagnostic',
    other: 'Other',
  }

  return labels[key] || key
}
</script>

<template>
  <section
    class="panel padded activity-summary"
    :aria-label="title"
  >
    <div class="section-heading">
      <h2>{{ title }}</h2>
      <span
        v-if="data"
        class="activity-total mono"
      >
        {{ data.total.toLocaleString('en-US') }} matching
      </span>
    </div>
    <p
      v-if="pending && !data"
      class="muted small"
      aria-live="polite"
    >
      Loading activity statistics…
    </p>
    <div v-else-if="error">
      <p class="muted small">Activity statistics are temporarily unavailable.</p>
      <button
        class="subtle-button"
        type="button"
        @click="refresh()"
      >
        Retry activity
      </button>
    </div>
    <template v-else-if="data">
      <p
        v-if="!data.ledger_count"
        class="muted small"
      >
        No indexed ledgers are available within this view.
      </p>
      <template v-else>
        <p class="muted small activity-window">
          Latest {{ data.ledger_count }} indexed ledgers within this view (limit
          {{ data.window_limit }}), {{ data.first_ledger }}–{{ data.last_ledger }}. Counts cover
          this window and its selected filters, not complete network history.
        </p>
        <p
          v-if="caption"
          class="muted small activity-window"
        >
          {{ caption }}
        </p>
        <div class="activity-breakdown">
          <span
            v-for="category in data.breakdown"
            :key="category.key"
            class="activity-category"
          >
            <span class="muted">{{ categoryLabel(category.key) }}</span>
            <strong class="mono">{{ category.count.toLocaleString('en-US') }}</strong>
          </span>
          <span
            v-if="successRate !== null"
            class="activity-category"
          >
            <span class="muted">Successful</span>
            <strong class="mono">{{ successRate }}%</strong>
          </span>
        </div>
        <p
          v-if="!data.total"
          class="muted small"
        >
          No matching {{ kind === 'transfers' ? 'movements' : kind }} in this indexed window.
        </p>
        <div class="activity-plot">
          <div
            class="activity-scale mono"
            aria-hidden="true"
          >
            <span>{{ maximumCount.toLocaleString('en-US') }}</span>
            <span>0</span>
          </div>
          <svg
            class="activity-chart"
            viewBox="0 0 600 180"
            preserveAspectRatio="none"
            role="img"
            tabindex="0"
            :aria-label="`${title} by indexed ledger bucket`"
            :aria-describedby="descriptionID"
            @keydown.left.prevent="selectBucket(selected - 1)"
            @keydown.right.prevent="selectBucket(selected + 1)"
            @keydown.home.prevent="selectBucket(0)"
            @keydown.end.prevent="selectBucket(data.buckets.length - 1)"
          >
            <title>{{ title }}: {{ data.total }} matching records</title>
            <line
              v-for="y in [10, 90, 170]"
              :key="y"
              x1="0"
              x2="600"
              :y1="y"
              :y2="y"
              class="activity-grid-line"
            />
            <g
              v-for="(bar, index) in bars"
              :key="data.buckets[index]!.first_ledger"
              @pointerenter="selectBucket(index)"
              @click="selectBucket(index)"
            >
              <title>
                Ledgers {{ data.buckets[index]!.first_ledger }}–{{
                  data.buckets[index]!.last_ledger
                }}: {{ data.buckets[index]!.count }} matching records
              </title>
              <rect
                :x="bar.x"
                y="0"
                :width="bar.width"
                height="180"
                fill="transparent"
              />
              <rect
                v-if="variant === 'bars'"
                :x="bar.x"
                :y="bar.y"
                :width="bar.width"
                :height="bar.height"
                rx="4"
                class="activity-bar"
                :class="{ selected: selected === index }"
              />
              <line
                v-if="selected === index"
                :x1="bar.x"
                :x2="bar.x + bar.width"
                y1="175"
                y2="175"
                class="activity-selected-marker"
              />
            </g>
            <polyline
              v-if="variant === 'line'"
              :points="line"
              class="activity-trend"
              vector-effect="non-scaling-stroke"
            />
          </svg>
        </div>
        <div
          v-if="bucket"
          class="activity-selection"
        >
          <button
            class="subtle-button"
            type="button"
            aria-label="Previous activity bucket"
            :disabled="selected === 0"
            @click="selectBucket(selected - 1)"
          >
            ←
          </button>
          <p
            class="small"
            aria-live="polite"
          >
            <strong>{{ bucket.count.toLocaleString('en-US') }} matching</strong> · Ledgers
            {{ bucket.first_ledger }}–{{ bucket.last_ledger }}
            <span class="muted activity-bucket-time">
              Ending <TimeValue :value="bucket.closed_at" />
            </span>
          </p>
          <button
            class="subtle-button"
            type="button"
            aria-label="Next activity bucket"
            :disabled="selected >= data.buckets.length - 1"
            @click="selectBucket(selected + 1)"
          >
            →
          </button>
        </div>
        <p
          :id="descriptionID"
          class="muted small"
        >
          Each {{ variant === 'line' ? 'point' : 'bar' }} covers up to ten indexed ledgers. Use the
          arrows or focus the chart and press Left or Right to inspect a bucket.
          {{
            variant === 'line'
              ? 'The baseline represents zero matches.'
              : 'Zero-height bars represent zero matches.'
          }}
          <span v-if="kind === 'transfers'">Movement counts do not sum token amounts.</span>
        </p>
        <details>
          <summary>Activity data ({{ data.buckets.length }} buckets)</summary>
          <dl class="activity-data">
            <template
              v-for="item in data.buckets"
              :key="item.first_ledger"
            >
              <dt>Ledgers {{ item.first_ledger }}–{{ item.last_ledger }}</dt>
              <dd>{{ item.count.toLocaleString('en-US') }} matching</dd>
            </template>
          </dl>
        </details>
      </template>
    </template>
  </section>
</template>
