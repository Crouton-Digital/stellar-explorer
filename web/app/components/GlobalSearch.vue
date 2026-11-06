<script setup lang="ts">
import type { CSSProperties } from 'vue'
import type { SearchMatch, SearchSuggestions } from '~/types'

const route = useRoute()
const trigger = ref<HTMLButtonElement>()
const input = ref<HTMLInputElement>()
const popup = ref<HTMLDialogElement>()
const query = ref('')
const open = ref(false)
const loading = ref(false)
const navigating = ref(false)
const composing = ref(false)
const error = ref('')
const submittedError = ref(false)
const items = ref<SearchMatch[]>([])
const active = ref(0)
const completedQuery = ref('')
const position = ref<CSSProperties>({})
const listID = `search-results-${useId()}`
const dialogID = `search-dialog-${useId()}`
const headingID = `${dialogID}-heading`
const trimmed = computed(() => query.value.trim())
const eligible = computed(
  () =>
    /^[1-9][0-9]{0,9}$/.test(trimmed.value) ||
    /^[0-9a-f]{4,64}$/i.test(trimmed.value) ||
    /^[GC][A-Z2-7]{3,55}$/i.test(trimmed.value),
)
let timer: ReturnType<typeof setTimeout> | undefined
let controller: AbortController | undefined
let revision = 0
let previousOverflow: string | undefined

function optionID(index: number) {
  return `${listID}-${index}`
}

function label(kind: SearchMatch['type']) {
  return {
    ledger: 'Ledger',
    transaction: 'Transaction',
    account: 'Account',
    contract: 'Contract',
  }[kind]
}

function cancel() {
  clearTimeout(timer)
  controller?.abort()
  controller = undefined
  revision++
  loading.value = false
}

function close() {
  cancel()
  if (!open.value) {
    return
  }

  open.value = false
  composing.value = false
  query.value = ''
  popup.value?.close()
  restoreScrolling()
  trigger.value?.focus({ preventScroll: true })
}

function restoreScrolling() {
  if (previousOverflow !== undefined) {
    document.documentElement.style.overflow = previousOverflow
    previousOverflow = undefined
  }
}

function updatePosition() {
  if (!open.value) {
    return
  }

  const viewport = window.visualViewport
  const left = viewport?.offsetLeft || 0
  const top = viewport?.offsetTop || 0
  const width = viewport?.width || window.innerWidth
  const height = viewport?.height || window.innerHeight

  position.value = {
    left: `${left + width / 2}px`,
    top: `${top + height / 2}px`,
    width: `${Math.min(640, Math.max(0, width - 32))}px`,
    maxHeight: `${Math.min(480, Math.max(0, height - 32))}px`,
  }
}

async function show() {
  if (open.value) {
    input.value?.focus({ preventScroll: true })
    return
  }

  open.value = true
  previousOverflow = document.documentElement.style.overflow
  document.documentElement.style.overflow = 'hidden'
  updatePosition()

  await nextTick()
  if (!open.value) {
    return
  }

  popup.value?.showModal()
  input.value?.focus({ preventScroll: true })
  schedule()
}

async function load() {
  cancel()
  if (!eligible.value || composing.value) {
    return false
  }

  const current = revision
  const text = trimmed.value
  const request = new AbortController()
  controller = request
  loading.value = true
  error.value = ''
  submittedError.value = false

  try {
    const result = await $fetch<SearchSuggestions>('/api/v1/search/suggestions', {
      query: { q: text },
      signal: request.signal,
      retry: 0,
    })
    if (current !== revision || request.signal.aborted || trimmed.value !== text) {
      return false
    }

    items.value = result.items
    completedQuery.value = text
    active.value = 0
    return true
  } catch {
    if (current === revision && !request.signal.aborted) {
      error.value = 'Search is temporarily unavailable. Try again.'
    }
    return false
  } finally {
    if (current === revision) {
      loading.value = false
      controller = undefined
    }
  }
}

function schedule() {
  cancel()
  items.value = []
  completedQuery.value = ''
  error.value = ''
  submittedError.value = false
  active.value = 0
  if (!open.value || !eligible.value || composing.value) {
    return
  }

  loading.value = true
  timer = setTimeout(() => void load(), 220)
}

