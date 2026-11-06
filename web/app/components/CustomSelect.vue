<script setup lang="ts">
import type { CSSProperties } from 'vue'

const props = defineProps<{
  id: string
  name: string
  modelValue: string
  options: { value: string; label: string }[]
}>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const trigger = ref<HTMLButtonElement>()
const popup = ref<HTMLUListElement>()
const open = ref(false)
const active = ref(0)
const position = ref<CSSProperties>({})
const selected = computed(() =>
  props.options.findIndex((option) => option.value === props.modelValue),
)
const label = computed(() => props.options[selected.value]?.label || 'Choose an option')
const listID = computed(() => `${props.id}-options`)
let typed = ''
let lastTypedAt = 0

function optionID(index: number) {
  return `${props.id}-option-${index}`
}

function close() {
  open.value = false
  typed = ''
}

function updatePosition() {
  if (!open.value || !trigger.value) {
    return
  }

  const rect = trigger.value.getBoundingClientRect()
  const viewport = window.visualViewport
  const viewportTop = viewport?.offsetTop || 0
  const viewportLeft = viewport?.offsetLeft || 0
  const viewportWidth = viewport?.width || window.innerWidth
  const viewportBottom = viewportTop + (viewport?.height || window.innerHeight)
  if (rect.bottom < viewportTop || rect.top > viewportBottom) {
    close()
    return
  }

  const gap = 8
  const below = Math.max(0, viewportBottom - rect.bottom - gap)
  const above = Math.max(0, rect.top - viewportTop - gap)
  const desiredHeight = Math.min(popup.value?.scrollHeight || props.options.length * 44 + 12, 280)
  const upwards = below < desiredHeight && above > below
  const maxHeight = Math.min(280, upwards ? above : below)
  const height = Math.min(desiredHeight, maxHeight)
  const width = Math.min(rect.width, viewportWidth - gap * 2)
  const left = Math.max(
    viewportLeft + gap,
    Math.min(rect.left, viewportLeft + viewportWidth - width - gap),
  )

  position.value = {
    left: `${left}px`,
    top: `${upwards ? rect.top - gap - height : rect.bottom + gap}px`,
    width: `${width}px`,
    maxHeight: `${maxHeight}px`,
  }
}

async function show(index = Math.max(0, selected.value)) {
  if (!props.options.length) {
    return
  }

  active.value = index
  open.value = true
  updatePosition()
  await nextTick()
  updatePosition()
  scrollActive()
}

function scrollActive() {
  const option = document.getElementById(optionID(active.value))
  const menu = popup.value
  if (!option || !menu) {
    return
  }

  if (option.offsetTop < menu.scrollTop) {
    menu.scrollTop = option.offsetTop
  } else if (option.offsetTop + option.offsetHeight > menu.scrollTop + menu.clientHeight) {
    menu.scrollTop = option.offsetTop + option.offsetHeight - menu.clientHeight
  }
}

function choose(index: number, restoreFocus = true) {
  const option = props.options[index]
  if (!option) {
    return
  }

  emit('update:modelValue', option.value)
  close()
  if (restoreFocus) {
    trigger.value?.focus({ preventScroll: true })
  }
}

function toggle() {
  if (open.value) {
    close()
  } else {
    void show()
  }
}

