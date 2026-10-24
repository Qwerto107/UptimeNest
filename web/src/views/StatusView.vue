<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { RefreshCw } from 'lucide-vue-next'
import type { PublicPanel, PublicMonitor, MonitorStatus } from '@/types'
import { request } from '@/lib/api'
import { bootstrap, loadBootstrap, timezone } from '@/composables/useApp'
import { useResource } from '@/composables/useResource'
import { formatTime, percent } from '@/lib/format'
import ThemeToggle from '@/components/ThemeToggle.vue'
import LoadState from '@/components/LoadState.vue'
import EmptyState from '@/components/EmptyState.vue'
import HeartbeatStrip from '@/components/HeartbeatStrip.vue'
import ConnectionDetails from '@/components/ConnectionDetails.vue'
import StatusConnectionMeta from '@/components/StatusConnectionMeta.vue'
import '@/styles/status.css'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const updatedAt = ref<number>()
const expanded = ref<number[]>([])
const heartbeatCount = 90
const year = new Date().getFullYear()

const { data, loading, error, refresh, refreshing } = useResource(async (signal) => {
  const [value] = await Promise.all([
    request<PublicPanel>('/public/panels/' + encodeURIComponent(String(route.params.slug)), {
      signal,
    }),
    loadBootstrap(),
  ])
  timezone.value = value.timezone
  updatedAt.value = Date.now()
  return value
}, 30000)

const monitors = computed(() => data.value?.monitors || [])
const overall = computed<MonitorStatus>(() => {
  if (!monitors.value.length) return 'unknown'
  if (monitors.value.some((monitor) => monitor.summary.status === 'down')) return 'down'
  if (monitors.value.every((monitor) => monitor.summary.status === 'paused')) return 'paused'
  if (monitors.value.some((monitor) => monitor.summary.status === 'unknown')) return 'unknown'
  return 'up'
})

const statusCopy = computed(() => {
  if (!monitors.value.length)
    return { title: t('status.emptyTitle'), description: t('status.emptyDescription') }
  if (overall.value === 'down') {
    const count = monitors.value.filter((monitor) => monitor.summary.status === 'down').length
    return { title: t('status.someDown'), description: t('status.downDescription', { count }) }
  }
  if (overall.value === 'paused')
    return { title: t('status.paused'), description: t('status.pausedDescription') }
  if (overall.value === 'unknown')
    return { title: t('status.waiting'), description: t('status.waitingDescription') }
  const count = monitors.value.filter((monitor) => monitor.summary.status === 'paused').length
  if (count)
    return { title: t('status.activeUp'), description: t('status.activeUpDescription', { count }) }
  return { title: t('status.allUp'), description: '' }
})

const groups = computed(() => {
  if (!data.value) return []
  const index = new Map(monitors.value.map((monitor) => [monitor.id, monitor]))
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

function toggle(id: number) {
  if (expanded.value.includes(id)) expanded.value = expanded.value.filter((value) => value !== id)
  else expanded.value.push(id)
}

function selectPanel(event: Event) {
  void router.push('/status/' + (event.target as HTMLSelectElement).value)
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
  <div class="pulse-page">
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
          <span>{{ t('common.brand') }}</span>
        </RouterLink>
        <div class="pulse-tools">
          <select
            v-if="bootstrap && bootstrap.panels.length > 1"
            class="pulse-panel-select"
            :aria-label="t('status.panel')"
            :value="route.params.slug"
            @change="selectPanel"
          >
            <option v-for="panel in bootstrap.panels" :key="panel.slug" :value="panel.slug">
              {{ panel.name }}
            </option>
          </select>
          <span v-else-if="data" class="pulse-panel-name">{{ data.panel.name }}</span>
          <ThemeToggle />
          <RouterLink class="pulse-admin-link" to="/admin">{{ t('nav.admin') }}</RouterLink>
        </div>
        <p v-if="data?.panel.description" class="pulse-panel-description">
          {{ data.panel.description }}
        </p>
      </header>

      <LoadState :loading="loading" :error="error" :has-data="!!data" @retry="refresh" />

      <template v-if="data">
        <section class="pulse-summary" :class="'pulse-summary-' + overall" aria-live="polite">
          <div class="pulse-summary-main">
            <span class="pulse-signal" aria-hidden="true">
              <span v-if="overall === 'up'" class="pulse-signal-wave" />
              <span class="pulse-signal-dot" />
            </span>
            <div>
              <h1>{{ statusCopy.title }}</h1>
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
          <button
            class="pulse-refresh"
            type="button"
            :disabled="refreshing"
            :aria-label="t('common.refresh')"
            :title="t('status.autoUpdate')"
            @click="refresh"
          >
            <RefreshCw :size="14" :class="{ 'animate-spin': refreshing }" />
          </button>
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
              <span>{{ t('status.heartbeat', { count: heartbeatCount }) }}</span>
            </div>
            <div class="pulse-service-list">
              <article v-for="monitor in group.monitors" :key="monitor.id" class="pulse-service">
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
                      class="pulse-uptime pulse-mono"
                      :title="t('status.uptimeShort')"
                      :aria-label="t('status.uptimeShort') + ' ' + percent(monitor.summary.uptime)"
                    >
                      {{ percent(monitor.summary.uptime) }}
                    </span>
                    <span class="pulse-state" :class="'pulse-state-' + monitor.summary.status">
                      <span aria-hidden="true" />
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
                <StatusConnectionMeta :beat="monitor.summary.latest" />
                <HeartbeatStrip
                  :beats="monitor.summary.heartbeats"
                  :count="heartbeatCount"
                  align-pixels
                />
                <div
                  v-if="expanded.includes(monitor.id)"
                  :id="'connection-' + monitor.id"
                  class="pulse-expanded"
                >
                  <ConnectionDetails :beat="monitor.summary.latest" />
                </div>
              </article>
            </div>
          </section>

          <div v-if="monitors.length" class="pulse-history-note">
            <span>{{ t('status.heartbeatHint') }}</span>
            <span>{{ t('status.uptimeShort') }}</span>
          </div>
        </main>

        <footer class="pulse-footer">
          <span>{{ t('status.copyright', { year }) }}</span>
          <span class="pulse-update-setting">
            {{ t('status.autoUpdate') }}
            <span class="pulse-timezone pulse-mono">{{ data.timezone }}</span>
          </span>
          <span class="pulse-mono">{{ t('status.poweredBy') }}</span>
        </footer>
      </template>
    </div>
  </div>
</template>
