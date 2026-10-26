<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import { useResizeObserver } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import type { HeartbeatPoint, LatencyBucket, LatencyHistory } from '@/types'
import { timezone } from '@/composables/useApp'
import { formatTime, latency } from '@/lib/format'

const props = defineProps<{ beats?: HeartbeatPoint[]; history?: LatencyHistory; interval?: number; loading?: boolean }>()
const { t, locale } = useI18n()
const helpId = `latency-help-${useId()}`
const plot = ref<HTMLElement>()
const width = ref(900)
const activeIndex = ref<number>()
const height = 88
const top = 8
const bottom = 80
const inset = 6

useResizeObserver(plot, ([entry]) => {
  if (entry && entry.contentRect.width > 0) width.value = entry.contentRect.width
})
watch(() => [props.beats, props.history], () => { activeIndex.value = undefined })

type ChartRecord = HeartbeatPoint & { bucket?: LatencyBucket }
const records = computed<ChartRecord[]>(() => {
  if (props.history) return props.history.buckets.map((bucket) => ({
    time: (bucket.start + bucket.end) / 2,
    status: bucket.failures ? 'down' : bucket.successes ? 'up' : 'unknown',
    latency: bucket.average ?? -1,
    bucket,
  }))
  return (props.beats || []).filter((beat) => Number.isFinite(beat.time) && beat.time > 0)
})
const recordCount = computed(() => props.history?.total ?? records.value.length)
function successful(beat: ChartRecord) {
  if (beat.bucket) return beat.bucket.successes > 0
  return beat.status === 'up' && Number.isFinite(beat.latency) && beat.latency >= 0
}
function measured(beat: ChartRecord) {
  if (beat.bucket) return beat.bucket.successes > 0
  if (!Number.isFinite(beat.latency) || beat.latency < 0) return false
  return beat.status === 'up' || (beat.status === 'down' && beat.failure === 'response')
}
const failureCount = computed(() => props.history?.failures ?? records.value.filter((beat) => beat.status === 'down').length)
const statistics = computed(() => {
  if (props.history) return props.history.statistics
  const values = records.value.filter(successful).map((beat) => beat.latency).sort((a, b) => a - b)
  if (!values.length) return null
  return {
    average: values.reduce((sum, value) => sum + value, 0) / values.length,
    p95: values[Math.ceil(values.length * 0.95) - 1]!,
    maximum: values[values.length - 1]!,
  }
})

