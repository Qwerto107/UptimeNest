<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronDown, LayoutDashboard, LayoutGrid, List, Rows3 } from 'lucide-vue-next'
import type { PublicPanel, PublicMonitor, MonitorStatus, StatusPageView } from '@/types'
import { request } from '@/lib/api'
import { bootstrap, loadBootstrap, siteName, timezone } from '@/composables/useApp'
import { useResource } from '@/composables/useResource'
import { useHeartbeatCount } from '@/composables/useHeartbeatCount'
import { formatTime, latency, percent } from '@/lib/format'
import { statusViewItems } from '@/lib/status-views'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { Button } from '@/components/ui/button'
import LoadState from '@/components/LoadState.vue'
import EmptyState from '@/components/EmptyState.vue'
import HeartbeatStrip from '@/components/HeartbeatStrip.vue'
import StatusServiceDetails from '@/components/StatusServiceDetails.vue'
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
// 访客选择只存在于当前页面内存中，自动刷新数据时保留，重新载入时回到管理员默认值。
const temporaryView = ref<StatusPageView>()
const view = computed(() => temporaryView.value ?? bootstrap.value?.statusPageView ?? 'detailed')
const cardWide = useMediaQuery('(min-width: 1000px)')
const cardMedium = useMediaQuery('(min-width: 680px)')
const cardColumns = computed(() => cardWide.value ? 3 : cardMedium.value ? 2 : 1)
const viewChoices = [{ value: 'compact', icon: List }, { value: 'detailed', icon: Rows3 }, { value: 'cards', icon: LayoutGrid }] as const
watch(() => route.params.slug, () => { temporaryView.value = undefined })
// 卡片详情互斥；从其他视图切回来时也只保留最后展开的一项。
watch(view, (value) => { if (value === 'cards') expanded.value = expanded.value.slice(-1) })
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
  else if (view.value === 'cards') expanded.value = [id]
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
  <div ref="pageElement" class="pulse-page" :data-view="view">
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

      <LoadState :loading="loading" :error="error" :has-data="!!data" skeleton="status" :status-view="view" :status-columns="cardColumns" @retry="refresh" />

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
          <section v-for="(group, groupIndex) in groups" :key="group.id" class="pulse-group">
            <div class="pulse-group-heading">
              <h2>{{ group.name }}</h2>
              <div v-if="groupIndex === 0" class="pulse-view-switch" role="group" :aria-label="t('status.switchView')" :title="t('status.viewTemporaryHint')" :style="{ '--view-index': viewChoices.findIndex(choice => choice.value === view) }">
                <span class="pulse-view-indicator" aria-hidden="true" />
                <button v-for="choice in viewChoices" :key="choice.value" type="button" class="pulse-view-button" :aria-pressed="view === choice.value" :aria-label="t(`status.views.${choice.value}`)" :title="t(`status.views.${choice.value}`)" @click="temporaryView = choice.value"><component :is="choice.icon" aria-hidden="true" /></button>
              </div>
            </div>
            <div class="pulse-service-list" :class="{ 'pulse-card-grid': view === 'cards' }" :style="{ '--status-columns': cardColumns }">
              <div v-if="view !== 'cards'" class="pulse-list-surface" aria-hidden="true" />
              <template v-for="{ key, monitor, details, column } in statusViewItems(group.monitors, view, cardColumns)" :key="key">
                <StatusServiceDetails v-if="details" :monitor="monitor" :open="expanded.includes(monitor.id)" :column="column" condensed grid @capacity="resizeHeartbeat" />
                <article v-else class="pulse-service" :class="`pulse-service-${view}`" :data-expanded="expanded.includes(monitor.id)" @click="view !== 'detailed' && toggle(monitor.id)">
                  <FailureHighlight :status="monitor.summary.status" />
                  <div class="pulse-service-heading">
                    <div class="pulse-service-name-line">
                      <StatusSignal v-if="view !== 'detailed'" :status="monitor.summary.status" />
                      <button
                        class="pulse-service-name"
                        :title="monitor.name"
                        :aria-label="t('status.showDetails', { name: monitor.name }) + ' · ' + statusLabel(monitor.summary.status)"
                        :aria-expanded="expanded.includes(monitor.id)"
                        :aria-controls="'connection-' + monitor.id"
                        @click.stop="toggle(monitor.id)"
                      >
                        {{ monitor.name }}
                      </button>
                      <span v-if="monitor.address" class="pulse-address pulse-mono" :title="monitor.address">
                        {{ monitor.address }}
                      </span>
                      <span v-if="view === 'detailed'" class="pulse-protocol pulse-mono">{{ protocol(monitor) }}</span>
                    </div>
                    <div class="pulse-service-result">
                      <span
                        class="pulse-uptime"
                        :title="t('status.uptimeShort')"
                        :aria-label="t('status.uptimeShort') + ' ' + percent(monitor.summary.uptime)"
                      >
                        <AnimatedValue class="pulse-mono" :value="percent(monitor.summary.uptime)" />
                      </span>
                      <span v-if="view !== 'detailed'" class="pulse-service-latency pulse-mono" :title="t('status.latestLatency')">{{ monitor.summary.status === 'up' ? latency(monitor.summary.latest?.latency) : '—' }}</span>
                      <span v-if="view === 'detailed' || monitor.summary.status !== 'up'" class="pulse-state" :class="'pulse-state-' + monitor.summary.status">
                        <StatusSignal :status="monitor.summary.status" />
                        {{ statusLabel(monitor.summary.status) }}
                      </span>
                    </div>
                  </div>
                  <p v-if="view === 'detailed' && monitor.description" class="pulse-service-description">
                    {{ monitor.description }}
                  </p>
                  <div v-if="view === 'detailed' && monitor.tags.length" class="pulse-tags">
                    <span v-for="tag in monitor.tags" :key="tag">{{ tag }}</span>
                  </div>
                  <StatusConnectionMeta v-if="view === 'detailed'" :beat="monitor.summary.latest" :tls-latency-warning-ms="bootstrap?.tlsLatencyWarningMs ?? 0" :cert-expiry-warning-days="monitor.certExpiryWarningDays ?? bootstrap?.certExpiryWarningDays ?? 30" />
                  <HeartbeatStrip :beats="monitor.summary.heartbeats" @capacity="resizeHeartbeat" />
                  <button v-if="view === 'compact'" class="pulse-detail-toggle" :aria-label="t('status.showDetails', { name: monitor.name })" :aria-expanded="expanded.includes(monitor.id)" :aria-controls="'connection-' + monitor.id" @click.stop="toggle(monitor.id)"><ChevronDown aria-hidden="true" /></button>
                  <StatusServiceDetails v-if="view !== 'cards'" :monitor="monitor" :open="expanded.includes(monitor.id)" :condensed="view === 'compact'" @capacity="resizeHeartbeat" />
                </article>
              </template>
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
