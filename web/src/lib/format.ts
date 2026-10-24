import { i18n } from '@/i18n'
import { timezone } from '@/composables/useApp'

export function formatTime(value?: number | null) {
  if (!value) return '—'
  const options: Intl.DateTimeFormatOptions = {
    timeZone: timezone.value,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }
  try {
    return new Intl.DateTimeFormat('zh-CN', options).format(value)
  } catch {
    // 浏览器时区目录落后时明确回退 UTC，避免日期格式化中断整个页面。
    return new Intl.DateTimeFormat('zh-CN', { ...options, timeZone: 'UTC' }).format(value) + ' UTC'
  }
}
export function percent(value?: number | null) {
  if (value == null) return '—'
  // 不将接近 100% 的故障数据四舍五入成 100%。
  return `${(Math.floor(value * 100) / 100).toFixed(2)}%`
}
export function latency(value?: number | null) {
  if (value == null) return '—'
  return `${value.toFixed(0)} ms`
}
export function certificate(expiry?: number) {
  if (!expiry) return '—'
  const days = Math.ceil((expiry - Date.now()) / 86400000)
  if (days <= 0) return i18n.global.t('status.expired')
  return i18n.global.t('status.days', { count: days })
}
