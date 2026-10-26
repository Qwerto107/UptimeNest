<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { LayoutDashboard } from 'lucide-vue-next'
import type { PublicPanel, PublicMonitor, MonitorStatus } from '@/types'
import { request } from '@/lib/api'
import { bootstrap, loadBootstrap, siteName, timezone } from '@/composables/useApp'
import { useResource } from '@/composables/useResource'
import { useHeartbeatCount } from '@/composables/useHeartbeatCount'
import { formatTime, percent } from '@/lib/format'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { Button } from '@/components/ui/button'
import LoadState from '@/components/LoadState.vue'
import EmptyState from '@/components/EmptyState.vue'
import HeartbeatStrip from '@/components/HeartbeatStrip.vue'
import LatencyHistoryChart from '@/components/LatencyHistoryChart.vue'
import ConnectionDetails from '@/components/ConnectionDetails.vue'
import StatusConnectionMeta from '@/components/StatusConnectionMeta.vue'
import StatusSignal from '@/components/StatusSignal.vue'
import FailureHighlight from '@/components/FailureHighlight.vue'
import AnimatedValue from '@/components/AnimatedValue.vue'
import RefreshIcon from '@/components/RefreshIcon.vue'
import { useMotion } from '@/composables/useMotion'
import '@/styles/status.css'
import PaginationControls from '@/components/PaginationControls.vue'
import { usePagination } from '@/composables/usePagination'

const { t } = useI18n()
const route = useRoute()
const updatedAt = ref<number>()
const expanded = ref<number[]>([])
const pageElement = ref<HTMLElement>()
const { animate, reducedMotion } = useMotion()
const { heartbeatCount, resizeHeartbeat } = useHeartbeatCount(90, () => void refresh())

const { data, loading, error, refresh, refreshing, secondsUntilRefresh } = useResource(async (signal) => {
  const [value] = await Promise.all([
    request<PublicPanel>(
      '/public/panels/' +
        encodeURIComponent(String(route.params.slug)) +
        `?heartbeatCount=${heartbeatCount.value}`,
      {
        signal,
      },
    ),
    loadBootstrap(),
  ])
  timezone.value = value.timezone
  updatedAt.value = Date.now()
  return value
}, 30000)

const monitors = computed(() => data.value?.monitors || [])
const { page, pageSize, offset } = usePagination(computed(() => monitors.value.length), 25)
const pageMonitors = computed(() => monitors.value.slice(offset.value, offset.value + pageSize.value))
const overall = computed<MonitorStatus>(() => {
  if (!monitors.value.length) return 'unknown'
  if (monitors.value.some((monitor) => monitor.summary.status === 'down')) return 'down'
  if (monitors.value.some((monitor) => monitor.summary.status === 'unknown')) return 'unknown'
  // 停用只代表暂停检测，不影响状态面板的正常汇总。
  return 'up'
})

const statusCopy = computed(() => {
  if (!monitors.value.length)
    return { title: t('status.emptyTitle'), description: t('status.emptyDescription') }
  if (overall.value === 'down') {
    const count = monitors.value.filter((monitor) => monitor.summary.status === 'down').length
    return { title: t('status.someDown'), description: t('status.downDescription', { count }) }
  }
  if (overall.value === 'unknown')
    return { title: t('status.waiting'), description: t('status.waitingDescription') }
  return { title: t('status.allUp'), description: '' }
})

const groups = computed(() => {
  if (!data.value) return []
  const index = new Map(pageMonitors.value.map((monitor) => [monitor.id, monitor]))
  return data.value.groups
    .map((group) => ({
      id: group.id,
      name: group.name,
      monitors: group.monitorIds
        .map((id) => index.get(id))
        .filter((monitor): monitor is PublicMonitor => !!monitor),
    }))
    .filter((group) => group.monitors.length)
})

