<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDevicePixelRatio, useEventListener, useResizeObserver } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import type { Heartbeat } from '@/types'
import { formatTime, latency } from '@/lib/format'
const props = defineProps<{ beats: Heartbeat[] }>()
const emit = defineEmits<{ capacity: [count: number] }>()
const { t } = useI18n()
const strip = ref<HTMLDivElement>()
const positions = ref<{ left: string; width: string }[]>([])
const activeIndex = ref<number>()

const { pixelRatio } = useDevicePixelRatio()

function align() {
  if (!strip.value) return
  const rect = strip.value.getBoundingClientRect()
  const layoutWidth = parseFloat(getComputedStyle(strip.value).width)
  if (!rect.width || !layoutWidth) return

  // 条宽和间距固定为 3px，再对齐物理像素；窗口大小只影响显示数量。
  const ratio = pixelRatio.value
  const scale = rect.width / layoutWidth
  const start = Math.ceil(rect.left * ratio)
  const end = Math.floor(rect.right * ratio)
  const width = Math.max(1, Math.round(3 * scale * ratio))
  const gap = Math.max(1, Math.round(3 * scale * ratio))
  const count = Math.max(0, Math.min(1000, Math.floor((end - start + gap) / (width + gap))))
  // 剩余空隙留在左侧，让最新记录始终位于条带右端。
  const first = end - count * width - Math.max(0, count - 1) * gap
  const previousCount = positions.value.length
  positions.value = Array.from({ length: count }, (_, index) => ({
    left: `${((first + index * (width + gap)) / ratio - rect.left) / scale}px`,
    width: `${width / ratio / scale}px`,
  }))
  activeIndex.value = undefined
  if (count !== previousCount) emit('capacity', count)
}

useResizeObserver(strip, align)
useEventListener(window, 'resize', align)
watch(pixelRatio, align, { flush: 'post' })
const slots = computed(() => {
  const count = positions.value.length
  if (!count) return []
  const recent = props.beats.slice(-count)
  return [...Array<null>(Math.max(0, count - recent.length)).fill(null), ...recent]
})
function label(beat: Heartbeat | null) {
  if (!beat) return t('common.noData')
  return t('status.heartbeatPoint', {
    time: formatTime(beat.time),
    status: t(`state.${beat.status}`),
    latency: latency(beat.latency),
  })
}
// 无障碍文本与提示共用标签；悬停和聚焦只更新高亮，不重复格式化时间。
const labels = computed(() => slots.value.map(label))
</script>
<template>
  <div
    ref="strip"
    class="heartbeat-strip heartbeat-strip-aligned"
    :aria-label="t('status.heartbeat', { count: slots.length })"
  >
    <!-- 整行共用矢量画布，避免浏览器逐个绘制背景时再次对小数边缘取整。 -->
    <svg
      v-if="positions.length"
      class="heartbeat-shapes"
      aria-hidden="true"
      focusable="false"
      shape-rendering="geometricPrecision"
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
      :aria-label="labels[index]"
      :data-tooltip="labels[index]"
      @mouseenter="activeIndex = index"
      @mouseleave="activeIndex = undefined"
      @focus="activeIndex = index"
      @blur="activeIndex = undefined"
    >
      <span class="sr-only">{{ labels[index] }}</span>
    </span>
  </div>
</template>
