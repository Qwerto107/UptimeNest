type Measurement = () => (() => void) | undefined

const pending = new Set<Measurement>()
let frame = 0

export function scheduleHeartbeatLayout(measure: Measurement) {
  pending.add(measure)
  if (frame) return
  frame = requestAnimationFrame(() => {
    frame = 0
    const measurements = [...pending]
    pending.clear()
    // 全部心跳条先读取尺寸，再统一更新，避免各行交错读写触发重复布局。
    const updates = measurements.map(measure => measure())
    for (const update of updates) update?.()
  })
}

export function cancelHeartbeatLayout(measure: Measurement) {
  pending.delete(measure)
  if (!pending.size) {
    cancelAnimationFrame(frame)
    frame = 0
  }
}
