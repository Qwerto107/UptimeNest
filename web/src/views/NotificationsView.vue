<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { TabsRoot, TabsList, TabsTrigger, TabsContent } from 'reka-ui'
import { Plus, Bell, Pencil, Trash2, Send, RefreshCw } from 'lucide-vue-next'
import type { NotificationChannel, NotificationRecord } from '@/types'
import NotificationChannelEditor, { type NotificationTemplates } from '@/components/NotificationChannelEditor.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import LoadState from '@/components/LoadState.vue'
import EmptyState from '@/components/EmptyState.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { request, send } from '@/lib/api'
import { formatTime } from '@/lib/format'
import { translateApiError } from '@/locales/api-errors'
import { useResource } from '@/composables/useResource'
import { useAction, notify } from '@/composables/useFeedback'
const { t, locale } = useI18n()
const { busy, run } = useAction()
const editorOpen = ref(false)
const deleteOpen = ref(false)
const selected = ref<NotificationChannel | null>(null)
const tab = ref('channels')
const older = ref<NotificationRecord[]>([])
const noMore = ref(false)
const { data, loading, error, refresh } = useResource(async signal => {
  const [channels, templates, records] = await Promise.all([
    request<NotificationChannel[]>('/admin/notification-channels', { signal }),
    request<NotificationTemplates>('/admin/notifications/templates', { signal }),
    request<NotificationRecord[]>('/admin/notifications/records', { signal }),
  ])
  return { channels, templates, records }
}, 15000)
function edit(channel: NotificationChannel | null) { selected.value = channel; editorOpen.value = true }
async function test(channel: NotificationChannel) {
  await run(async () => { await send(`/admin/notification-channels/${channel.id}/test`, 'POST'); notify(t('notification.testQueued')); await refresh() })
}
async function remove() {
  if (!selected.value) return
  await run(async () => { await send(`/admin/notification-channels/${selected.value!.id}`, 'DELETE'); deleteOpen.value = false; notify(t('common.deleted')); await refresh() })
}
async function loadMore() {
  const records = [...(data.value?.records || []), ...older.value]
  const before = records.at(-1)?.id
  if (!before) return
  await run(async () => { const next = await request<NotificationRecord[]>(`/admin/notifications/records?before=${before}`); older.value.push(...next); noMore.value = next.length < 50 })
}
function visibleRecords() {
  const seen = new Set<number>()
  return [...(data.value?.records || []), ...older.value].filter(item => { if (seen.has(item.id)) return false; seen.add(item.id); return true })
}
</script>
<template>
  <div class="page-heading"><div><h1>{{ t('notification.title') }}</h1><p>{{ t('notification.subtitle') }}</p></div><Button :disabled="!data" @click="edit(null)"><Plus />{{ t('notification.add') }}</Button></div>
  <TabsRoot v-model="tab">
  <div class="notification-page-toolbar">
    <TabsList class="notification-tabs" :aria-label="t('notification.title')">
      <TabsTrigger value="channels" class="notification-tab">{{ t('notification.channels') }}</TabsTrigger>
      <TabsTrigger value="records" class="notification-tab">{{ t('notification.records') }}</TabsTrigger>
    </TabsList>
    <Button variant="ghost" size="icon" :aria-label="t('common.refresh')" :disabled="busy || loading" @click="older = []; noMore = false; refresh()"><RefreshCw /></Button>
  </div>
  <LoadState :loading="loading" :error="error" :has-data="!!data" skeleton="panels" @retry="refresh" />
  <template v-if="data">
    <TabsContent value="channels">
      <EmptyState v-if="!data.channels.length" :title="t('notification.emptyTitle')" :description="t('notification.emptyHint')"><template #icon><Bell /></template><Button @click="edit(null)"><Plus />{{ t('notification.add') }}</Button></EmptyState>
      <div v-else class="entity-list">
        <article v-for="channel in data.channels" :key="channel.id" class="entity-row">
          <span class="entity-icon"><Bell :size="20" /></span>
          <div class="entity-info"><div class="flex flex-wrap items-center gap-2"><h2>{{ channel.name }}</h2><Badge>{{ t(channel.enabled ? 'common.enabled' : 'common.disabled') }}</Badge></div><p>{{ t(`notification.types.${channel.type}`) }}</p></div>
          <div class="row-actions">
            <Button variant="ghost" size="icon" :disabled="busy || !channel.enabled" :title="t('notification.test')" :aria-label="`${t('notification.test')} ${channel.name}`" @click="test(channel)"><Send /></Button>
            <Button variant="ghost" size="icon" :disabled="busy" :aria-label="`${t('common.edit')} ${channel.name}`" @click="edit(channel)"><Pencil /></Button>
            <Button variant="ghost" size="icon" :disabled="busy" :aria-label="`${t('common.delete')} ${channel.name}`" @click="selected = channel; deleteOpen = true"><Trash2 /></Button>
          </div>
        </article>
      </div>
    </TabsContent>
    <TabsContent value="records">
      <p class="field-hint mb-4">{{ t('notification.recordsHint') }}</p>
      <EmptyState v-if="!visibleRecords().length" :title="t('notification.noRecords')" :description="t('notification.noRecordsHint')"><template #icon><Send /></template></EmptyState>
      <div v-else class="notification-records">
        <article v-for="item in visibleRecords()" :key="item.id" class="notification-record">
          <div><strong>{{ item.event.monitorName }}</strong><span class="field-hint">{{ item.channelName || `#${item.channelId}` }} · {{ t(`notification.events.${item.event.alertType}`) }}</span></div>
          <div><Badge class="justify-self-start" :class="{ 'text-destructive': item.status === 'failed' }">{{ t(`notification.states.${item.status}`) }}</Badge><span class="field-hint">{{ formatTime(item.event.time) }} · {{ t('notification.attempts', { count: item.attempts }) }}</span></div>
          <p v-if="item.error" class="notification-record-error">{{ translateApiError(item.error, locale) }}</p>
        </article>
      </div>
      <Button v-if="data.records.length === 50 && !noMore" class="mt-4" variant="outline" :disabled="busy" @click="loadMore">{{ t('common.loadMore') }}</Button>
    </TabsContent>
    <NotificationChannelEditor v-model:open="editorOpen" :channel="selected" :templates="data.templates" @saved="refresh" />
  </template>
  </TabsRoot>
  <ConfirmDialog v-model:open="deleteOpen" :title="t('notification.deleteTitle')" :description="t('notification.deleteHint', { name: selected?.name })" :busy="busy" @confirm="remove" />
</template>
