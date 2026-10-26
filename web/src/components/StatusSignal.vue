<script setup lang="ts">
import { computed, ref } from 'vue'
import { useDocumentVisibility, useIntersectionObserver, usePreferredReducedMotion } from '@vueuse/core'
import type { MonitorStatus } from '@/types'

defineProps<{ status: MonitorStatus }>()
const signal = ref<HTMLElement>()
const visible = useDocumentVisibility()
const reduced = usePreferredReducedMotion()
const inView = ref(false)
useIntersectionObserver(signal, ([entry]) => { inView.value = !!entry?.isIntersecting })
const running = computed(() => visible.value === 'visible' && inView.value && reduced.value !== 'reduce')
</script>

<template>
  <span ref="signal" class="pulse-signal" :data-state="status" aria-hidden="true">
    <span v-if="status === 'up' || status === 'down'" class="pulse-signal-wave" :style="{ animationPlayState: running ? 'running' : 'paused' }" />
    <span class="pulse-signal-dot" />
  </span>
</template>
