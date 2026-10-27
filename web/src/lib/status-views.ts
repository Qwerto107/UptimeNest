import type { PublicMonitor, StatusPageView } from '@/types'

// 详情跟在所在卡片行的最后一张卡片之后，避免展开时把同排的其他卡片挤到下一行。
export function statusViewItems(monitors: PublicMonitor[], view: StatusPageView, columns: number) {
  const entries: { key: string; monitor: PublicMonitor; details: boolean; column: number }[] = []
  const size = view === 'cards' ? Math.max(1, columns) : monitors.length || 1
  for (let start = 0; start < monitors.length; start += size) {
    const row = monitors.slice(start, start + size)
    row.forEach((monitor, column) => entries.push({ key: `monitor-${monitor.id}`, monitor, details: false, column }))
    if (view === 'cards') {
      row.forEach((monitor, column) => entries.push({ key: `details-${monitor.id}`, monitor, details: true, column }))
    }
  }
  return entries
}