interface ChartPoint {
  beat: ChartRecord
  index: number
  x: number
  y: number
  maximumY: number
  minimumY: number
  successful: boolean
  measured: boolean
  recovered: boolean
}
interface FailureSpan {
  first: ChartPoint
  last: ChartPoint
  x: number
  width: number
  count: number
}
const chart = computed(() => {
  const start = props.history?.start ?? records.value[0]?.time ?? 0
  const end = props.history?.end ?? records.value[records.value.length - 1]?.time ?? start
  const maximum = records.value.filter(measured).reduce((value, beat) => Math.max(value, beat.bucket?.maximum ?? beat.latency), 0)
  const ceiling = Math.max(1, maximum * 1.15)
  const yFor = (value: number) => bottom - value / ceiling * (bottom - top)
  const xFor = (time: number) => inset + (time - start) / (end - start) * (width.value - inset * 2)
  // 短范围的时间段可能小于检测间隔，正常调度间隙仍连线，超过间隔的缺口断线。
  const contiguous = (a: ChartPoint, b: ChartPoint) => !a.beat.bucket || !b.beat.bucket || b.beat.bucket.start - a.beat.bucket.end <= (props.interval ?? 0) * 1500
  const points = records.value.map((beat, index): ChartPoint => {
    let x = width.value / 2
    if (end > start) x = inset + (beat.time - start) / (end - start) * (width.value - inset * 2)
    let y = bottom
    if (measured(beat)) y -= beat.latency / ceiling * (bottom - top)
    return { beat, index, x, y, maximumY: yFor(beat.bucket?.maximum ?? beat.latency), minimumY: yFor(beat.bucket?.minimum ?? beat.latency), successful: successful(beat), measured: measured(beat), recovered: !beat.bucket && successful(beat) && records.value[index - 1]?.status === 'down' }
  })
  // 无已确认的响应延迟或数据缺失处断线；已确认的异常响应保留延迟并单独着色。
  const segments: ChartPoint[][] = []
  let segment: ChartPoint[] = []
  for (const point of points) {
    // 长时间未采集的历史断线，不延伸至整个请求范围。
    if (segment.length && !contiguous(segment[segment.length - 1]!, point)) { segments.push(segment); segment = [] }
    if (point.measured) segment.push(point)
    else if (segment.length) { segments.push(segment); segment = [] }
  }
  if (segment.length) segments.push(segment)
  const paths = segments.map((items) => items.map((point, index) => {
    let command = 'L'
    if (index === 0) command = 'M'
    return `${command}${point.x.toFixed(2)} ${point.y.toFixed(2)}`
  }).join(' '))
  const areas = segments.flatMap((items, index) => {
    if (items.length < 2) return []
    return [`${paths[index]} L${items[items.length - 1]!.x.toFixed(2)} ${bottom} L${items[0]!.x.toFixed(2)} ${bottom} Z`]
  })
  const marks = (items: ChartPoint[]) => items.map((point) => `M${point.x.toFixed(2)} ${bottom - 2}v4`).join(' ')
  const spans: FailureSpan[] = []
  for (let index = 0; index < points.length; index++) {
    const first = points[index]!
    if (first.beat.status !== 'down' || (!first.beat.bucket && first.measured)) continue
    let endIndex = index
    while (endIndex + 1 < points.length) {
      const next = points[endIndex + 1]!
      if (next.beat.status !== 'down' || (!next.beat.bucket && next.measured) || next.beat.failure !== first.beat.failure || !contiguous(points[endIndex]!, next)) break
      endIndex++
    }
    const last = points[endIndex]!
    let left = inset
    let right = width.value - inset
    if (index > 0) left = (points[index - 1]!.x + first.x) / 2
    if (endIndex + 1 < points.length) right = (last.x + points[endIndex + 1]!.x) / 2
    if (start === end) { left = first.x - 1; right = first.x + 1 }
    if (first.beat.bucket && last.beat.bucket) { left = xFor(first.beat.bucket.start); right = xFor(last.beat.bucket.end) }
    // 区段边界取相邻采样中点，表达记录覆盖范围，不推断精确故障起止时刻。
    spans.push({ first, last, x: left, width: Math.max(2, right - left), count: points.slice(index, endIndex + 1).reduce((sum, point) => sum + (point.beat.bucket?.failures ?? 1), 0) })
    index = endIndex
  }
  const abnormal = points.flatMap((point, index) => {
    const previous = points[index - 1]
    if (!previous?.measured || !point.measured || !contiguous(previous, point) || (point.beat.status !== 'down' && previous.beat.status !== 'down')) return []
    return [`M${previous.x.toFixed(2)} ${previous.y.toFixed(2)}L${point.x.toFixed(2)} ${point.y.toFixed(2)}`]
  }).join(' ')
  return {
    points, start, end, ceiling, spans, abnormal, line: paths.join(' '), area: areas.join(' '),
    responses: points.filter((point) => point.beat.status === 'down' && point.measured),
    recoveries: points.filter((point) => point.recovered),
    hasMeasurements: points.some((point) => point.measured),
    missing: marks(points.filter((point) => !point.successful && point.beat.status !== 'down')),
    isolated: segments.filter((items) => items.length === 1 && items[0]!.successful).map((items) => items[0]!),
    maximum: points.find((point) => point.successful && (point.beat.bucket?.maximum ?? point.beat.latency) === statistics.value?.maximum),
    latest: points[points.length - 1],
  }
})
function createClock(seconds: boolean) {
  const options: Intl.DateTimeFormatOptions = { timeZone: timezone.value, hour: '2-digit', minute: '2-digit', hour12: false }
  if (seconds) options.second = '2-digit'
  if (props.history && props.history.end - props.history.start > 86400000) { options.month = '2-digit'; options.day = '2-digit' }
  try { return new Intl.DateTimeFormat(locale.value, options) }
  catch { return new Intl.DateTimeFormat(locale.value, { ...options, timeZone: 'UTC' }) }
}
const clock = computed(() => createClock(!props.history))
const bucketClock = computed(() => createClock(true))
const axis = computed(() => {
  if (chart.value.start === chart.value.end) return [chart.value.start]
  return [chart.value.start, (chart.value.start + chart.value.end) / 2, chart.value.end]
})
const selected = computed(() => {
  if (activeIndex.value === undefined) return undefined
  return chart.value.points[activeIndex.value]
})
const selectedSpan = computed(() => {
  const point = selected.value
  if (!point) return undefined
  return chart.value.spans.find((span) => point.index >= span.first.index && point.index <= span.last.index)
})
function annotationLeft(x: number) {
  const margin = Math.min(props.history ? 145 : 108, width.value / 2)
  return Math.max(margin, Math.min(width.value - margin, x))
}
const annotations = computed(() => {
  // 仅展示能容纳的区段标注，其余故障仍可通过指针、触摸和键盘查看。
  const items: { span: FailureSpan; left: number }[] = []
  const candidates = [...chart.value.spans].sort((a, b) => b.width - a.width || b.count - a.count)
  for (const span of candidates) {
    const left = annotationLeft(span.x + span.width / 2)
    if (items.some((item) => Math.abs(item.left - left) < 224)) continue
    items.push({ span, left })
    if (width.value < 520 || items.length === 2) break
  }
  return items
})
const tooltipLeft = computed(() => {
  const span = selectedSpan.value
  if (span && !selected.value?.beat.bucket) return annotationLeft(span.x + span.width / 2)
  return annotationLeft(selected.value?.x || 0)
})
function spanTime(span: FailureSpan) {
  if (span.first.beat.bucket && span.last.beat.bucket) return `${bucketClock.value.format(span.first.beat.bucket.start)} – ${bucketClock.value.format(span.last.beat.bucket.end)}`
  const first = clock.value.format(span.first.beat.time)
  if (span.first.beat.time === span.last.beat.time) return first
  return `${first} – ${clock.value.format(span.last.beat.time)}`
}
function failureLabel(beat: HeartbeatPoint) {
  if (beat.failure === 'timeout') {
    if (Number.isFinite(beat.latency) && beat.latency > 0) return t('status.latencyTimeout', { seconds: Math.round(beat.latency / 100) / 10 })
    return t('status.latencyTimedOut')
  }
  if (beat.failure === 'refused') return t('status.latencyRefused')
  if (beat.failure === 'response') {
    if (beat.statusCode) return `HTTP ${beat.statusCode}`
    return t('status.latencyAbnormalResponse')
  }
  return t('state.down')
}
const selectedLabel = computed(() => {
  const point = selected.value
  if (!point) return ''
  if (point.beat.status === 'down') return failureLabel(point.beat)
  if (point.recovered) return t('status.latencyRecovered')
  return t(`state.${point.beat.status}`)
})

