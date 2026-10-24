<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, GripVertical, ArrowUp, ArrowDown, X, Trash2 } from 'lucide-vue-next'
import type { PanelGroup, MonitorView } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const groups = defineModel<PanelGroup[]>({ required: true })
const props = defineProps<{ monitors: MonitorView[]; busy: boolean }>()
const { t } = useI18n()
const assigned = computed(() => new Set(groups.value.flatMap((group) => group.monitorIds)))
const available = computed(() =>
  props.monitors.filter((monitor) => !assigned.value.has(monitor.id)),
)
type Drag =
  | { kind: 'group'; groupId: string }
  | { kind: 'monitor'; groupId: string; monitorId: number }
const drag = ref<Drag>()

function addGroup() {
  groups.value.push({
    id: crypto.randomUUID(),
    name: t('panel.newGroup', { count: groups.value.length + 1 }),
    monitorIds: [],
  })
}
function shift<T>(items: T[], index: number, offset: number) {
  const target = index + offset
  if (target < 0 || target >= items.length) return
  const [item] = items.splice(index, 1)
  if (item !== undefined) items.splice(target, 0, item)
}
function monitorName(id: number) {
  return props.monitors.find((monitor) => monitor.id === id)?.name || String(id)
}
function addMonitor(group: PanelGroup, event: Event) {
  const select = event.target as HTMLSelectElement
  const id = Number(select.value)
  if (id && !assigned.value.has(id)) group.monitorIds.push(id)
  select.value = ''
}
function moveMonitor(source: PanelGroup, id: number, targetId: string, index?: number) {
  const target = groups.value.find((group) => group.id === targetId)
  const current = source.monitorIds.indexOf(id)
  if (!target || current < 0) return
  let next = index ?? target.monitorIds.length
  source.monitorIds.splice(current, 1)
  if (source === target && current < next) next--
  target.monitorIds.splice(next, 0, id)
}
function changeGroup(source: PanelGroup, id: number, event: Event) {
  const select = event.target as HTMLSelectElement
  moveMonitor(source, id, select.value)
  select.value = ''
}
function start(event: DragEvent, value: Drag) {
  if (props.busy) return
  drag.value = value
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', value.groupId)
  }
}
function drop(target: PanelGroup, index?: number) {
  if (props.busy) return
  const value = drag.value
  if (!value) return
  if (value.kind === 'group') {
    const current = groups.value.findIndex((group) => group.id === value.groupId)
    const next = groups.value.indexOf(target)
    if (current >= 0 && next >= 0 && current !== next) {
      const [group] = groups.value.splice(current, 1)
      if (group) groups.value.splice(next, 0, group)
    }
  } else {
    const source = groups.value.find((group) => group.id === value.groupId)
    if (source) moveMonitor(source, value.monitorId, target.id, index)
  }
  drag.value = undefined
}
</script>

<template>
  <fieldset class="form-section" :disabled="busy">
    <legend>{{ t('panel.displayGroups') }}</legend>
    <p class="field-hint">{{ t('panel.groupHint') }}</p>
    <div class="panel-groups-editor">
      <section
        v-for="(group, groupIndex) in groups"
        :key="group.id"
        class="panel-display-group"
        :data-group-id="group.id"
      >
        <div class="panel-group-heading" @dragover.prevent @drop.prevent="drop(group)">
          <button
            type="button"
            class="panel-drag-handle"
            draggable="true"
            :aria-label="t('panel.dragGroup', { name: group.name })"
            @dragstart="start($event, { kind: 'group', groupId: group.id })"
            @dragend="drag = undefined"
          >
            <GripVertical :size="16" />
          </button>
          <Input
            v-model="group.name"
            required
            maxlength="200"
            class="panel-group-name"
            :aria-label="t('panel.groupName', { count: groupIndex + 1 })"
          />
          <div class="row-actions">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              :disabled="groupIndex === 0"
              :aria-label="t('panel.moveUp', { name: group.name })"
              @click="shift(groups, groupIndex, -1)"
            >
              <ArrowUp />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              :disabled="groupIndex === groups.length - 1"
              :aria-label="t('panel.moveDown', { name: group.name })"
              @click="shift(groups, groupIndex, 1)"
            >
              <ArrowDown />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              class="text-destructive"
              :aria-label="t('panel.removeGroup', { name: group.name })"
              @click="groups.splice(groupIndex, 1)"
            >
              <Trash2 />
            </Button>
          </div>
        </div>
        <div class="panel-group-monitors" @dragover.prevent @drop.stop.prevent="drop(group)">
          <div
            v-for="(id, index) in group.monitorIds"
            :key="id"
            class="panel-group-monitor"
            @dragover.prevent
            @drop.stop.prevent="drop(group, index)"
          >
            <button
              type="button"
              class="panel-drag-handle"
              draggable="true"
              :aria-label="t('panel.dragMonitor', { name: monitorName(id) })"
              @dragstart.stop="start($event, { kind: 'monitor', groupId: group.id, monitorId: id })"
              @dragend="drag = undefined"
            >
              <GripVertical :size="15" />
            </button>
            <span class="panel-monitor-name">{{ monitorName(id) }}</span>
            <div class="row-actions">
              <select
                v-if="groups.length > 1"
                class="control panel-monitor-move"
                value=""
                :aria-label="t('panel.moveToGroup', { name: monitorName(id) })"
                @change="changeGroup(group, id, $event)"
              >
                <option value="" disabled>{{ t('panel.moveTo') }}</option>
                <option
                  v-for="target in groups.filter((item) => item.id !== group.id)"
                  :key="target.id"
                  :value="target.id"
                >
                  {{ target.name }}
                </option>
              </select>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                :disabled="index === 0"
                :aria-label="t('panel.moveUp', { name: monitorName(id) })"
                @click="shift(group.monitorIds, index, -1)"
              >
                <ArrowUp />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                :disabled="index === group.monitorIds.length - 1"
                :aria-label="t('panel.moveDown', { name: monitorName(id) })"
                @click="shift(group.monitorIds, index, 1)"
              >
                <ArrowDown />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                :aria-label="t('panel.removeMonitor', { name: monitorName(id) })"
                @click="group.monitorIds.splice(index, 1)"
              >
                <X />
              </Button>
            </div>
          </div>
          <p v-if="!group.monitorIds.length" class="field-hint panel-group-empty">
            {{ t('panel.emptyGroup') }}
          </p>
        </div>
        <select
          class="control"
          value=""
          :disabled="!available.length"
          :aria-label="t('panel.addMonitorTo', { name: group.name })"
          @change="addMonitor(group, $event)"
        >
          <option value="" disabled>
            {{ available.length ? t('panel.addMonitor') : t('panel.allSelected') }}
          </option>
          <option v-for="monitor in available" :key="monitor.id" :value="monitor.id">
            {{ monitor.name }}
          </option>
        </select>
      </section>
    </div>
    <Button type="button" variant="outline" :disabled="groups.length >= 100" @click="addGroup">
      <Plus />
      {{ t('panel.addGroup') }}
    </Button>
    <p v-if="!monitors.length" class="field-hint">{{ t('panel.noMonitors') }}</p>
  </fieldset>
</template>