async function toggle(id: number) {
  // 只插值位置和卡片底板的缩放，正文保持原始比例，不动画化布局宽高。
  const elements = reducedMotion.value === 'reduce' ? [] : [...(pageElement.value?.querySelectorAll<HTMLElement>('.pulse-service, .pulse-group-heading, .pulse-list-surface, .list-pagination, .pulse-footer') || [])]
  const before = elements.map((element) => ({ element, rect: element.getBoundingClientRect() }))
  if (expanded.value.includes(id)) expanded.value = expanded.value.filter((value) => value !== id)
  else expanded.value.push(id)
  await nextTick()
  for (const { element, rect } of before) {
    if (!element.isConnected) continue
    const after = element.getBoundingClientRect()
    const dy = rect.top - after.top
    const scale = element.classList.contains('pulse-list-surface') && after.height ? rect.height / after.height : 1
    if (Math.abs(dy) < .5 && Math.abs(scale - 1) < .001) continue
    animate(element, [{ transform: `translateY(${dy}px) scaleY(${scale})` }, { transform: 'translateY(0) scaleY(1)' }], 260)
  }
}

function finish(animation: Animation | undefined, done: () => void) {
  if (animation) void animation.finished.then(done, done)
  else done()
}
function resetDetails(element: Element) {
  const style = (element as HTMLElement).style
  for (const property of ['position', 'top', 'left', 'width', 'margin-top', 'pointer-events']) style.removeProperty(property)
}
function enterDetails(element: Element, done: () => void) {
  resetDetails(element)
  finish(animate(element, [{ opacity: 0, transform: 'translateY(6px)' }, { opacity: 1, transform: 'translateY(0)' }], 200, 60), done)
}
function leaveDetails(element: Element, done: () => void) {
  const node = element as HTMLElement
  if (reducedMotion.value === 'reduce' || !node.parentElement) { done(); return }
  const rect = node.getBoundingClientRect()
  const parent = node.parentElement.getBoundingClientRect()
  // 离场内容临时脱离文档流，下方各行通过 FLIP 移回新位置。
  Object.assign(node.style, { position: 'absolute', top: `${rect.top - parent.top}px`, left: `${rect.left - parent.left}px`, width: `${rect.width}px`, marginTop: '0', pointerEvents: 'none' })
  finish(animate(node, [{ opacity: 1, transform: 'translateY(0)' }, { opacity: 0, transform: 'translateY(-4px)' }], 180), done)
}

function statusLabel(status: MonitorStatus) {
  if (status === 'up') return t('status.legendUp')
  return t('state.' + status)
}

function protocol(monitor: PublicMonitor) {
  if (monitor.protocol && monitor.type !== 'ping') return monitor.protocol
  return t('monitor.' + monitor.type)
}
</script>

