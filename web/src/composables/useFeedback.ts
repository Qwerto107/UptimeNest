import { ref } from 'vue'
import { i18n } from '@/i18n'

export const feedback = ref<{ id: number; text: string; error: boolean } | null>(null)
let timer: ReturnType<typeof setTimeout>
let nextID = 0

export function notify(text: string, error = false) {
  clearTimeout(timer)
  feedback.value = { id: ++nextID, text, error }
  timer = setTimeout(() => {
    feedback.value = null
  }, error ? 6000 : 3000)
}

export function clearSuccess() {
  if (feedback.value && !feedback.value.error) {
    clearTimeout(timer)
    feedback.value = null
  }
}

export function useAction() {
  const busy = ref(false)
  async function run(action: () => Promise<void>) {
    if (busy.value) return false
    busy.value = true
    try {
      await action()
      return true
    } catch (error) {
      notify(error instanceof Error ? error.message : i18n.global.t('common.requestError'), true)
      return false
    } finally {
      busy.value = false
    }
  }
  return { busy, run }
}