async function choose(item: SearchMatch) {
  close()
  navigating.value = true
  try {
    await navigateTo(item.path)
  } finally {
    navigating.value = false
  }
}

async function submit() {
  if (!open.value || !trimmed.value || navigating.value || composing.value) {
    return
  }

  const text = trimmed.value
  let current = revision
  navigating.value = true
  try {
    if (eligible.value && completedQuery.value !== text) {
      const accepted = await load()
      if (!accepted) {
        return
      }
    }

    if (!open.value || trimmed.value !== text) {
      return
    }

    if (error.value && completedQuery.value !== text) {
      return
    }

    const selected = items.value[active.value]
    if (selected && completedQuery.value === text) {
      await choose(selected)
      return
    }

    cancel()
    current = revision
    const request = new AbortController()
    controller = request
    const result = await $fetch<{ path: string }>('/api/v1/search', {
      query: { q: text },
      signal: request.signal,
      retry: 0,
    })
    if (revision === current && trimmed.value === text) {
      close()
      await navigateTo(result.path)
    }
  } catch (reason: any) {
    if (revision === current && open.value && trimmed.value === text) {
      error.value = reason?.data?.error?.message || 'Search is unavailable. Try again.'
      submittedError.value = true
    }
  } finally {
    navigating.value = false
  }
}

function compositionStart() {
  composing.value = true
  cancel()
}

function compositionEnd() {
  composing.value = false
  schedule()
}

function keydown(event: KeyboardEvent) {
  if (event.isComposing) {
    return
  }

  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (items.value.length) {
      active.value =
        (active.value + (event.key === 'ArrowDown' ? 1 : -1) + items.value.length) %
        items.value.length
      document.getElementById(optionID(active.value))?.scrollIntoView({ block: 'nearest' })
    }
  }
}

function outside(event: MouseEvent) {
  if (event.target !== popup.value) {
    return
  }

  const rect = popup.value.getBoundingClientRect()
  if (
    event.clientX < rect.left ||
    event.clientX > rect.right ||
    event.clientY < rect.top ||
    event.clientY > rect.bottom
  ) {
    close()
  }
}

function cancelDialog() {
  if (!composing.value) {
    close()
  }
}

function trapFocus(event: KeyboardEvent) {
  if (event.key === 'Escape' && !event.isComposing && !composing.value) {
    event.preventDefault()
    close()
    return
  }

  if (event.key !== 'Tab' || !popup.value) {
    return
  }

  const controls = popup.value.querySelectorAll<HTMLElement>('input, button:not(:disabled)')
  const first = controls[0]
  const last = controls[controls.length - 1]
  if (event.shiftKey && event.target === first) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && event.target === last) {
    event.preventDefault()
    first?.focus()
  }
}

