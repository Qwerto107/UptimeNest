import type { Group, MonitorStatus, MonitorView } from '@/types'

export interface GroupBranch {
  group: Group
  children: GroupBranch[]
  monitors: MonitorView[]
  descendants: MonitorView[]
  status: MonitorStatus
}

// 汇总完整子树状态；筛选只是改变可见项，不把隐藏的故障变成正常。
export function groupStatus(monitors: MonitorView[]): MonitorStatus {
  if (!monitors.length) return 'unknown'
  if (monitors.some((monitor) => monitor.summary.status === 'down')) return 'down'
  if (monitors.every((monitor) => monitor.summary.status === 'paused')) return 'paused'
  if (monitors.some((monitor) => monitor.summary.status === 'unknown')) return 'unknown'
  return 'up'
}

export function groupTree(groups: Group[], monitors: MonitorView[]): GroupBranch[] {
  const build = (parentId: number, visited: Set<number>): GroupBranch[] => {
    return groups
      .filter((group) => (group.parentId || 0) === parentId && !visited.has(group.id))
      .map((group) => {
        const next = new Set(visited).add(group.id)
        const children = build(group.id, next)
        const direct = monitors.filter((monitor) => monitor.groupId === group.id)
        const descendants = [...direct, ...children.flatMap((child) => child.descendants)]
        return { group, children, monitors: direct, descendants, status: groupStatus(descendants) }
      })
  }
  return build(0, new Set())
}

export function groupChoices(groups: Group[]) {
  const result: { id: number; label: string }[] = []
  const visit = (branches: GroupBranch[], prefix: string) => {
    for (const branch of branches) {
      let label = branch.group.name
      if (prefix) label = prefix + ' / ' + label
      result.push({ id: branch.group.id, label })
      visit(branch.children, label)
    }
  }
  visit(groupTree(groups, []), '')
  return result
}

export function descendantGroups(groups: Group[], id: number): Set<number> {
  const result = new Set([id])
  const pending = [id]
  while (pending.length) {
    const parent = pending.pop()
    for (const group of groups) {
      if (group.parentId === parent && !result.has(group.id)) {
        result.add(group.id)
        pending.push(group.id)
      }
    }
  }
  return result
}
