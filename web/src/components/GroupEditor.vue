<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Group } from '@/types'
import { send } from '@/lib/api'
import { descendantGroups, groupChoices } from '@/lib/groups'
import { useAction, notify } from '@/composables/useFeedback'
import Modal from '@/components/Modal.vue'
import Field from '@/components/Field.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ group: Group | null; parentId: number; groups: Group[] }>()
const emit = defineEmits<{ saved: [] }>()
const { t } = useI18n()
const { busy, run } = useAction()
const name = ref('')
const parentId = ref(0)
const parentOptions = computed(() => {
  // 禁止把分组移动到自身或子孙下面，避免形成循环。
  const excluded = descendantGroups(props.groups, props.group?.id || -1)
  return groupChoices(props.groups).filter((group) => !excluded.has(group.id))
})
watch(open, (value) => {
  if (!value) return
  name.value = props.group?.name || ''
  parentId.value = props.parentId
  if (props.group) parentId.value = props.group.parentId
})
async function save() {
  await run(async () => {
    let path = '/admin/groups'
    let method: 'POST' | 'PUT' = 'POST'
    if (props.group) {
      path += `/${props.group.id}`
      method = 'PUT'
    }
    await send(path, method, { name: name.value, parentId: parentId.value })
    open.value = false
    notify(t('common.saved'))
    emit('saved')
  })
}
</script>

<template>
  <Modal v-model:open="open" :title="group ? t('group.edit') : t('group.add')" :busy="busy">
    <form class="editor-form" @submit.prevent="save">
      <Field id="group-name" :label="t('common.name')" required>
        <Input
          id="group-name"
          v-model="name"
          required
          maxlength="200"
          :placeholder="t('group.nameExample')"
          :disabled="busy"
        />
      </Field>
      <Field id="group-parent" :label="t('group.parent')">
        <Select
          id="group-parent"
          v-model="parentId"
          :disabled="busy"
          :options="[{ value: 0, label: t('group.root') }, ...parentOptions.map((group) => ({ value: group.id, label: group.label }))]"
        />
      </Field>
      <div class="form-actions">
        <Button type="button" variant="outline" :disabled="busy" @click="open = false">
          {{ t('common.cancel') }}
        </Button>
        <Button type="submit" :disabled="busy">{{ t('common.save') }}</Button>
      </div>
    </form>
  </Modal>
</template>
