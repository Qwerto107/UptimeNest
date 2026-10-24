import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { i18n } from '@/i18n'

// 自动刷新保留上次成功数据；卸载后取消请求，避免过期响应覆盖新页面。
export function useResource<T>(loader: (signal: AbortSignal) => Promise<T>, interval = 0) {
  const data = shallowRef<T>()
  const loading = ref(true)
  const refreshing = ref(false)
  const error = ref('')
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setInterval> | undefined
  let disposed = false
  let version = 0
  async function refresh() {
    if (disposed) return
    // 用户操作后的刷新取消旧请求，避免旧统计覆盖已经清理或停用后的新状态。
    controller?.abort()
    const current = ++version
    refreshing.value = true
    controller = new AbortController()
    try {
      const value = await loader(controller.signal)
      if (!disposed && current === version) {
        data.value = value
        error.value = ''
      }
    } catch (cause) {
      if (
        !disposed &&
        current === version &&
        !(cause instanceof DOMException && cause.name === 'AbortError')
      ) {
        error.value = cause instanceof Error ? cause.message : i18n.global.t('common.requestError')
      }
    } finally {
      if (current === version) {
        loading.value = false
        refreshing.value = false
      }
    }
  }
  onMounted(() => {
    void refresh()
    if (interval)
      timer = setInterval(() => {
        if (document.visibilityState === 'visible') void refresh()
      }, interval)
  })
  onBeforeUnmount(() => {
    disposed = true
    controller?.abort()
    clearInterval(timer)
  })
  return { data, loading, refreshing, error, refresh }
}