function selectPoint(event: MouseEvent | PointerEvent) {
  if (!plot.value || !chart.value.points.length) return
  const x = event.clientX - plot.value.getBoundingClientRect().left
  // 根据真实时间位置二分定位，移动指针时不遍历全部历史记录。
  const points = chart.value.points
  let left = 0
  let right = points.length - 1
  while (left < right) {
    const middle = Math.floor((left + right) / 2)
    if (points[middle]!.x < x) left = middle + 1
    else right = middle
  }
  if (left > 0 && Math.abs(points[left - 1]!.x - x) < Math.abs(points[left]!.x - x)) left--
  activeIndex.value = left
}
function moveSelection(event: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const last = chart.value.points.length - 1
  let next = activeIndex.value ?? last
  if (event.key === 'ArrowLeft') next--
  if (event.key === 'ArrowRight') next++
  if (event.key === 'Home') next = 0
  if (event.key === 'End') next = last
  activeIndex.value = Math.max(0, Math.min(last, next))
}
</script>

<template>
  <section class="pulse-latency-chart" :class="{ 'pulse-latency-range': history }" :aria-label="t('status.latencyHistory')" :aria-busy="loading">
    <div class="pulse-latency-heading">
      <div class="pulse-latency-title">
        <h3>{{ t('status.latencyHistory') }}</h3>
        <span v-if="!loading">{{ t(history ? 'status.rangeChecks' : 'status.heartbeat', { count: recordCount }) }}</span>
      </div>
      <slot name="controls" />
      <dl v-if="!loading && (statistics || failureCount)" class="pulse-latency-statistics pulse-mono" :title="t(history ? 'status.rangeStatisticsHint' : 'status.latencyStatisticsHint')">
        <template v-if="statistics">
          <div><dt>{{ t('status.historyAverage') }}</dt><dd>{{ latency(statistics.average) }}</dd></div>
          <div><dt><abbr :title="t('status.latencyP95Hint')">P95</abbr></dt><dd>{{ latency(statistics.p95) }}</dd></div>
          <div><dt>{{ t('status.historyMaximum') }}</dt><dd>{{ latency(statistics.maximum) }}</dd></div>
        </template>
        <div v-if="failureCount" class="pulse-latency-failure-count"><dt>{{ t('status.historyFailures') }}</dt><dd>{{ t('status.failureChecks', { count: failureCount }) }}</dd></div>
      </dl>
    </div>
    <p v-if="loading" class="pulse-latency-empty pulse-latency-no-data" role="status">{{ t('common.loading') }}</p>
    <template v-else-if="records.length || history">
      <p :id="helpId" class="sr-only">{{ t(history ? 'status.rangeChartHelp' : 'status.latencyChartHelp') }}</p>
      <div
        ref="plot" class="pulse-latency-plot" tabindex="0" role="group"
        :aria-label="t('status.latencyHistory')" :aria-describedby="helpId"
        @pointermove="selectPoint" @click="selectPoint" @pointerleave="activeIndex = undefined"
        @focus="activeIndex = chart.points.length - 1" @blur="activeIndex = undefined" @keydown="moveSelection"
      >
        <svg :viewBox="`0 0 ${width} ${height}`" role="img" :aria-label="t('status.latencyChartSummary', { count: recordCount, start: formatTime(chart.start), end: formatTime(chart.end) })">
          <path :d="`M${inset} 24H${width - inset} M${inset} 52H${width - inset}`" class="pulse-latency-grid" />
          <g v-for="span in chart.spans" :key="span.first.index" class="pulse-latency-outage">
            <rect :x="span.x" y="0" :width="span.width" :height="bottom" class="pulse-latency-outage-area" />
            <rect :x="span.x" :y="bottom - 2" :width="span.width" height="2" class="pulse-latency-outage-edge" />
            <path v-if="span.first.beat.failure === 'timeout'" :d="`M${Math.max(inset, Math.min(width - inset - 9, span.first.x - 4.5))} 1.5h9l-4.5 7z`" class="pulse-latency-timeout" />
          </g>
          <path :d="chart.area" class="pulse-latency-area" />
          <path v-for="point in chart.points.filter((point) => point.beat.bucket && point.measured)" :key="point.index" :d="`M${point.x} ${point.maximumY}V${point.minimumY}`" class="pulse-latency-envelope" />
          <path :d="chart.line" class="pulse-latency-line" />
          <path :d="chart.abnormal" class="pulse-latency-abnormal" />
          <path :d="chart.missing" class="pulse-latency-missing" />
          <circle v-for="(point, index) in chart.isolated" :key="index" :cx="point.x" :cy="point.y" r="2" class="pulse-latency-single" />
          <circle v-if="chart.maximum && (history || chart.maximum !== chart.latest)" :cx="chart.maximum.x" :cy="chart.maximum.maximumY" r="3.5" class="pulse-latency-maximum" />
          <circle v-for="point in chart.responses" :key="point.index" :cx="point.x" :cy="point.y" r="3.5" class="pulse-latency-response" />
          <circle v-for="point in chart.recoveries" :key="point.index" :cx="point.x" :cy="point.y" r="3.5" class="pulse-latency-recovered" />
          <circle v-if="chart.latest?.successful && !chart.latest.recovered" :cx="chart.latest.x" :cy="chart.latest.y" r="4" class="pulse-latency-latest" />
          <template v-if="selected">
            <path :d="`M${selected.x} ${top}V${bottom}`" class="pulse-latency-guide" />
            <circle :cx="selected.x" :cy="selected.y" r="4" :class="['pulse-latency-selected', `pulse-latency-selected-${selected.beat.status}`, { 'pulse-latency-recovered': selected.recovered }]" />
          </template>
        </svg>
        <template v-if="!selected">
          <div v-for="annotation in annotations" :key="annotation.span.first.index" class="pulse-latency-tooltip pulse-latency-annotation pulse-mono" :style="{ left: `${annotation.left}px` }">
            <span :title="`${formatTime(annotation.span.first.beat.time)} – ${formatTime(annotation.span.last.beat.time)}`">{{ spanTime(annotation.span) }}</span>
            <span class="pulse-latency-fault-label">{{ failureLabel(annotation.span.first.beat) }} · {{ t('status.failureChecks', { count: annotation.span.count }) }}</span>
          </div>
        </template>
        <p v-if="!chart.hasMeasurements" class="pulse-latency-empty">{{ t(records.length ? 'status.latencyNoSuccess' : 'status.latencyNoData') }}</p>
        <div v-if="selected" class="pulse-latency-tooltip pulse-mono" :style="{ left: `${tooltipLeft}px` }" role="status" aria-live="polite">
          <template v-if="selected.beat.bucket">
            <span :title="`${formatTime(selected.beat.bucket.start)} – ${formatTime(selected.beat.bucket.end)}`">{{ bucketClock.format(selected.beat.bucket.start) }} – {{ bucketClock.format(selected.beat.bucket.end) }}</span>
            <span>{{ t('status.historyAverage') }} {{ latency(selected.beat.bucket.average) }} · {{ t('status.historyMinimum') }} {{ latency(selected.beat.bucket.minimum) }} · {{ t('status.historyMaximum') }} {{ latency(selected.beat.bucket.maximum) }}</span>
            <span :class="{ 'pulse-latency-fault-label': selected.beat.bucket.failures }">{{ t('status.bucketChecks', { count: selected.beat.bucket.count, failures: selected.beat.bucket.failures }) }}</span>
          </template>
          <template v-else>
          <span v-if="selectedSpan" :title="`${formatTime(selectedSpan.first.beat.time)} – ${formatTime(selectedSpan.last.beat.time)}`">{{ spanTime(selectedSpan) }}</span>
          <time v-else :datetime="new Date(selected.beat.time).toISOString()">{{ formatTime(selected.beat.time) }}</time>
          <span :class="{ 'pulse-latency-fault-label': selected.beat.status === 'down' }">
            {{ selectedLabel }} ·
            <template v-if="selectedSpan">{{ t('status.failureChecks', { count: selectedSpan.count }) }}</template>
            <template v-else>{{ latency(selected.beat.latency) }}</template>
          </span>
          </template>
        </div>
      </div>
      <div class="pulse-latency-axis pulse-mono" :class="{ 'pulse-latency-axis-single': axis.length === 1 }">
        <time v-for="time in axis" :key="time" :datetime="new Date(time).toISOString()" :title="formatTime(time)">{{ clock.format(time) }}</time>
      </div>
    </template>
    <p v-else class="pulse-latency-empty pulse-latency-no-data">{{ t('status.latencyNoData') }}</p>
    <p v-if="history && !loading" class="pulse-latency-note">{{ t('status.rangeChartHint') }}<template v-if="history.availableStart > history.start"> {{ t('status.rangeRetentionHint', { start: formatTime(history.availableStart) }) }}</template></p>
  </section>
