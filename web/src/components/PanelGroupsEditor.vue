<script setup lang="ts">
import { computed, onBeforeUnmount, useId } from 'vue'
import { useEventListener, useMediaQuery } from '@vueuse/core'
import { VueDraggable } from 'vue-draggable-plus'
import type { DraggableEvent } from 'vue-draggable-plus'
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
const monitorDragGroup = `panel-monitors-${useId()}`
const reducedMotion = useMediaQuery('(prefers-reduced-motion: reduce)')
const dragAnimation = computed(() => {
  if (reducedMotion.value) return 0
  return 180
})
// 使用整项浮层跟随指针；让位时只移动相邻项，触屏和滚动容器采用同一套排序行为。
const dragOptions = {
  forceFallback: true,
  // 浮层留在编辑区域内，弹窗关闭时随其一起移除。
  fallbackOnBody: false,
  fallbackTolerance: 4,
  ghostClass: 'panel-sort-placeholder',
  chosenClass: 'panel-sort-chosen',
  fallbackClass: 'panel-sort-preview',
  easing: 'cubic-bezier(0.16, 1, 0.3, 1)' as const,
  scroll: false,
}
let scrollParent: HTMLElement | null = null
let scrollFrame: number | undefined
let pointerY = 0
let lastScrollAt = 0
function trackPointer(event: MouseEvent | TouchEvent) {
  if ('touches' in event) {
    pointerY = event.touches[0]?.clientY ?? pointerY
    return
  }
  pointerY = event.clientY
}
function scrollDuringDrag(time: number) {
  if (!scrollParent) return
  const step = Math.min(time - lastScrollAt, 32) * 0.5
  lastScrollAt = time
  const bounds = scrollParent.getBoundingClientRect()
  const footer = scrollParent.querySelector('.form-actions')?.getBoundingClientRect()
  const bottom = footer?.top ?? bounds.bottom
  // 固定操作栏上方才是列表的可见下边缘，不能按整个表单底部判断。
  if (pointerY < bounds.top + 48) scrollParent.scrollTop -= step
  else if (pointerY > bottom - 48) scrollParent.scrollTop += step
  scrollFrame = requestAnimationFrame(scrollDuringDrag)
}
function stopScrolling() {
  if (scrollFrame !== undefined) cancelAnimationFrame(scrollFrame)
  scrollFrame = undefined
  scrollParent = null
}
function startScrolling(event: DraggableEvent) {
  stopScrolling()
  scrollParent = event.from.closest<HTMLElement>('.editor-form')
  lastScrollAt = performance.now()
  scrollFrame = requestAnimationFrame(scrollDuringDrag)
}
useEventListener(window, 'pointermove', trackPointer, { capture: true, passive: true })
useEventListener(window, 'touchmove', trackPointer, { capture: true, passive: true })
onBeforeUnmount(stopScrolling)

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
function moveMonitor(source: PanelGroup, id: number, targetId: string) {
  const target = groups.value.find((group) => group.id === targetId)
  const current = source.monitorIds.indexOf(id)
  if (!target || current < 0) return
  source.monitorIds.splice(current, 1)
  target.monitorIds.push(id)
}
function changeGroup(source: PanelGroup, id: number, event: Event) {
  const select = event.target as HTMLSelectElement
  moveMonitor(source, id, select.value)
  select.value = ''
}
</script>

<template>
  <fieldset class="form-section" :disabled="busy">
    <legend>{{ t('panel.displayGroups') }}</legend>
    <p class="field-hint">{{ t('panel.groupHint') }}</p>
    <VueDraggable
      v-model="groups"
      v-bind="dragOptions"
      class="panel-groups-editor"
      draggable=".panel-display-group"
      handle=".panel-group-drag-handle"
      :animation="dragAnimation"
      :disabled="busy"
      @start="startScrolling"
      @end="stopScrolling"
    >
      <section
        v-for="(group, groupIndex) in groups"
        :key="group.id"
        class="panel-display-group"
        :data-group-id="group.id"
      >
        <div class="panel-group-heading">
          <button
            type="button"
            class="panel-drag-handle panel-group-drag-handle"
            :aria-label="t('panel.dragGroup', { name: group.name })"
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
        <VueDraggable
          v-model="group.monitorIds"
          v-bind="dragOptions"
          class="panel-group-monitors"
          draggable=".panel-group-monitor"
          handle=".panel-monitor-drag-handle"
          :group="monitorDragGroup"
          :animation="dragAnimation"
          :disabled="busy"
          :empty-insert-threshold="24"
          @start="startScrolling"
          @end="stopScrolling"
        >
          <div
            v-for="(id, index) in group.monitorIds"
            :key="id"
            class="panel-group-monitor"
            :data-monitor-id="id"
          >
            <button
              type="button"
              class="panel-drag-handle panel-monitor-drag-handle"
              :aria-label="t('panel.dragMonitor', { name: monitorName(id) })"
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
                <option value="" disabled selected>{{ t('panel.moveTo') }}</option>
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
        </VueDraggable>
        <select
          class="control"
          value=""
          :disabled="!available.length"
          :aria-label="t('panel.addMonitorTo', { name: group.name })"
          @change="addMonitor(group, $event)"
        >
          <option value="" disabled selected>
            {{ available.length ? t('panel.addMonitor') : t('panel.allSelected') }}
          </option>
          <option v-for="monitor in available" :key="monitor.id" :value="monitor.id">
            {{ monitor.name }}
          </option>
        </select>
      </section>
    </VueDraggable>
    <Button type="button" variant="outline" :disabled="groups.length >= 100" @click="addGroup">
      <Plus />
      {{ t('panel.addGroup') }}
    </Button>
    <p v-if="!monitors.length" class="field-hint">{{ t('panel.noMonitors') }}</p>
  </fieldset>
</template>
