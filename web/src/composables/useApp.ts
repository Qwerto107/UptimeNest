import { computed, ref } from 'vue'
import { i18n } from '@/i18n'
import type { Bootstrap } from '@/types'
import { request } from '@/lib/api'

export const bootstrap = ref<Bootstrap>()
export const timezone = ref('Asia/Singapore')
export const siteName = computed(() => bootstrap.value?.siteName || i18n.global.t('common.brand'))
export const dark = ref(false)
let bootstrapRequest: Promise<Bootstrap> | undefined
let bootstrapLoadedAt = 0

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

export async function loadBootstrap(maxAge = 60000) {
  if (bootstrap.value && Date.now() - bootstrapLoadedAt < maxAge) return bootstrap.value
  if (bootstrapRequest) return bootstrapRequest
  bootstrapRequest = (async () => {
    const value = await request<Bootstrap>('/public/bootstrap', { revalidate: true })
    // 未识别的语言回退英语，页面语言标记与实际文案保持一致。
    const language = value.language === 'zh-CN' ? 'zh-CN' : 'en-US'
    i18n.global.locale.value = language
    document.documentElement.lang = language
    bootstrap.value = value
    timezone.value = value.timezone
    bootstrapLoadedAt = Date.now()
    return value
  })()
  try { return await bootstrapRequest }
  finally { bootstrapRequest = undefined }
}
