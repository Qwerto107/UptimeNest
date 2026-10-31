<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useDevicePixelRatio, useDocumentVisibility, useEventListener, useIntersectionObserver, usePreferredReducedMotion, useResizeObserver } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import type { HeartbeatPoint } from '@/types'
import { dark } from '@/composables/useApp'
import { formatTime, latency } from '@/lib/format'
import { cancelHeartbeatLayout, scheduleHeartbeatLayout } from '@/lib/heartbeat-layout'

const props = defineProps<{ beats: HeartbeatPoint[] }>()
const emit = defineEmits<{ capacity: [count: number] }>()
const { t } = useI18n()
const strip = ref<HTMLDivElement>()
const canvas = ref<HTMLCanvasElement>()
const activeIndex = ref<number>()
const inView = ref(false)
const visibility = useDocumentVisibility()
const reducedMotion = usePreferredReducedMotion()
const { pixelRatio } = useDevicePixelRatio()
const layout = shallowRef({ left: 0, width: 0, pixels: 0, height: 20, first: 0, bar: 0, gap: 0, count: 0 })
const slotCount = computed(() => layout.value.count)
const slots = computed(() => {
  const count = slotCount.value
  const recent = props.beats.slice(-count)
  if (!count) return []
  return [...Array<null>(Math.max(0, count - recent.length)).fill(null), ...recent]
})
const bitmap = document.createElement('canvas')
let palette = { up: '', down: '', empty: '' }
let previousGeometry = ''
let introduced = false
let revealFrame = 0
let revealStart = 0
let revealProgress = 1

function measure() {
  if (!strip.value) return
  const rect = strip.value.getBoundingClientRect()
  const style = getComputedStyle(strip.value)
  const width = parseFloat(style.width)
  if (!rect.width || !width) return
  const ratio = pixelRatio.value
  const colors = {
    up: style.getPropertyValue('--pulse-ok').trim() || style.getPropertyValue('--heartbeat').trim(),
    down: style.getPropertyValue('--pulse-down').trim() || style.getPropertyValue('--heartbeat-down').trim(),
    empty: style.getPropertyValue('--pulse-empty').trim() || style.getPropertyValue('--heartbeat-empty').trim(),
  }
  const geometry = [rect.left, rect.width, width, ratio, ...Object.values(colors)].join(',')
  if (geometry === previousGeometry) return
  const scale = rect.width / width
  const unit = scale * ratio
  const start = Math.ceil(rect.left * ratio)
  const end = Math.floor(rect.right * ratio)
  const bar = Math.max(1, Math.round(3 * unit))
  const gap = bar
  const count = Math.max(0, Math.min(1000, Math.floor((end - start + gap) / (bar + gap))))
  const first = end - start - count * bar - Math.max(0, count - 1) * gap
  return () => {
    previousGeometry = geometry
    palette = colors
    const previousCount = layout.value.count
    // 画布边缘和细条都对齐物理像素；最新一条始终贴近右侧。
    layout.value = { left: (start / ratio - rect.left) / scale, width: (end - start) / unit, pixels: end - start, height: Math.max(1, Math.round(20 * unit)), first, bar, gap, count }
    activeIndex.value = undefined
    if (count !== previousCount) emit('capacity', count)
  }
}

function align() { scheduleHeartbeatLayout(measure) }
useResizeObserver(strip, align)
useEventListener(window, 'resize', align)
watch([pixelRatio, dark], align, { flush: 'post' })
useIntersectionObserver(strip, ([entry]) => { inView.value = !!entry?.isIntersecting })

function drawBar(context: CanvasRenderingContext2D, index: number, active = false) {
  const { first, bar, gap, height } = layout.value
  const beat = slots.value[index]
  let color = palette.empty
  if (beat?.status === 'up') color = palette.up
  if (beat?.status === 'down') color = palette.down
  const unit = height / 20
  let barHeight = 14 * unit
  if (active) barHeight *= 1.25
  context.fillStyle = color
  context.beginPath()
  context.roundRect(first + index * (bar + gap), (height - barHeight) / 2, bar, barHeight, unit)
  context.fill()
}

