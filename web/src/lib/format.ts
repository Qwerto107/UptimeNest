import { computed } from 'vue'
import { i18n } from '@/i18n'
import { timezone } from '@/composables/useApp'

// 仅在语言或时区变化时重建格式化器，避免每个心跳重复创建。
const dateTimeFormat = computed(() => {
  const locale = i18n.global.locale.value
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
    return { formatter: new Intl.DateTimeFormat(locale, options), suffix: '' }
  } catch {
    // 浏览器时区目录落后时明确回退 UTC，避免日期格式化中断整个页面。
    return {
      formatter: new Intl.DateTimeFormat(locale, { ...options, timeZone: 'UTC' }),
      suffix: ' UTC',
    }
  }
})

export function formatTime(value?: number | null) {
  if (!value) return '—'
  const { formatter, suffix } = dateTimeFormat.value
  return formatter.format(value) + suffix
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
