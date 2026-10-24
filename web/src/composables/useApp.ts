import { computed, ref } from 'vue'
import { i18n } from '@/i18n'
import type { Bootstrap } from '@/types'
import { request } from '@/lib/api'

export const bootstrap = ref<Bootstrap>()
export const timezone = ref('Asia/Singapore')
export const siteName = computed(() => bootstrap.value?.siteName || i18n.global.t('common.brand'))
export const dark = ref(false)

export function setTheme(value: boolean) {
  dark.value = value
  document.documentElement.classList.toggle('dark', value)
  try {
    localStorage.setItem('uptimenest-theme', value ? 'dark' : 'light')
  } catch {
    /* 浏览器禁用存储时仍允许当前会话切换。 */
  }
}

export function initTheme() {
  try {
    dark.value = localStorage.getItem('uptimenest-theme') === 'dark'
  } catch {
    dark.value = false
  }
  document.documentElement.classList.toggle('dark', dark.value)
}

export async function loadBootstrap() {
  const value = await request<Bootstrap>('/public/bootstrap')
  bootstrap.value = value
  timezone.value = value.timezone
  return value
}