function paint() {
  const element = canvas.value
  const context = element?.getContext('2d')
  if (!context || !layout.value.pixels) return
  context.clearRect(0, 0, element!.width, element!.height)
  context.save()
  context.globalAlpha = revealProgress < 1 ? .5 : 1
  context.drawImage(bitmap, 0, 0)
  context.restore()
  if (revealProgress < 1) {
    const ease = 1 - (1 - revealProgress) ** 3
    const edge = -64 + (layout.value.width + 64) * ease
    const ratio = layout.value.pixels / layout.value.width
    context.save()
    context.beginPath()
    context.moveTo(0, 0)
    context.lineTo(edge * ratio, 0)
    context.lineTo((edge + 64) * ratio, bitmap.height)
    context.lineTo(0, bitmap.height)
    context.closePath()
    context.clip()
    context.drawImage(bitmap, 0, 0)
    context.restore()
  }
  if (activeIndex.value !== undefined) drawBar(context, activeIndex.value, true)
}

function rebuild() {
  if (!canvas.value || !layout.value.pixels) return
  // 静态心跳只绘制一次；入场帧复用位图，避免逐帧重画所有细条。
  bitmap.width = layout.value.pixels
  bitmap.height = layout.value.height
  canvas.value.width = bitmap.width
  canvas.value.height = bitmap.height
  const context = bitmap.getContext('2d')
  if (!context) return
  for (let index = 0; index < slots.value.length; index++) drawBar(context, index)
  paint()
}
watch([layout, () => props.beats], rebuild, { flush: 'post' })
watch(activeIndex, paint, { flush: 'post' })

function stopIntro() {
  cancelAnimationFrame(revealFrame)
  revealFrame = 0
  revealProgress = 1
  paint()
}
function reveal(time: number) {
  const duration = Math.min(1200, Math.max(800, slotCount.value * 6 + 200))
  revealProgress = Math.max(0, Math.min(1, (time - revealStart) / duration))
  paint()
  if (revealProgress < 1) revealFrame = requestAnimationFrame(reveal)
  else revealFrame = 0
}
watch([slotCount, inView, visibility, reducedMotion], ([count, visible, pageVisibility, motion]) => {
  if (motion === 'reduce') { introduced = true; stopIntro(); return }
  if (!visible || pageVisibility !== 'visible') { stopIntro(); return }
  if (!count || introduced) return
  introduced = true
  revealStart = performance.now() + 120
  revealProgress = 0
  paint()
  revealFrame = requestAnimationFrame(reveal)
}, { flush: 'post' })

function selectPoint(event: PointerEvent) {
  const { pixels, width, first, bar, gap, count } = layout.value
  if (!count || !width) return
  // 指针坐标直接换算为记录索引，不扫描元素，也不在移动时重新测量布局。
  const x = event.offsetX * pixels / width
  activeIndex.value = Math.max(0, Math.min(count - 1, Math.floor((x - first + gap / 2) / (bar + gap))))
}
function clearPoint(event: PointerEvent) {
  if (event.pointerType !== 'touch') activeIndex.value = undefined
}
const activeLabel = computed(() => {
  if (activeIndex.value === undefined) return ''
  const beat = slots.value[activeIndex.value]
  if (!beat) return t('common.noData')
  return t('status.heartbeatPoint', { time: formatTime(beat.time), status: t(`state.${beat.status}`), latency: latency(beat.latency) })
})
const tooltipStyle = computed(() => {
  const index = activeIndex.value ?? 0
  const { left, pixels, width, first, bar, gap, count } = layout.value
  const ratio = pixels / width
  let x = first + index * (bar + gap) + bar / 2
  let shift = '-50%'
  if (index < 12) { x -= bar / 2; shift = '0%' }
  else if (index >= count - 12) { x += bar / 2; shift = '-100%' }
  return { left: `${left + x / ratio}px`, '--tooltip-x': shift }
})
useEventListener(document, 'pointerdown', (event) => {
  if (!strip.value?.contains(event.target as Node)) activeIndex.value = undefined
})
onBeforeUnmount(() => {
  cancelHeartbeatLayout(measure)
  cancelAnimationFrame(revealFrame)
})
</script>

<template>
  <div ref="strip" class="heartbeat-strip" role="img" :aria-label="t('status.heartbeat', { count: slotCount })">
    <canvas ref="canvas" class="heartbeat-canvas" :style="{ left: `${layout.left}px`, width: `${layout.width}px` }" aria-hidden="true" @pointermove="selectPoint" @pointerdown="selectPoint" @pointerleave="clearPoint" @pointercancel="activeIndex = undefined" />
    <span v-if="activeIndex !== undefined" class="heartbeat-tooltip" :style="tooltipStyle">{{ activeLabel }}</span>
  </div>
</template>
