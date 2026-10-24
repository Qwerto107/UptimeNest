<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useEventListener } from '@vueuse/core'
import { Plus, Search, Pencil, Pause, Play, Trash2, ArrowUpRight, RefreshCw } from 'lucide-vue-next'
import type { MonitorView, Group } from '@/types'
import { request, send } from '@/lib/api'
import { useResource } from '@/composables/useResource'
import { useAction, notify } from '@/composables/useFeedback'
import { percent, latency } from '@/lib/format'
import LoadState from '@/components/LoadState.vue'
import EmptyState from '@/components/EmptyState.vue'
import HeartbeatStrip from '@/components/HeartbeatStrip.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import MonitorEditor from '@/components/MonitorEditor.vue'
import MonitorGroupTree from '@/components/MonitorGroupTree.vue'
import { groupChoices, descendantGroups } from '@/lib/groups'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
const { t } = useI18n()
const route = useRoute()
const search = ref('')
const searchBox = ref<HTMLDivElement>()
const searchShortcut = navigator.userAgent.includes('Mac')
  ? 'common.searchShortcutMac'
  : 'common.searchShortcut'
useEventListener(window, 'keydown', (event) => {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    searchBox.value?.querySelector('input')?.focus()
  }
})
const groupId = ref(String(route.query.group || 'all'))
const status = ref('all')
const editorOpen = ref(false)
const selected = ref<MonitorView | null>(null)
const deleteOpen = ref(false)
const deleting = ref<MonitorView | null>(null)
const { busy, run } = useAction()
const { data, loading, error, refresh, refreshing } = useResource(async (signal) => {
  const [monitors, groups] = await Promise.all([
    request<MonitorView[]>('/admin/monitors', { signal }),
    request<Group[]>('/admin/groups', { signal }),
  ])
  return { monitors, groups }
}, 15000)
const filtered = computed(() =>
  (data.value?.monitors || []).filter((m) => {
    const query = search.value.trim().toLowerCase()
    const path = choices.value.find((group) => group.id === m.groupId)?.label || ''
    if (query && !`${m.name} ${m.target} ${m.tags.join(' ')} ${path}`.toLowerCase().includes(query))
      return false
    if (groupId.value !== 'all' && !selectedGroups.value.has(m.groupId)) return false
    return status.value === 'all' || m.summary.status === status.value
  }),
)
const choices = computed(() => groupChoices(data.value?.groups || []))
const selectedGroups = computed(() => {
  if (groupId.value === '0') return new Set([0])
  return descendantGroups(data.value?.groups || [], Number(groupId.value))
})
const searching = computed(
  () => !!search.value.trim() || groupId.value !== 'all' || status.value !== 'all',
)
const overview = computed(() => {
  const monitors = data.value?.monitors || []
  return [
    { key: 'total', count: monitors.length },
    { key: 'up', count: monitors.filter((monitor) => monitor.summary.status === 'up').length },
    { key: 'down', count: monitors.filter((monitor) => monitor.summary.status === 'down').length },
    { key: 'paused', count: monitors.filter((monitor) => !monitor.enabled).length },
  ]
})
function edit(monitor: MonitorView | null) {
  selected.value = monitor
  editorOpen.value = true
}
function askDelete(monitor: MonitorView) {
  deleting.value = monitor
  deleteOpen.value = true
}
async function remove() {
  if (!deleting.value) return
  await run(async () => {
    await send(`/admin/monitors/${deleting.value!.id}`, 'DELETE')
    deleteOpen.value = false
    notify(t('common.deleted'))
    await refresh()
  })
}
async function toggle(monitor: MonitorView) {
  await run(async () => {
    await send(`/admin/monitors/${monitor.id}/enabled`, 'PUT', {
      enabled: !monitor.enabled,
    })
    notify(monitor.enabled ? t('monitor.pausedToast') : t('monitor.enabledToast'))
    await refresh()
  })
}
</script>
<template>
  <div class="page-heading">
    <div>
      <h1>{{ t('monitor.title') }}</h1>
      <p>{{ t('monitor.subtitle') }}</p>
    </div>
    <Button @click="edit(null)">
      <Plus />
      {{ t('monitor.add') }}
    </Button>
  </div>
  <LoadState :loading="loading" :error="error" :has-data="!!data" @retry="refresh" />
  <template v-if="data">
    <div class="monitor-overview">
      <div
        v-for="item in overview"
        :key="item.key"
        class="overview-stat"
        :class="'overview-' + item.key"
      >
        <span class="overview-caption">{{ t('monitor.' + item.key + 'Caption') }}</span>
        <div>
          <strong>{{ item.count }}</strong>
          <span>{{ t('monitor.' + item.key + 'Label') }}</span>
        </div>
      </div>
    </div>
    <EmptyState
      v-if="!data.monitors.length && !data.groups.length"
      :title="t('monitor.emptyTitle')"
      :description="t('monitor.emptyDescription')"
    >
      <Button @click="edit(null)">
        <Plus />
        {{ t('monitor.add') }}
      </Button>
    </EmptyState>
    <template v-else>
      <div class="list-toolbar">
        <div ref="searchBox" class="search-input">
          <Search :size="16" />
          <Input
            v-model="search"
            :placeholder="t('common.search')"
            :aria-label="t('common.search')"
          />
          <kbd>{{ t(searchShortcut) }}</kbd>
        </div>
        <select v-model="groupId" class="control filter-select" :aria-label="t('monitor.group')">
          <option value="all">{{ t('monitor.allGroups') }}</option>
          <option value="0">{{ t('common.ungrouped') }}</option>
          <option v-for="group in choices" :key="group.id" :value="String(group.id)">
            {{ group.label }}
          </option>
        </select>
        <select v-model="status" class="control filter-select" :aria-label="t('common.status')">
          <option value="all">{{ t('monitor.allStates') }}</option>
          <option v-for="kind in ['up', 'down', 'unknown', 'paused']" :key="kind" :value="kind">
            {{ t(`state.${kind}`) }}
          </option>
        </select>
        <Button
          variant="outline"
          size="icon"
          :disabled="refreshing"
          :aria-label="t('common.refresh')"
          @click="refresh"
        >
          <RefreshCw :class="{ 'animate-spin': refreshing }" />
        </Button>
      </div>
      <EmptyState v-if="!filtered.length && searching" :title="t('monitor.noMatch')" />
      <MonitorGroupTree
        v-else
        :groups="data.groups"
        :monitors="data.monitors"
        :visible-ids="filtered.map((monitor) => monitor.id)"
        :searching="searching"
      >
        <template #monitor="{ monitor }">
          <article class="monitor-list-row" :data-state="monitor.summary.status">
            <div class="monitor-list-info">
              <div class="monitor-identity">
                <RouterLink class="monitor-title" :to="`/admin/monitors/${monitor.id}`">
                  {{ monitor.name }}
                </RouterLink>
              </div>
              <p class="monitor-target">
                {{ monitor.target }}
                <template v-if="monitor.type === 'tcp'">:{{ monitor.port }}</template>
              </p>
              <div v-if="monitor.tags.length" class="mt-1 flex flex-wrap gap-1">
                <Badge v-for="tag in monitor.tags" :key="tag">{{ tag }}</Badge>
              </div>
            </div>
            <Badge class="monitor-protocol">{{ t(`monitor.${monitor.type}`) }}</Badge>
            <div class="monitor-list-state">
              <StatusBadge :status="monitor.summary.status" />
              <span>
                {{
                  latency(
                    monitor.summary.status === 'up' ? monitor.summary.latest?.latency : undefined,
                  )
                }}
              </span>
            </div>
            <div class="monitor-list-history">
              <HeartbeatStrip :beats="monitor.summary.heartbeats" />
              <span>{{ percent(monitor.summary.uptime) }}</span>
            </div>
            <div class="row-actions">
              <Button
                as-child
                variant="ghost"
                size="icon"
                :aria-label="`${t('common.view')} ${monitor.name}`"
              >
                <RouterLink :to="`/admin/monitors/${monitor.id}`"><ArrowUpRight /></RouterLink>
              </Button>
              <Button
                variant="ghost"
                size="icon"
                :aria-label="`${t('common.edit')} ${monitor.name}`"
                @click="edit(monitor)"
              >
                <Pencil />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                :disabled="busy"
                :aria-label="`${monitor.enabled ? t('monitor.pause') : t('monitor.start')} ${monitor.name}`"
                @click="toggle(monitor)"
              >
                <Pause v-if="monitor.enabled" />
                <Play v-else />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                class="text-destructive"
                :disabled="busy"
                :aria-label="`${t('common.delete')} ${monitor.name}`"
                @click="askDelete(monitor)"
              >
                <Trash2 />
              </Button>
            </div>
          </article>
        </template>
      </MonitorGroupTree>
    </template>
  </template>
  <MonitorEditor
    v-model:open="editorOpen"
    :monitor="selected"
    :groups="data?.groups || []"
    @saved="refresh"
  />
  <ConfirmDialog
    v-model:open="deleteOpen"
    :title="t('monitor.deleteTitle')"
    :description="t('monitor.deleteDescription', { name: deleting?.name })"
    :busy="busy"
    :label="t('common.delete')"
    @confirm="remove"
  />
</template>
