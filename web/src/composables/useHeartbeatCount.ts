import { ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'

// 同页按最宽的心跳条请求历史；延迟合并多行测量和连续缩放，避免重复请求。
export function useHeartbeatCount(initial: number, refresh: () => void) {
  // 首次请求覆盖整个视口能容纳的数量，避免先画灰色空槽、测量后再补取。
  // 与条柱相同地对齐物理像素，确保浏览器缩放时也不会低估所需历史。
  const ratio = window.devicePixelRatio || 1
  const size = Math.max(1, Math.round(3 * ratio))
  const viewportCount = Math.ceil((window.innerWidth * ratio + size) / (size * 2))
  const heartbeatCount = ref(Math.min(1000, Math.max(initial, viewportCount)))
  const refreshHistory = useDebounceFn(refresh, 250)
  function resizeHeartbeat(count: number) {
    if (count <= heartbeatCount.value) return
    heartbeatCount.value = Math.min(count, 1000)
    void refreshHistory()
  }
  return { heartbeatCount, resizeHeartbeat }
}
