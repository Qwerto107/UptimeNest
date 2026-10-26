import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { i18n } from '@/i18n'

// 自动刷新保留上次成功数据；卸载后取消请求，避免过期响应覆盖新页面。
export function useResource<T>(loader: (signal: AbortSignal) => Promise<T>, interval = 0) {
  const data = shallowRef<T>()
  const loading = ref(true)
  const refreshing = ref(false)
  const error = ref('')
  const secondsUntilRefresh = ref(0)
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let countdown: ReturnType<typeof setInterval> | undefined
  let nextRefreshAt = 0
  let disposed = false
  let version = 0
  function stopTimers() {
    clearTimeout(timer)
    clearInterval(countdown)
  }
  function updateCountdown() {
    secondsUntilRefresh.value = Math.max(0, Math.ceil((nextRefreshAt - Date.now()) / 1000))
  }
  function schedule() {
    stopTimers()
    if (disposed || !interval || document.visibilityState !== 'visible') return
    updateCountdown()
    timer = setTimeout(() => void refresh(), Math.max(0, nextRefreshAt - Date.now()))
    countdown = setInterval(updateCountdown, 1000)
  }
  function visibilityChanged() {
    if (document.visibilityState !== 'visible') stopTimers()
    else if (!refreshing.value) schedule()
  }
  async function refresh() {
    if (disposed) return
    stopTimers()
    secondsUntilRefresh.value = 0
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
        nextRefreshAt = Date.now() + interval
        schedule()
      }
    }
  }
  onMounted(() => {
    void refresh()
    if (interval) document.addEventListener('visibilitychange', visibilityChanged)
  })
  onBeforeUnmount(() => {
    disposed = true
    controller?.abort()
    stopTimers()
    document.removeEventListener('visibilitychange', visibilityChanged)
  })
  return { data, loading, refreshing, error, refresh, secondsUntilRefresh }
}
