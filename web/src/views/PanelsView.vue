<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Pencil, Trash2, ArrowUpRight, Star, PanelsTopLeft } from 'lucide-vue-next'
import PanelGroupsEditor from '@/components/PanelGroupsEditor.vue'
import type { Panel, MonitorView } from '@/types'
import { request, send } from '@/lib/api'
import { useResource } from '@/composables/useResource'
import { useAction, notify } from '@/composables/useFeedback'
import { loadBootstrap } from '@/composables/useApp'
import LoadState from '@/components/LoadState.vue'
import EmptyState from '@/components/EmptyState.vue'
import Modal from '@/components/Modal.vue'
import Field from '@/components/Field.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
const { t } = useI18n()
const { busy, run } = useAction()
const editorOpen = ref(false)
const deleteOpen = ref(false)
const selected = ref<Panel | null>(null)
const form = reactive<Panel>({
  id: 0,
  name: '',
  pageTitle: '',
  slug: '',
  description: '',
  monitorIds: [],
  groups: [],
  isDefault: false,
})
const { data, loading, error, refresh } = useResource(async (signal) => {
  const [panels, monitors] = await Promise.all([
    request<Panel[]>('/admin/panels', { signal }),
    request<MonitorView[]>('/admin/monitors', { signal }),
  ])
  return { panels, monitors }
})
function edit(panel: Panel | null) {
  Object.assign(form, {
    id: 0,
    name: '',
    pageTitle: '',
    slug: '',
    description: '',
    monitorIds: [],
    groups: [],
    isDefault: false,
  })
  if (panel)
    Object.assign(form, {
      ...panel,
      monitorIds: [...panel.monitorIds],
      groups: (panel.groups || []).map((group) => ({
        ...group,
        monitorIds: [...group.monitorIds],
      })),
    })
  editorOpen.value = true
}
function askDelete(panel: Panel) {
  selected.value = panel
  deleteOpen.value = true
}
async function updated() {
  await refresh()
  await loadBootstrap(0)
}
async function save() {
  await run(async () => {
    await send('/admin/panels' + (form.id ? `/${form.id}` : ''), form.id ? 'PUT' : 'POST', {
      ...form,
      monitorIds: form.groups.flatMap((group) => group.monitorIds),
    })
    editorOpen.value = false
    notify(t('common.saved'))
    await updated()
  })
}
async function remove() {
  if (!selected.value) return
  await run(async () => {
    await send(`/admin/panels/${selected.value!.id}`, 'DELETE')
    deleteOpen.value = false
    notify(t('common.deleted'))
    await updated()
  })
}
async function setDefault(panel: Panel) {
  await run(async () => {
    await send(`/admin/panels/${panel.id}`, 'PUT', {
      ...panel,
      isDefault: true,
    })
    notify(t('common.saved'))
    await updated()
  })
}
</script>
<template>
  <div class="page-heading">
    <div>
      <h1>{{ t('panel.title') }}</h1>
      <p>{{ t('panel.subtitle') }}</p>
    </div>
    <Button @click="edit(null)">
      <Plus />
      {{ t('panel.add') }}
    </Button>
  </div>
  <LoadState :loading="loading" :error="error" :has-data="!!data" skeleton="panels" @retry="refresh" />
  <template v-if="data">
    <EmptyState
      v-if="!data.panels.length"
      :title="t('panel.emptyTitle')"
      :description="t('panel.emptyDescription')"
    >
      <template #icon><PanelsTopLeft /></template>
      <Button @click="edit(null)">
        <Plus />
        {{ t('panel.add') }}
      </Button>
    </EmptyState>
    <div v-else class="entity-list">
      <article
        v-for="panel in data.panels"
        :key="panel.id"
        class="entity-row"
        :class="{ 'entity-row-default': panel.isDefault }"
      >
        <span class="entity-icon"><PanelsTopLeft :size="20" /></span>
        <div class="entity-info">
          <div class="flex flex-wrap items-center gap-2">
            <h2>{{ panel.name }}</h2>
            <Badge v-if="panel.isDefault" class="default-badge">
              <Star :size="12" class="mr-1" />
              {{ t('panel.default') }}
            </Badge>
          </div>
          <p class="panel-path">
            /status/{{ panel.slug }}
            <span>· {{ t('panel.count', { count: panel.monitorIds.length }) }}</span>
          </p>
        </div>
        <div class="row-actions">
          <Button
            v-if="!panel.isDefault"
            variant="ghost"
            size="icon"
            :disabled="busy"
            :aria-label="`${t('panel.setDefault')} ${panel.name}`"
            :title="t('panel.setDefault')"
            @click="setDefault(panel)"
          >
            <Star />
          </Button>
          <Button
            as-child
            variant="ghost"
            size="icon"
            :aria-label="`${t('panel.preview')} ${panel.name}`"
          >
            <RouterLink :to="`/status/${panel.slug}`" target="_blank"><ArrowUpRight /></RouterLink>
          </Button>
          <Button
            variant="ghost"
            size="icon"
            :aria-label="`${t('common.edit')} ${panel.name}`"
            @click="edit(panel)"
          >
            <Pencil />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            class="text-destructive"
            :aria-label="`${t('common.delete')} ${panel.name}`"
            @click="askDelete(panel)"
          >
            <Trash2 />
          </Button>
        </div>
      </article>
    </div>
  </template>
  <Modal
    v-model:open="editorOpen"
    :title="form.id ? t('panel.edit') : t('panel.add')"
    :description="t('panel.editorHint')"
    wide
    :busy="busy"
  >
    <form class="editor-form panel-editor-form" @submit.prevent="save">
      <fieldset :disabled="busy" class="form-grid">
        <Field id="panel-name" :label="t('common.name')" required>
          <Input
            id="panel-name"
            v-model="form.name"
            required
            :placeholder="t('panel.nameExample')"
          />
        </Field>
        <Field id="panel-slug" :label="t('panel.slug')" :hint="t('panel.slugHint')" required>
          <Input
            id="panel-slug"
            v-model="form.slug"
            required
            pattern="[a-z0-9][a-z0-9\-]{0,63}"
            :placeholder="t('panel.slugExample')"
          />
        </Field>
        <Field id="panel-page-title" :label="t('panel.pageTitle')" :hint="t('panel.pageTitleHint')">
          <Input id="panel-page-title" v-model="form.pageTitle" maxlength="200" />
        </Field>
        <Field id="panel-description" :label="t('common.description')" class="col-span-full">
          <Textarea id="panel-description" v-model="form.description" maxlength="4000" />
        </Field>
      </fieldset>
      <div class="toggle-row">
        <div>
          <label for="panel-default">{{ t('panel.default') }}</label>
          <p>{{ t('panel.defaultHint') }}</p>
        </div>
        <Switch id="panel-default" v-model="form.isDefault" :disabled="busy" />
      </div>
      <PanelGroupsEditor
        v-if="editorOpen"
        v-model="form.groups"
        :monitors="data?.monitors || []"
        :busy="busy"
      />
      <div class="form-actions">
        <Button type="button" variant="outline" :disabled="busy" @click="editorOpen = false">
          {{ t('common.cancel') }}
        </Button>
        <Button type="submit" :disabled="busy">
          {{ busy ? t('common.saving') : t('common.save') }}
        </Button>
      </div>
    </form>
  </Modal>
  <ConfirmDialog
    v-model:open="deleteOpen"
    :title="t('panel.deleteTitle')"
    :description="t('panel.deleteDescription', { name: selected?.name })"
    :busy="busy"
    @confirm="remove"
  />
</template>
