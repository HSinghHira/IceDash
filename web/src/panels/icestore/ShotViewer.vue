<script setup lang="ts">
// A screenshot at full size over the page: ← and → step, Escape closes.
import { computed, nextTick, onMounted, ref } from 'vue'
import Icon from '../../components/Icon.vue'
import { strings } from '../../strings'

const props = defineProps<{ shots: string[]; title: string }>()
const index = defineModel<number>({ required: true })
const emit = defineEmits<{ close: [] }>()

const box = ref<HTMLElement | null>(null)
const n = computed(() => props.shots.length)
const go = (d: number) => (index.value = (index.value + d + n.value) % n.value)

function key(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
  else if (e.key === 'ArrowRight') go(1)
  else if (e.key === 'ArrowLeft') go(-1)
  else return
  e.preventDefault()
  e.stopPropagation()
}

onMounted(() => nextTick(() => box.value?.focus()))
</script>

<template>
  <Teleport to="body">
    <div
      ref="box"
      class="viewer"
      role="dialog"
      aria-modal="true"
      :aria-label="`${title}: ${strings.screenshotN(index + 1, n)}`"
      tabindex="-1"
      @keydown="key"
      @click.self="emit('close')"
    >
      <button type="button" class="close" :aria-label="strings.close" @click="emit('close')"><Icon name="close" :size="18" /></button>
      <button v-if="n > 1" type="button" class="side prev" :aria-label="strings.previous" @click="go(-1)"><Icon name="arrowLeft" :size="20" /></button>
      <img :src="shots[index]" :alt="strings.screenshotN(index + 1, n)" referrerpolicy="no-referrer" />
      <button v-if="n > 1" type="button" class="side next" :aria-label="strings.next" @click="go(1)"><Icon name="arrowRight" :size="20" /></button>
      <p class="count mono">{{ index + 1 }} / {{ n }}</p>
    </div>
  </Teleport>
</template>

<style scoped>
.viewer {
  position: fixed;
  inset: 0;
  z-index: 900;
  display: grid;
  place-items: center;
  padding: 56px 72px;
  background: rgba(0, 0, 0, 0.86);
  outline: none;
}
img {
  max-width: 100%;
  max-height: calc(100vh - 112px);
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  object-fit: contain;
}
.close,
.side {
  position: absolute;
  display: grid;
  place-items: center;
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 50%;
  background: rgba(20, 20, 20, 0.7);
  color: #fff;
  cursor: pointer;
}
.close:hover,
.side:hover {
  background: rgba(50, 50, 50, 0.85);
}
.close {
  top: 16px;
  right: 16px;
  width: 38px;
  height: 38px;
}
.side {
  top: 50%;
  width: 44px;
  height: 44px;
  transform: translateY(-50%);
}
.prev {
  left: 16px;
}
.next {
  right: 16px;
}
.count {
  position: absolute;
  bottom: 16px;
  margin: 0;
  color: rgba(255, 255, 255, 0.7);
  font-family: var(--font-mono);
  font-size: 12px;
}
@media (max-width: 600px) {
  .viewer {
    padding: 56px 16px;
  }
  .side {
    top: auto;
    bottom: 8px;
    transform: none;
  }
}
</style>