function keydown(event: KeyboardEvent) {
  if (event.ctrlKey || event.metaKey) {
    return
  }

  const key = event.key
  if (key === 'Escape') {
    if (open.value) {
      event.preventDefault()
      close()
    }
    return
  }

  if (key === 'Tab') {
    if (open.value) {
      choose(active.value, false)
    }
    return
  }

  if (['ArrowDown', 'ArrowUp', 'Home', 'End', 'PageDown', 'PageUp'].includes(key)) {
    event.preventDefault()
    if (!open.value) {
      void show(
        key === 'End' ? props.options.length - 1 : key === 'Home' ? 0 : Math.max(0, selected.value),
      )
    } else if (event.altKey && key === 'ArrowUp') {
      choose(active.value)
    } else if (key === 'Home' || key === 'End') {
      active.value = key === 'Home' ? 0 : props.options.length - 1
    } else {
      const movement = { ArrowDown: 1, ArrowUp: -1, PageDown: 10, PageUp: -10 }[key] || 0
      active.value = Math.max(0, Math.min(props.options.length - 1, active.value + movement))
    }
    typed = ''
    return
  }

  if (key === 'Enter' || key === ' ') {
    event.preventDefault()
    if (open.value) {
      choose(active.value)
    } else {
      void show()
    }
    return
  }

  if (key.length === 1 && !event.altKey) {
    event.preventDefault()
    const now = Date.now()
    typed = now - lastTypedAt < 700 ? typed + key.toLowerCase() : key.toLowerCase()
    lastTypedAt = now
    const repeated = [...typed].every((character) => character === typed[0])
    const prefix = repeated ? typed[0]! : typed
    const start = repeated ? active.value + 1 : 0
    const indexes = props.options.map((_, index) => (start + index) % props.options.length)
    const match = indexes.find((index) =>
      props.options[index]!.label.toLowerCase().startsWith(prefix),
    )
    if (!open.value) {
      void show(match ?? Math.max(0, selected.value))
    } else if (match !== undefined) {
      active.value = match
    }
  }
}

function outside(event: Event) {
  const target = event.target as Node | null
  if (open.value && !trigger.value?.contains(target) && !popup.value?.contains(target)) {
    close()
  }
}

watch(active, async () => {
  if (open.value) {
    await nextTick()
    scrollActive()
  }
})

watch(() => props.modelValue, close)
watch(() => props.options, close)

onMounted(() => {
  document.addEventListener('pointerdown', outside)
  document.addEventListener('focusin', outside)
  window.addEventListener('resize', updatePosition)
  window.addEventListener('scroll', updatePosition, true)
  window.visualViewport?.addEventListener('resize', updatePosition)
  window.visualViewport?.addEventListener('scroll', updatePosition)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', outside)
  document.removeEventListener('focusin', outside)
  window.removeEventListener('resize', updatePosition)
  window.removeEventListener('scroll', updatePosition, true)
  window.visualViewport?.removeEventListener('resize', updatePosition)
  window.visualViewport?.removeEventListener('scroll', updatePosition)
})
</script>

<template>
  <div class="custom-select">
    <input
      type="hidden"
      :name="name"
      :value="modelValue"
    />
    <button
      :id="id"
      ref="trigger"
      class="select-trigger"
      type="button"
      role="combobox"
      :aria-labelledby="`${id}-label`"
      aria-haspopup="listbox"
      :aria-expanded="open"
      :aria-controls="open ? listID : undefined"
      :aria-activedescendant="open ? optionID(active) : undefined"
      :disabled="!options.length"
      @click="toggle"
      @keydown="keydown"
    >
      <span>{{ label }}</span>
      <svg
        viewBox="0 0 20 20"
        fill="none"
        stroke="currentColor"
        stroke-width="1.6"
        aria-hidden="true"
        :class="{ 'is-open': open }"
      >
        <path d="m5 7.5 5 5 5-5" />
      </svg>
    </button>
    <Teleport to="body">
      <ul
        v-if="open"
        :id="listID"
        ref="popup"
        class="select-popup"
        role="listbox"
        :aria-labelledby="`${id}-label`"
        :style="position"
      >
        <li
          v-for="(option, index) in options"
          :id="optionID(index)"
          :key="option.value"
          role="option"
          :aria-selected="option.value === modelValue"
          :class="{ 'is-active': index === active, 'is-selected': option.value === modelValue }"
          @pointerdown.prevent
          @pointermove="active = index"
          @click="choose(index)"
        >
          <span>{{ option.label }}</span>
          <svg
            v-if="option.value === modelValue"
            viewBox="0 0 20 20"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            aria-hidden="true"
          >
            <path d="m4 10 4 4 8-8" />
          </svg>
        </li>
      </ul>
    </Teleport>
  </div>
</template>
