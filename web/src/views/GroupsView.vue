<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Pencil, Trash2, Folder } from 'lucide-vue-next'
import type { Group, MonitorView } from '@/types'
import { request, send } from '@/lib/api'
import { useResource } from '@/composables/useResource'
import { useAction, notify } from '@/composables/useFeedback'
import LoadState from '@/components/LoadState.vue'
import EmptyState from '@/components/EmptyState.vue'
import Modal from '@/components/Modal.vue'
import Field from '@/components/Field.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import MonitorGroupTree from '@/components/MonitorGroupTree.vue'
import { groupChoices, descendantGroups } from '@/lib/groups'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
const { t } = useI18n()
const { busy, run } = useAction()
const editorOpen = ref(false)
const selected = ref<Group | null>(null)
const name = ref('')
const parentId = ref(0)
const deleteOpen = ref(false)
const { data, loading, error, refresh } = useResource(async (signal) => {
  const [groups, monitors] = await Promise.all([
    request<Group[]>('/admin/groups', { signal }),
    request<MonitorView[]>('/admin/monitors', { signal }),
  ])
  return { groups, monitors }
}, 15000)
const parentOptions = computed(() => {
  const excluded = descendantGroups(data.value?.groups || [], selected.value?.id || -1)
  return groupChoices(data.value?.groups || []).filter((group) => !excluded.has(group.id))
})
function edit(group: Group | null, parent = 0) {
  selected.value = group
  name.value = group?.name || ''
  parentId.value = group?.parentId || parent
  editorOpen.value = true
}
function askDelete(group: Group) {
  selected.value = group
  deleteOpen.value = true
}
async function save() {
  await run(async () => {
    await send(
      '/admin/groups' + (selected.value ? `/${selected.value.id}` : ''),
      selected.value ? 'PUT' : 'POST',
      { name: name.value, parentId: parentId.value },
    )
    editorOpen.value = false
    notify(t('common.saved'))
    await refresh()
  })
}
async function remove() {
  if (!selected.value) return
  await run(async () => {
    await send(`/admin/groups/${selected.value!.id}`, 'DELETE')
    deleteOpen.value = false
    notify(t('common.deleted'))
    await refresh()
  })
}
</script>
<template>
  <div class="page-heading">
    <div>
      <h1>{{ t('group.title') }}</h1>
      <p>{{ t('group.subtitle') }}</p>
    </div>
    <Button @click="edit(null)">
      <Plus />
      {{ t('group.add') }}
    </Button>
  </div>
  <LoadState :loading="loading" :error="error" :has-data="!!data" @retry="refresh" />
  <template v-if="data">
    <EmptyState
      v-if="!data.groups.length"
      :title="t('group.emptyTitle')"
      :description="t('group.emptyDescription')"
    >
      <template #icon><Folder /></template>
      <Button @click="edit(null)">
        <Plus />
        {{ t('group.add') }}
      </Button>
    </EmptyState>
    <MonitorGroupTree v-else class="group-tree" :groups="data.groups" :monitors="data.monitors">
      <template #group-actions="{ group }">
        <Button
          variant="ghost"
          size="icon"
          :aria-label="t('group.addChild', { name: group.name })"
          @click="edit(null, group.id)"
        >
          <Plus />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          :aria-label="`${t('common.edit')} ${group.name}`"
          @click="edit(group)"
        >
          <Pencil />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          class="text-destructive"
          :aria-label="`${t('common.delete')} ${group.name}`"
          @click="askDelete(group)"
        >
          <Trash2 />
        </Button>
      </template>
    </MonitorGroupTree>
  </template>
  <Modal
    v-model:open="editorOpen"
    :title="selected ? t('group.edit') : t('group.add')"
    :busy="busy"
  >
    <form class="editor-form" @submit.prevent="save">
      <Field id="group-name" :label="t('common.name')" required>
        <Input
          id="group-name"
          v-model="name"
          required
          maxlength="200"
          :placeholder="t('group.nameExample')"
        />
      </Field>
      <Field id="group-parent" :label="t('group.parent')">
        <select id="group-parent" v-model.number="parentId" class="control">
          <option :value="0">{{ t('group.root') }}</option>
          <option v-for="group in parentOptions" :key="group.id" :value="group.id">
            {{ group.label }}
          </option>
        </select>
      </Field>
      <div class="form-actions">
        <Button type="button" variant="outline" :disabled="busy" @click="editorOpen = false">
          {{ t('common.cancel') }}
        </Button>
        <Button type="submit" :disabled="busy">{{ t('common.save') }}</Button>
      </div>
    </form>
  </Modal>
  <ConfirmDialog
    v-model:open="deleteOpen"
    :title="t('group.deleteTitle')"
    :description="t('group.deleteDescription', { name: selected?.name })"
    :busy="busy"
    @confirm="remove"
  />
</template>
