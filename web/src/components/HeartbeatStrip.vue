<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDevicePixelRatio, useEventListener, useResizeObserver } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import type { Heartbeat } from '@/types'
import { formatTime, latency } from '@/lib/format'
const props = withDefaults(
  defineProps<{ beats: Heartbeat[]; count?: number; alignPixels?: boolean }>(),
  { count: 60, alignPixels: false },
)
const { t } = useI18n()
const strip = ref<HTMLDivElement>()
const positions = ref<{ left: string; width: string }[]>([])
const activeIndex = ref<number>()

if (props.alignPixels) {
  const { pixelRatio } = useDevicePixelRatio()

  function align() {
    if (!strip.value) return
    const rect = strip.value.getBoundingClientRect()
    const layoutWidth = parseFloat(getComputedStyle(strip.value).width)
    if (!rect.width || !layoutWidth) return

    // 将条宽和左右边缘对齐物理像素，避免缩放后同一行出现 3、4 像素交替。
    const ratio = pixelRatio.value
    const scale = rect.width / layoutWidth
    const start = Math.round(rect.left * ratio)
    const end = Math.round(rect.right * ratio)
    // 窄屏至少留出一个物理像素的间隔，防止相邻条柱连成粗条。
    const maxWidth = Math.floor((end - start - (props.count - 1)) / props.count)
    const width = Math.max(
      1,
      Math.min(Math.round(3 * scale * ratio), maxWidth),
    )
    const step = (end - start - width) / Math.max(1, props.count - 1)
    positions.value = Array.from({ length: props.count }, (_, index) => ({
      left: `${((start + Math.round(index * step)) / ratio - rect.left) / scale}px`,
      width: `${width / ratio / scale}px`,
    }))
  }

  useResizeObserver(strip, align)
  useEventListener(window, 'resize', align)
  watch([pixelRatio, () => props.count], align, { flush: 'post' })
}
const slots = computed(() => {
  const recent = props.beats.slice(-props.count)
  return [...Array<null>(Math.max(0, props.count - recent.length)).fill(null), ...recent]
})
function label(beat: Heartbeat | null) {
  if (!beat) return t('common.noData')
  return t('status.heartbeatPoint', {
    time: formatTime(beat.time),
    status: t(`state.${beat.status}`),
    latency: latency(beat.latency),
  })
}
</script>
<template>
  <div
    ref="strip"
    class="heartbeat-strip"
    :class="{ 'heartbeat-strip-aligned': alignPixels && positions.length === count }"
    :aria-label="t('status.heartbeat', { count: props.count })"
  >
    <!-- 整行共用矢量画布，避免浏览器逐个绘制背景时再次对小数边缘取整。 -->
    <svg
      v-if="alignPixels && positions.length === count"
      class="heartbeat-shapes"
      aria-hidden="true"
      focusable="false"
      shape-rendering="crispEdges"
    >
      <rect
        v-for="(beat, index) in slots"
        :key="index"
        :x="positions[index]?.left"
        :width="positions[index]?.width"
        y="1"
        height="14"
        rx="1"
        :class="[
          'heartbeat-shape',
          beat ? `beat-${beat.status}` : 'beat-empty',
          { 'heartbeat-shape-active': activeIndex === index },
        ]"
      />
    </svg>
    <span
      v-for="(beat, index) in slots"
      :key="index"
      :tabindex="beat ? 0 : undefined"
      :class="['heartbeat', beat ? `beat-${beat.status}` : 'beat-empty']"
      :style="positions[index]"
      :aria-label="label(beat)"
      :data-tooltip="label(beat)"
      @mouseenter="activeIndex = index"
      @mouseleave="activeIndex = undefined"
      @focus="activeIndex = index"
      @blur="activeIndex = undefined"
    >
      <span class="sr-only">{{ label(beat) }}</span>
    </span>
  </div>
</template>
