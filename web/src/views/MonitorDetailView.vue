<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Pencil, Pause, Play, Trash2 } from 'lucide-vue-next'
import type { MonitorView, Group, Heartbeat, MonitorLog, Settings } from '@/types'
import { request, send } from '@/lib/api'
import { useResource } from '@/composables/useResource'
import { useAction, notify } from '@/composables/useFeedback'
import { percent, latency, formatTime } from '@/lib/format'
import LoadState from '@/components/LoadState.vue'
import EmptyState from '@/components/EmptyState.vue'
import HeartbeatStrip from '@/components/HeartbeatStrip.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import ConnectionDetails from '@/components/ConnectionDetails.vue'
import MonitorEditor from '@/components/MonitorEditor.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
const { t } = useI18n()
const route = useRoute()
const id = Number(route.params.id)
const tab = ref<'heartbeats' | 'logs'>('heartbeats')
const editorOpen = ref(false)
const clearOpen = ref(false)
const { busy, run } = useAction()
const olderBeats = ref<Heartbeat[]>([])
const olderLogs = ref<MonitorLog[]>([])
const hasMoreBeats = ref(true)
const hasMoreLogs = ref(true)
const { data, loading, error, refresh } = useResource(async (signal) => {
  const [monitors, groups, beats, logs, settings] = await Promise.all([
    request<MonitorView[]>('/admin/monitors', { signal }),
    request<Group[]>('/admin/groups', { signal }),
    request<Heartbeat[]>(`/admin/monitors/${id}/heartbeats`, { signal }),
    request<MonitorLog[]>(`/admin/monitors/${id}/logs`, { signal }),
    request<Settings>('/admin/settings', { signal }),
  ])
  return {
    monitor: monitors.find((m) => m.id === id),
    groups,
    beats,
    logs,
    retentionDays: settings.retentionDays,
  }
}, 15000)
watch(data, (value) => {
  if (!value) return
  // 清空后不足一页时，丢弃已缓存的更早记录，避免已删除历史重新出现。
  if (value.beats.length < 100) olderBeats.value = []
  if (value.logs.length < 100) olderLogs.value = []
})
// 自动刷新和向前分页可能重叠，按主键去重后再按时间排序。
function merge<T extends { id: number; time: number }>(recent: T[], older: T[]): T[] {
  const cutoff = Date.now() - (data.value?.retentionDays || 90) * 86400000
  return [...new Map([...older, ...recent].map((item) => [item.id, item])).values()]
    .filter((item) => item.time >= cutoff)
    .sort((a, b) => b.time - a.time || b.id - a.id)
}
const beats = computed(() => merge(data.value?.beats || [], olderBeats.value))
const logs = computed(() => merge(data.value?.logs || [], olderLogs.value))
const canLoadMore = computed(() => {
  if (tab.value === 'heartbeats')
    return hasMoreBeats.value && (data.value?.beats.length || 0) >= 100
  return hasMoreLogs.value && (data.value?.logs.length || 0) >= 100
})
async function loadMore() {
  await run(async () => {
    if (tab.value === 'heartbeats') {
      const before = beats.value.at(-1)?.id
      const items = await request<Heartbeat[]>(`/admin/monitors/${id}/heartbeats?before=${before}`)
      olderBeats.value.push(...items)
      hasMoreBeats.value = items.length === 100
    } else {
      const before = logs.value.at(-1)?.id
      const items = await request<MonitorLog[]>(`/admin/monitors/${id}/logs?before=${before}`)
      olderLogs.value.push(...items)
      hasMoreLogs.value = items.length === 100
    }
  })
}
async function toggle() {
  if (!data.value?.monitor) return
  const enabled = !data.value.monitor.enabled
  await run(async () => {
    await send(`/admin/monitors/${id}/enabled`, 'PUT', { enabled })
    notify(enabled ? t('monitor.enabledToast') : t('monitor.pausedToast'))
    await refresh()
  })
}
async function clearHistory() {
  const kind = tab.value
  await run(async () => {
    const result = await send<{ deleted: number }>(`/admin/monitors/${id}/${kind}`, 'DELETE')
    olderBeats.value = []
    olderLogs.value = []
    hasMoreBeats.value = true
    hasMoreLogs.value = true
    clearOpen.value = false
    notify(t('monitor.cleared', { count: result.deleted }))
    await refresh()
  })
}
</script>
<template>
  <RouterLink to="/admin/monitors" class="back-link">
    <ArrowLeft :size="16" />
    {{ t('nav.monitors') }}
  </RouterLink>
  <LoadState :loading="loading" :error="error" :has-data="!!data" @retry="refresh" />
  <template v-if="data?.monitor">
    <div class="page-heading detail-heading">
      <div>
        <div class="flex flex-wrap items-center gap-3">
          <h1>{{ data.monitor.name }}</h1>
          <StatusBadge :status="data.monitor.summary.status" />
        </div>
        <p class="break-all">
          {{ data.monitor.target }}
          <template v-if="data.monitor.type === 'tcp'">:{{ data.monitor.port }}</template>
        </p>
      </div>
      <div class="flex gap-2">
        <Button variant="outline" :disabled="busy" @click="toggle">
          <Pause v-if="data.monitor.enabled" />
          <Play v-else />
          {{ data.monitor.enabled ? t('monitor.pause') : t('monitor.start') }}
        </Button>
        <Button @click="editorOpen = true">
          <Pencil />
          {{ t('common.edit') }}
        </Button>
      </div>
    </div>
    <section class="detail-summary">
      <div class="detail-stats">
        <div>
          <span>{{ t('status.uptimeShort') }}</span>
          <strong>{{ percent(data.monitor.summary.uptime) }}</strong>
        </div>
        <div>
          <span>{{ t('monitor.latency') }}</span>
          <strong>{{ latency(data.monitor.summary.latest?.latency) }}</strong>
        </div>
        <div>
          <span>{{ t('monitor.interval') }}</span>
          <strong>
            {{ data.monitor.interval }}
            <small>{{ t('common.seconds') }}</small>
          </strong>
        </div>
      </div>
      <HeartbeatStrip :beats="data.monitor.summary.heartbeats" />
      <p class="field-hint mt-2">{{ t('status.heartbeatHint') }}</p>
      <ConnectionDetails :beat="data.monitor.summary.latest" />
      <p v-if="data.monitor.description" class="detail-description">
        {{ data.monitor.description }}
      </p>
      <div v-if="data.monitor.tags.length" class="mt-3 flex gap-1">
        <Badge v-for="tag in data.monitor.tags" :key="tag">{{ tag }}</Badge>
      </div>
    </section>
    <section class="history-section">
      <header class="history-header">
        <div class="history-tabs" role="tablist">
          <button
            id="tab-heartbeats"
            role="tab"
            :aria-selected="tab === 'heartbeats'"
            aria-controls="history-content"
            :class="{ active: tab === 'heartbeats' }"
            @click="tab = 'heartbeats'"
          >
            {{ t('monitor.heartbeats') }}
          </button>
          <button
            id="tab-logs"
            role="tab"
            :aria-selected="tab === 'logs'"
            aria-controls="history-content"
            :class="{ active: tab === 'logs' }"
            @click="tab = 'logs'"
          >
            {{ t('monitor.logs') }}
          </button>
        </div>
        <Button
          variant="ghost"
          size="sm"
          class="text-destructive"
          :disabled="busy"
          @click="clearOpen = true"
        >
          <Trash2 />
          {{ tab === 'heartbeats' ? t('monitor.clearHeartbeats') : t('monitor.clearLogs') }}
        </Button>
      </header>
      <p v-if="tab === 'logs'" class="history-hint">
        {{ t('monitor.logHint') }}
      </p>
      <div
        id="history-content"
        role="tabpanel"
        :aria-labelledby="`tab-${tab}`"
        class="history-content"
      >
        <EmptyState
          v-if="!(tab === 'heartbeats' ? beats.length : logs.length)"
          :title="t('common.empty')"
          :description="t('monitor.historyEmpty')"
        />
        <div v-else class="table-scroll">
          <table class="history-table">
            <thead>
              <tr>
                <th>{{ t('common.time') }}</th>
                <th>{{ t('common.status') }}</th>
                <th v-if="tab === 'heartbeats'">{{ t('monitor.latency') }}</th>
                <th>{{ t('monitor.result') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in tab === 'heartbeats' ? beats : logs" :key="item.id">
                <td class="tabular-nums whitespace-nowrap">
                  {{ formatTime(item.time) }}
                </td>
                <td><StatusBadge :status="item.status" /></td>
                <td v-if="tab === 'heartbeats'" class="tabular-nums whitespace-nowrap">
                  {{ latency((item as Heartbeat).latency) }}
                </td>
                <td class="history-message">{{ item.message }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="canLoadMore" class="history-more">
          <Button variant="outline" :disabled="busy" @click="loadMore">
            {{ t('common.loadMore') }}
          </Button>
        </div>
      </div>
    </section>
    <MonitorEditor
      v-model:open="editorOpen"
      :monitor="data.monitor"
      :groups="data.groups"
      @saved="refresh"
    />
    <ConfirmDialog
      v-model:open="clearOpen"
      :title="t('monitor.clearTitle', { kind: t(`monitor.${tab}`) })"
      :description="
        t('monitor.clearDescription', {
          name: data.monitor.name,
          kind: t(`monitor.${tab}`),
        })
      "
      :busy="busy"
      @confirm="clearHistory"
    />
  </template>
</template>
