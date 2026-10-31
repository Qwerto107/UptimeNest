<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef, useId, watch } from 'vue'
import { useDevicePixelRatio, useDocumentVisibility, useEventListener, useIntersectionObserver, usePreferredReducedMotion, useResizeObserver } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import type { HeartbeatPoint } from '@/types'
import { formatTime, latency } from '@/lib/format'
import { cancelHeartbeatLayout, scheduleHeartbeatLayout } from '@/lib/heartbeat-layout'
const props = defineProps<{ beats: HeartbeatPoint[] }>()
const emit = defineEmits<{ capacity: [count: number] }>()
const { t } = useI18n()
const strip = ref<HTMLDivElement>()
const positions = shallowRef<{ left: string; width: string }[]>([])
const activeIndex = ref<number>()
const intro = ref(false)
const introRunning = ref(false)
const inView = ref(false)
const visibility = useDocumentVisibility()
const introDuration = ref(1000)
const stripWidth = ref(900)
const revealId = `heartbeat-reveal-${useId()}`
const reducedMotion = usePreferredReducedMotion()
let introduced = false
let introTimer: ReturnType<typeof setTimeout> | undefined
let introFrame = 0
useIntersectionObserver(strip, ([entry]) => { inView.value = !!entry?.isIntersecting })

const { pixelRatio } = useDevicePixelRatio()
let previousGeometry: number[] = []

function measure() {
  if (!strip.value) return
  const rect = strip.value.getBoundingClientRect()
  const layoutWidth = parseFloat(getComputedStyle(strip.value).width)
  if (!rect.width || !layoutWidth) return
  const ratio = pixelRatio.value
  const geometry = [rect.left, rect.width, layoutWidth, ratio]
  // 观察器可能重复通知；尺寸不变时保留位置和标签，避免整条心跳重新渲染。
  if (geometry.every((value, index) => value === previousGeometry[index])) return

  // 条宽和间距固定为 3px，再对齐物理像素；窗口大小只影响显示数量。
  const scale = rect.width / layoutWidth
  const start = Math.ceil(rect.left * ratio)
  const end = Math.floor(rect.right * ratio)
  const width = Math.max(1, Math.round(3 * scale * ratio))
  const gap = Math.max(1, Math.round(3 * scale * ratio))
  const count = Math.max(0, Math.min(1000, Math.floor((end - start + gap) / (width + gap))))
  // 剩余空隙留在左侧，让最新记录始终位于条带右端。
  const first = end - count * width - Math.max(0, count - 1) * gap
  const previousCount = positions.value.length
  const nextPositions = Array.from({ length: count }, (_, index) => ({
    left: `${((first + index * (width + gap)) / ratio - rect.left) / scale}px`,
    width: `${width / ratio / scale}px`,
  }))
  return () => {
    previousGeometry = geometry
    stripWidth.value = layoutWidth
    positions.value = nextPositions
    activeIndex.value = undefined
    if (count !== previousCount) emit('capacity', count)
  }
}

function align() { scheduleHeartbeatLayout(measure) }
useResizeObserver(strip, align)
useEventListener(window, 'resize', align)
watch(pixelRatio, align, { flush: 'post' })
onBeforeUnmount(() => cancelHeartbeatLayout(measure))
const slotCount = computed(() => positions.value.length)
const slots = computed(() => {
  const count = slotCount.value
  if (!count) return []
  const recent = props.beats.slice(-count)
  return [...Array<null>(Math.max(0, count - recent.length)).fill(null), ...recent]
})
watch([() => slots.value.length, inView, visibility], ([count, visible, pageVisibility]) => {
  if (!count || introduced) return
  if (reducedMotion.value === 'reduce') { introduced = true; return }
  if (!visible || pageVisibility !== 'visible') return
  introduced = true
  introDuration.value = Math.min(1200, Math.max(800, count * 6 + 200))
  intro.value = true
  // 遮罩先停在最左侧，等待内容真正绘制后起播，避免首屏出现时已扫到中间。
  introFrame = requestAnimationFrame(() => {
    introFrame = requestAnimationFrame(() => {
      if (!inView.value || visibility.value !== 'visible') { introduced = false; intro.value = false; return }
      if (reducedMotion.value === 'reduce') return
      introRunning.value = true
      introTimer = setTimeout(stopIntro, introDuration.value + 120)
    })
  })
}, { flush: 'post' })
function stopIntro() {
  cancelAnimationFrame(introFrame)
  clearTimeout(introTimer)
  intro.value = false
  introRunning.value = false
}
watch(reducedMotion, (value) => { if (value === 'reduce') { introduced = true; stopIntro() } })
onBeforeUnmount(stopIntro)
function label(beat: HeartbeatPoint | null) {
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
      <!-- 一条遮罩代替逐根独立动画；底层记录从首帧可见，不等待动效结束。 -->
      <defs v-if="intro"><mask :id="revealId" maskUnits="userSpaceOnUse" x="0" y="0" :width="stripWidth" height="16">
        <rect :width="stripWidth" height="16" fill="white" fill-opacity=".5" />
        <path :d="`M0 0H${stripWidth}L${stripWidth + 64} 16H0Z`" fill="white" class="heartbeat-sweep" :style="{ '--sweep-start': `${-stripWidth - 64}px`, animationDuration: `${introDuration}ms`, animationPlayState: introRunning ? 'running' : 'paused' }" />
      </mask></defs>
      <g :mask="intro ? `url(#${revealId})` : undefined">
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
      </g>
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

<style scoped>
.heartbeat-sweep {
  animation: heartbeat-sweep var(--motion-ease) 120ms backwards;
}
@keyframes heartbeat-sweep {
  from { transform: translateX(var(--sweep-start)); }
  to { transform: translateX(0); }
}
</style>
