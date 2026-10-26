<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useDocumentVisibility, useIntersectionObserver } from '@vueuse/core'
import type { MonitorStatus } from '@/types'
import { useMotion } from '@/composables/useMotion'

const props = defineProps<{ status: MonitorStatus }>()
const edge = ref<HTMLElement>()
const flash = ref<HTMLElement>()
const { animate, reducedMotion } = useMotion()
const visibility = useDocumentVisibility()
const inView = ref(false)
const pending = ref(props.status === 'down')
const row = computed(() => edge.value?.parentElement)
let frame = 0
let animations: Animation[] = []
useIntersectionObserver(row, ([entry]) => { inView.value = !!entry?.isIntersecting })

function cancelHighlight() {
  cancelAnimationFrame(frame)
  frame = 0
  for (const animation of animations) animation.cancel()
  animations = []
}

watch([() => props.status, inView, visibility, reducedMotion], ([status, visible, pageVisibility, motion], previous) => {
  if (status !== 'down' || motion === 'reduce') {
    pending.value = false
    cancelHighlight()
    return
  }
  if (previous?.[0] !== 'down') pending.value = true
  if (!visible || pageVisibility !== 'visible') { cancelHighlight(); return }
  if (!pending.value || frame) return

  // 等故障行进入视口并完成首帧绘制，避免动效在用户看到之前已经播完。
  frame = requestAnimationFrame(() => {
    frame = requestAnimationFrame(() => {
      frame = 0
      if (props.status !== 'down' || !inView.value || visibility.value !== 'visible' || reducedMotion.value === 'reduce') return
      pending.value = false
      animations = [
        animate(edge.value, [{ transform: 'scaleY(0)' }, { transform: 'scaleY(1)' }], 520, 200),
        // 整体使用线性时间轴，分段缓动保留参考中的 280ms 渐亮和 920ms 淡出。
        animate(flash.value, [
          { opacity: 0, offset: 0, easing: 'ease-out' },
          { opacity: 1, offset: 280 / 1200, easing: 'ease-out' },
          { opacity: 0, offset: 1 },
        ], 1200, 200, 'linear'),
      ].filter((animation): animation is Animation => !!animation)
    })
  })
}, { immediate: true, flush: 'post' })
onBeforeUnmount(cancelHighlight)
</script>

<template>
  <span v-show="status === 'down'" ref="edge" class="pulse-failure-edge" :style="{ transform: pending && reducedMotion !== 'reduce' ? 'scaleY(0)' : undefined }" aria-hidden="true" />
  <span v-show="status === 'down'" ref="flash" class="pulse-failure-flash" aria-hidden="true" />
</template>

<style scoped>
.pulse-failure-flash {
  background: color-mix(in srgb, var(--pulse-down) 16%, transparent);
}
</style>