function shortcut(event: KeyboardEvent) {
  if (event.defaultPrevented || event.isComposing || event.altKey) {
    return
  }

  const target = event.target as HTMLElement | null
  if (
    target?.closest('input, textarea, select, [contenteditable]:not([contenteditable="false"])')
  ) {
    return
  }

  const command = (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k'
  const slash = !event.ctrlKey && !event.metaKey && event.key === '/'
  if (command || slash) {
    event.preventDefault()
    void show()
  }
}

function clear() {
  query.value = ''
  input.value?.focus()
}

watch(query, schedule)
watch(() => route.fullPath, close)

onMounted(() => {
  document.addEventListener('keydown', shortcut)
  window.addEventListener('resize', updatePosition)
  window.visualViewport?.addEventListener('resize', updatePosition)
  window.visualViewport?.addEventListener('scroll', updatePosition)
})

onBeforeUnmount(() => {
  cancel()
  popup.value?.close()
  restoreScrolling()
  document.removeEventListener('keydown', shortcut)
  window.removeEventListener('resize', updatePosition)
  window.visualViewport?.removeEventListener('resize', updatePosition)
  window.visualViewport?.removeEventListener('scroll', updatePosition)
})
</script>

<template>
  <button
    ref="trigger"
    class="search"
    type="button"
    aria-label="Open search"
    aria-haspopup="dialog"
    aria-keyshortcuts="/ Control+k Meta+k"
    :aria-expanded="open"
    :aria-controls="dialogID"
    @click="show"
  >
    <svg
      class="search-icon"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.7"
      aria-hidden="true"
    >
      <circle
        cx="10.5"
        cy="10.5"
        r="6.5"
      />
      <path
        d="m16 16 4 4"
        stroke-linecap="round"
      />
    </svg>
    <span class="search-placeholder">Search hash, address or ledger…</span>
    <kbd aria-hidden="true">/</kbd>
  </button>
  <Teleport to="body">
    <dialog
      :id="dialogID"
      ref="popup"
      class="search-popup"
      aria-modal="true"
      :aria-labelledby="headingID"
      :style="position"
      @cancel.prevent="cancelDialog"
      @click="outside"
      @keydown="trapFocus"
    >
      <span
        :id="headingID"
        class="sr-only"
      >
        Search indexed Testnet
      </span>
      <form
        v-if="open"
        class="search-form"
        role="search"
        :aria-busy="loading || navigating"
        @submit.prevent="submit"
      >
        <div class="search-field">
          <svg
            class="search-icon"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.7"
            aria-hidden="true"
          >
            <circle
              cx="10.5"
              cy="10.5"
              r="6.5"
            />
            <path
              d="m16 16 4 4"
              stroke-linecap="round"
            />
          </svg>
          <label
            class="sr-only"
            for="global-search"
          >
            Search transactions, accounts, contracts or ledgers
          </label>
          <input
            id="global-search"
            ref="input"
            v-model="query"
            type="search"
            role="combobox"
            aria-autocomplete="list"
            aria-haspopup="listbox"
            :aria-expanded="items.length > 0"
            :aria-controls="items.length ? listID : undefined"
            :aria-activedescendant="items.length ? optionID(active) : undefined"
            placeholder="Tx hash / account / contract / ledger…"
            autocomplete="off"
            spellcheck="false"
            maxlength="128"
            @keydown="keydown"
            @compositionstart="compositionStart"
            @compositionend="compositionEnd"
          />
          <button
            v-if="query"
            class="search-clear"
            type="button"
            aria-label="Clear search"
            @click="clear"
          >
            ×
          </button>
          <button
            class="search-close"
            type="button"
            aria-label="Close search"
            @click="close"
          >
            <svg
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.7"
              stroke-linecap="round"
              aria-hidden="true"
            >
              <path d="m6 6 12 12M6 18 18 6" />
            </svg>
          </button>
        </div>
        <div
          v-if="trimmed"
          class="search-results"
        >
          <p
            v-if="loading"
            class="search-message muted"
            role="status"
          >
            Searching indexed data…
          </p>
          <div
            v-else-if="error"
            class="search-message"
          >
            <p :role="submittedError ? 'alert' : 'status'">{{ error }}</p>
            <button
              v-if="eligible"
              class="subtle-button"
              type="button"
              @click="load"
            >
              Retry search
            </button>
          </div>
          <p
            v-else-if="!eligible"
            class="search-message muted"
          >
            Enter a ledger number or at least 4 hash/address characters.
          </p>
          <p
            v-else-if="completedQuery === trimmed && !items.length"
            class="search-message muted"
            role="status"
          >
            No matches in indexed history. Try a longer prefix or another identifier.
          </p>
          <ul
            v-if="items.length"
            :id="listID"
            role="listbox"
            aria-label="Search results"
          >
            <li
              v-for="(item, index) in items"
              :id="optionID(index)"
              :key="`${item.type}:${item.key}`"
              class="search-option"
              :class="{ active: index === active }"
              role="option"
              :aria-selected="index === active"
              @pointermove="active = index"
              @mousedown.prevent
              @click="choose(item)"
            >
              <span class="search-kind">{{ label(item.type) }}</span>
              <strong class="mono">{{ item.key }}</strong>
              <span
                aria-hidden="true"
                class="search-open"
              >
                ↗
              </span>
            </li>
          </ul>
        </div>
        <div
          v-if="trimmed"
          class="search-popup-footer"
        >
          <span class="muted small">↑ ↓ Select · Enter Open</span>
          <button
            class="subtle-button"
            :disabled="navigating || !trimmed"
            type="submit"
          >
            Search ↵
          </button>
        </div>
      </form>
    </dialog>
  </Teleport>
</template>