<template>
  <div ref="pageElement" class="pulse-page">
    <div class="pulse-container">
      <header class="pulse-header">
        <RouterLink class="pulse-brand" to="/" :aria-label="t('common.home')">
          <svg width="28" height="28" viewBox="0 0 28 28" fill="none" aria-hidden="true">
            <rect
              x="1"
              y="1"
              width="26"
              height="26"
              rx="7"
              stroke="currentColor"
              stroke-width="1.5"
            />
            <path
              d="M5 15h5l3-7 4 12 3-5h3"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
          <span :title="data?.panel.pageTitle || siteName">
            {{ data?.panel.pageTitle || siteName }}
          </span>
        </RouterLink>
        <div class="pulse-tools">
          <ThemeToggle />
          <Button variant="ghost" size="icon" as-child>
            <RouterLink to="/admin" :aria-label="t('nav.admin')" :title="t('nav.admin')">
              <LayoutDashboard />
            </RouterLink>
          </Button>
        </div>
        <p v-if="data?.panel.description" class="pulse-panel-description">
          <span>{{ data.panel.description }}</span>
        </p>
      </header>

      <LoadState :loading="loading" :error="error" :has-data="!!data" skeleton="status" @retry="refresh" />

      <template v-if="data">
        <section class="pulse-summary" :class="'pulse-summary-' + overall" aria-live="polite">
          <div class="pulse-summary-main">
            <StatusSignal :status="overall" />
            <div>
              <h1 class="pulse-summary-title">{{ statusCopy.title }}</h1>
              <p class="pulse-updated">
                {{ t('status.lastUpdated') }}
                <time
                  class="pulse-mono"
                  :datetime="new Date(updatedAt || 0).toISOString()"
                  :title="formatTime(updatedAt)"
                >
                  {{ formatTime(updatedAt) }}
                </time>
              </p>
              <p v-if="statusCopy.description" class="pulse-status-description">
                {{ statusCopy.description }}
              </p>
            </div>
          </div>
          <div class="pulse-refresh-controls">
          <span class="pulse-next-refresh pulse-mono" :title="t('status.refreshCountdown', { seconds: secondsUntilRefresh })">{{ refreshing ? t('status.refreshing') : `${secondsUntilRefresh}s` }}</span>
          <button
            class="pulse-refresh"
            type="button"
            :disabled="refreshing"
            :aria-label="t('common.refresh')"
            :title="t('common.refresh')"
            @click="refresh"
          >
            <RefreshIcon :size="14" :refreshing="refreshing" />
          </button>
          </div>
        </section>

        <main class="pulse-main">
          <EmptyState
            v-if="!monitors.length"
            :title="t('status.emptyTitle')"
            :description="t('status.emptyDescription')"
          />
          <section v-for="group in groups" :key="group.id" class="pulse-group">
            <div class="pulse-group-heading">
              <h2>{{ group.name }}</h2>
            </div>
            <div class="pulse-service-list">
              <div class="pulse-list-surface" aria-hidden="true" />
              <article v-for="monitor in group.monitors" :key="monitor.id" class="pulse-service">
                <FailureHighlight :status="monitor.summary.status" />
                <div class="pulse-service-heading">
                  <div class="pulse-service-name-line">
                    <button
                      class="pulse-service-name"
                      :aria-label="t('status.showDetails', { name: monitor.name })"
                      :aria-expanded="expanded.includes(monitor.id)"
                      :aria-controls="'connection-' + monitor.id"
                      @click="toggle(monitor.id)"
                    >
                      {{ monitor.name }}
                    </button>
                    <span v-if="monitor.address" class="pulse-address pulse-mono">
                      {{ monitor.address }}
                    </span>
                    <span class="pulse-protocol pulse-mono">{{ protocol(monitor) }}</span>
                  </div>
                  <div class="pulse-service-result">
                    <span
                      class="pulse-uptime"
                      :title="t('status.uptimeShort')"
                      :aria-label="t('status.uptimeShort') + ' ' + percent(monitor.summary.uptime)"
                    >
                      <AnimatedValue class="pulse-mono" :value="percent(monitor.summary.uptime)" />
                    </span>
                    <span class="pulse-state" :class="'pulse-state-' + monitor.summary.status">
                      <StatusSignal :status="monitor.summary.status" />
                      {{ statusLabel(monitor.summary.status) }}
                    </span>
                  </div>
                </div>
                <p v-if="monitor.description" class="pulse-service-description">
                  {{ monitor.description }}
                </p>
                <div v-if="monitor.tags.length" class="pulse-tags">
                  <span v-for="tag in monitor.tags" :key="tag">{{ tag }}</span>
                </div>
                <StatusConnectionMeta :beat="monitor.summary.latest" :tls-latency-warning-ms="bootstrap?.tlsLatencyWarningMs ?? 0" />
                <HeartbeatStrip :beats="monitor.summary.heartbeats" @capacity="resizeHeartbeat" />
                <Transition :css="false" @enter="enterDetails" @leave="leaveDetails" @leave-cancelled="resetDetails">
                  <div
                    v-if="expanded.includes(monitor.id)"
                    :id="'connection-' + monitor.id"
                    class="pulse-expanded"
                  >
                    <div class="pulse-expanded-content">
                      <LatencyHistoryChart :beats="monitor.summary.heartbeats" />
                      <ConnectionDetails :beat="monitor.summary.latest" />
                    </div>
                  </div>
                </Transition>
              </article>
            </div>
          </section>

          <PaginationControls v-model:page="page" v-model:pageSize="pageSize" :total="monitors.length" :label="t('pagination.services')" />
        </main>

        <footer class="pulse-footer">
          <span>{{ t('status.copyright') }}</span>
        </footer>
      </template>
    </div>
  </div>
</template>
