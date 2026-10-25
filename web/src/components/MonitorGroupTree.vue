<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, ChevronRight, Folder, FolderOpen } from 'lucide-vue-next'
import type { Group, MonitorView } from '@/types'
import { groupTree, type GroupBranch } from '@/lib/groups'
import StatusBadge from '@/components/StatusBadge.vue'
import PaginationControls from '@/components/PaginationControls.vue'
import { usePagination } from '@/composables/usePagination'

const props = withDefaults(
  defineProps<{
    groups: Group[]
    monitors: MonitorView[]
    visibleIds?: number[]
    visibleGroupIds?: number[]
    searching?: boolean
    filterKey?: string
  }>(),
  { searching: false, filterKey: '' },
)
const { t } = useI18n()
const collapsed = ref(new Set<number>())
watch(
  () => props.searching,
  (value) => {
    if (value) collapsed.value.clear()
  },
)
type Row =
  | { kind: 'group'; branch: GroupBranch; depth: number }
  | { kind: 'monitor'; monitor: MonitorView; depth: number }
const rows = computed(() => {
  const result: Row[] = []
  const visible = new Set(props.visibleIds || props.monitors.map((monitor) => monitor.id))
  const visibleGroups = new Set(props.visibleGroupIds)
  const visit = (branches: GroupBranch[], depth: number) => {
    for (const branch of branches) {
      if (
        props.searching &&
        !visibleGroups.has(branch.group.id) &&
        !branch.descendants.some((monitor) => visible.has(monitor.id))
      )
        continue
      result.push({ kind: 'group', branch, depth })
      if (collapsed.value.has(branch.group.id)) continue
      visit(branch.children, depth + 1)
      for (const monitor of branch.monitors) {
        if (visible.has(monitor.id)) result.push({ kind: 'monitor', monitor, depth: depth + 1 })
      }
    }
  }
  visit(groupTree(props.groups, props.monitors), 0)
  const grouped = new Set(props.groups.map((group) => group.id))
  for (const monitor of props.monitors) {
    if (!grouped.has(monitor.groupId) && visible.has(monitor.id))
      result.push({ kind: 'monitor', monitor, depth: 0 })
  }
  return result
})
const { page, pageSize, offset } = usePagination(computed(() => rows.value.length))
watch(() => props.filterKey, () => { page.value = 1 })
const pageRows = computed(() => {
  const visible = rows.value.slice(offset.value, offset.value + pageSize.value)
  const first = visible[0]
  if (!first) return []
  const groups = new Map(rows.value.filter((row) => row.kind === 'group').map((row) => [row.branch.group.id, row]))
  let parent = 0
  if (first.kind === 'group') parent = first.branch.group.parentId
  else parent = first.monitor.groupId
  const context: Row[] = []
  // 跨页时补上首行的父级标题，保留完整分组状态及折叠操作。
  while (parent) {
    const row = groups.get(parent)
    if (!row) break
    context.unshift(row)
    parent = row.branch.group.parentId
  }
  return [...context, ...visible]
})
function toggle(id: number) {
  if (collapsed.value.has(id)) collapsed.value.delete(id)
  else collapsed.value.add(id)
}
</script>

<template>
  <div class="monitor-tree monitor-list">
    <div
      v-for="row in pageRows"
      :key="row.kind + '-' + (row.kind === 'group' ? row.branch.group.id : row.monitor.id)"
      class="tree-entry"
      :class="{ 'tree-entry-nested': row.depth > 0 }"
      :style="{ '--tree-depth': row.depth }"
    >
      <div v-if="row.kind === 'group'" class="monitor-group-row">
        <button
          type="button"
          class="monitor-group-toggle"
          :aria-expanded="!collapsed.has(row.branch.group.id)"
          :aria-label="t('group.toggle', { name: row.branch.group.name })"
          @click="toggle(row.branch.group.id)"
        >
          <ChevronRight v-if="collapsed.has(row.branch.group.id)" :size="15" />
          <ChevronDown v-else :size="15" />
          <Folder v-if="collapsed.has(row.branch.group.id)" :size="18" />
          <FolderOpen v-else :size="18" />
          <span>{{ row.branch.group.name }}</span>
        </button>
        <span class="group-monitor-count">
          {{ t('group.count', { count: row.branch.descendants.length }) }}
        </span>
        <StatusBadge v-if="row.branch.descendants.length" :status="row.branch.status" />
        <div class="row-actions"><slot name="group-actions" :group="row.branch.group" /></div>
      </div>
      <slot v-else name="monitor" :monitor="row.monitor">
        <div class="group-monitor-leaf">
          <RouterLink :to="'/admin/monitors/' + row.monitor.id">{{ row.monitor.name }}</RouterLink>
          <StatusBadge :status="row.monitor.summary.status" />
        </div>
      </slot>
    </div>
  </div>
  <PaginationControls v-model:page="page" v-model:pageSize="pageSize" :total="rows.length" :label="t('pagination.monitorRows')" />
</template>