</template>

<style scoped>
.pulse-latency-chart {
  /* 状态页沿用自身配色；后台使用共用主题变量。 */
  --latency-warning: #92400e;
  margin-top: var(--pulse-service-gap, 20px);
  border-top: 1px solid var(--pulse-border, var(--border));
  padding: 14px 0;
}
.pulse-latency-heading,
.pulse-latency-title,
.pulse-latency-statistics,
.pulse-latency-statistics > div {
  display: flex;
  align-items: baseline;
}
.pulse-latency-heading {
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px 20px;
  margin-bottom: 6px;
}
.pulse-latency-title {
  flex-wrap: wrap;
  gap: 4px 10px;
}
.pulse-latency-title h3 {
  margin: 0;
  font-size: 12px;
  font-weight: 500;
}
.pulse-latency-title > span {
  color: var(--pulse-muted, var(--muted-foreground));
  font-size: 11px;
}
.pulse-latency-statistics {
  flex-wrap: wrap;
  gap: 6px 16px;
  margin: 0;
  font-size: 12px;
}
.pulse-latency-statistics > div {
  gap: 6px;
}
.pulse-latency-statistics dt {
  color: var(--pulse-muted, var(--muted-foreground));
}
.pulse-latency-statistics dd {
  margin: 0;
}
.pulse-latency-statistics abbr {
  text-decoration: none;
}
.pulse-latency-plot {
  position: relative;
  min-width: 0;
  /* 为两行提示预留固定空间，避免悬停时遮点或改变布局。 */
  padding-top: 56px;
  border-radius: 4px;
}
.pulse-latency-plot > svg {
  display: block;
  width: 100%;
  height: 88px;
}
.pulse-latency-grid {
  fill: none;
  stroke: var(--pulse-border, var(--border));
  stroke-dasharray: 3 5;
}
.pulse-latency-area {
  fill: var(--pulse-ok, var(--heartbeat));
  fill-opacity: 0.09;
}
.pulse-latency-line {
  fill: none;
  stroke: var(--pulse-ok, var(--heartbeat));
  stroke-width: 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.pulse-latency-envelope {
  stroke: var(--pulse-ok, var(--heartbeat));
  stroke-opacity: 0.4;
  stroke-width: 1;
}
.pulse-latency-range .pulse-latency-plot { padding-top: 86px; }
.pulse-latency-range .pulse-latency-empty { inset-block-start: 86px; }
.pulse-latency-range .pulse-latency-tooltip { width: max-content; max-width: min(290px, calc(100% - 16px)); white-space: normal; }
.pulse-latency-note {
  margin: 10px 0 0;
  color: var(--pulse-muted, var(--muted-foreground));
  font-size: 11px;
}
.pulse-latency-missing {
  fill: none;
  stroke: var(--pulse-down, var(--heartbeat-down));
  stroke-width: 3;
  stroke-linecap: round;
}
.pulse-latency-missing {
  stroke: var(--pulse-muted, var(--muted-foreground));
}
.pulse-latency-single {
  fill: var(--pulse-ok, var(--heartbeat));
}
.pulse-latency-maximum,
.pulse-latency-latest,
.pulse-latency-selected {
  stroke: var(--pulse-surface, var(--card));
  stroke-width: 1.5;
}
.pulse-latency-maximum {
  fill: var(--pulse-warning, var(--latency-warning));
}
.pulse-latency-latest,
.pulse-latency-selected-up {
  fill: var(--pulse-ok, var(--heartbeat));
}
.pulse-latency-selected-down {
  fill: var(--pulse-down, var(--heartbeat-down));
}
.pulse-latency-selected-unknown,
.pulse-latency-selected-paused {
  fill: var(--pulse-muted, var(--muted-foreground));
}
.pulse-latency-guide {
  fill: none;
  stroke: var(--pulse-muted, var(--muted-foreground));
  stroke-dasharray: 3 4;
  opacity: 0.5;
}
.pulse-latency-tooltip {
  position: absolute;
  top: 0;
  display: grid;
  gap: 3px;
  max-width: calc(100% - 16px);
  padding: 7px 10px;
  border: 1px solid var(--pulse-border, var(--border));
  border-radius: 6px;
  background: var(--pulse-pill, var(--muted));
  color: var(--pulse-text, var(--foreground));
  font-size: 11px;
  line-height: 1.4;
  white-space: nowrap;
  pointer-events: none;
  transform: translateX(-50%);
}
.pulse-latency-tooltip > span {
  font-weight: 500;
}
.pulse-latency-tooltip > time,
.pulse-latency-annotation > span:first-child {
  color: var(--pulse-muted, var(--muted-foreground));
}
.pulse-latency-tooltip .pulse-latency-fault-label,
.pulse-latency-failure-count dd {
  color: var(--pulse-down-text, var(--destructive));
}
.pulse-latency-outage-area {
  fill: var(--pulse-down, var(--heartbeat-down));
  fill-opacity: 0.13;
}
.pulse-latency-outage-edge,
.pulse-latency-timeout,
.pulse-latency-response {
  fill: var(--pulse-down, var(--heartbeat-down));
}
.pulse-latency-response {
  stroke: var(--pulse-surface, var(--card));
  stroke-width: 1.5;
}
.pulse-latency-abnormal {
  fill: none;
  stroke: var(--pulse-down, var(--heartbeat-down));
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.pulse-latency-recovered {
  fill: var(--pulse-surface, var(--card));
  stroke: var(--pulse-ok, var(--heartbeat));
  stroke-width: 1.6;
}
.pulse-latency-axis {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-top: 5px;
  color: var(--pulse-muted, var(--muted-foreground));
  font-size: 10.5px;
}
.pulse-latency-axis-single {
  justify-content: center;
}
.pulse-latency-empty {
  position: absolute;
  inset: 56px 0 0;
  display: grid;
  place-items: center;
  margin: 0;
  color: var(--pulse-muted, var(--muted-foreground));
  font-size: 12px;
  text-align: center;
  pointer-events: none;
}
.pulse-latency-no-data {
  position: static;
  min-height: 88px;
}
:global(.dark) .pulse-latency-chart {
  --latency-warning: #f5b94a;
}
.pulse-mono {
  font-family: 'Pulse Mono', monospace;
  font-variant-numeric: tabular-nums;
}
</style>
