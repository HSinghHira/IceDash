<script setup lang="ts">
// A bundle as a spotlight panel on Discover, like ZimaOS's "Build your
// private AI workspace" block: what the bundle is for on the left, and its
// apps as tiles on the right.
import { computed } from 'vue'
import type { CatalogApp } from '../../api/apps'
import AppIcon from '../../components/AppIcon.vue'
import Icon from '../../components/Icon.vue'
import type { Bundle } from '../../lib/bundles'
import { startBundle } from '../../store'
import { strings } from '../../strings'
import { admin, installedOf } from './actions'

const props = defineProps<{ bundle: Bundle; apps: CatalogApp[] }>()
defineEmits<{ open: [app: CatalogApp]; bundles: [] }>()

const words = computed(() => strings.bundles[props.bundle.id] ?? { name: props.bundle.id, about: '' })
const have = computed(() => props.apps.filter((a) => installedOf(a)).length)
const left = computed(() => props.apps.length - have.value)
</script>

<template>
  <section class="spot" :data-bundle="bundle.id" :aria-label="words.name">
    <div class="pitch">
      <span class="kicker"><Icon :name="bundle.icon" :size="14" />{{ strings.bundleLabel }} · {{ words.name }}</span>
      <p class="about">{{ words.about }}</p>
      <span class="acts">
        <button
          type="button"
          class="pill"
          :class="left ? 'install' : 'open'"
          :disabled="!admin || !left"
          :title="!admin ? strings.adminInstallOnly : undefined"
          @click="startBundle(bundle.id, apps.map((a) => a.id))"
        >
          {{ left ? strings.bundleInstall(left) : strings.bundleAllInstalled }}
        </button>
        <button type="button" class="link" @click="$emit('bundles')">{{ strings.seeAll }}</button>
      </span>
    </div>
    <ul class="tiles">
      <li v-for="a in apps.slice(0, 3)" :key="a.id">
        <button type="button" class="tile" :data-app="a.id" @click="$emit('open', a)">
          <span class="sq">
            <AppIcon :src="a.icon" :name="a.title" :size="68" />
            <Icon v-if="installedOf(a)" name="check" :size="14" class="ok" />
          </span>
          <span class="name">{{ a.title }}</span>
          <span class="about-sm">{{ a.tagline }}</span>
        </button>
      </li>
    </ul>
  </section>
</template>

<style scoped src="./shared.css"></style>
<style scoped>
.spot {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 28px;
  margin-top: 40px;
  padding: 28px 28px 28px 34px;
  border-radius: var(--radius-card);
  border: 1px solid var(--line);
  background: var(--card);
}
.pitch {
  display: grid;
  justify-items: start;
  gap: 12px;
}
.kicker {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--ac);
  font-size: 12.5px;
  font-weight: 600;
}
.about {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  line-height: 1.45;
  max-width: 42ch;
}
.acts {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 4px;
}
.tiles {
  display: grid;
  grid-template-columns: repeat(3, 128px);
  gap: 14px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.tile {
  display: grid;
  gap: 4px;
  width: 100%;
  padding: 0;
  border: 0;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.sq {
  position: relative;
  display: grid;
  place-items: center;
  aspect-ratio: 1;
  margin-bottom: 6px;
  border-radius: 18px;
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.05);
  transition: border-color 0.15s;
}
.tile:hover .sq {
  border-color: rgba(var(--ac-rgb), 0.5);
}
.sq .ok {
  position: absolute;
  top: 8px;
  right: 8px;
  color: var(--ac);
}
.name {
  font-size: 14px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.about-sm {
  color: var(--mut);
  font-size: 12px;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
@container (max-width: 760px) {
  .spot {
    grid-template-columns: 1fr;
    padding: 22px;
  }
  .tiles {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
