<script setup lang="ts">
import { onBeforeUnmount, onMounted, watchEffect } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { CheckCircle2, AlertCircle, X } from 'lucide-vue-next'
import { feedback, notify } from '@/composables/useFeedback'
import { Button } from '@/components/ui/button'
import { loadBootstrap, siteName } from '@/composables/useApp'
const router = useRouter()
const { t } = useI18n()
watchEffect(() => {
  document.title = siteName.value
})
onMounted(() => void loadBootstrap().catch((error) => notify(error.message, true)))
function expired() {
  notify(t('auth.sessionExpired'), true)
  void router.replace('/admin/login')
}
onMounted(() => window.addEventListener('session-expired', expired))
onBeforeUnmount(() => window.removeEventListener('session-expired', expired))
</script>
<template>
  <RouterView v-slot="{ Component, route }">
    <component :is="Component" :key="route.path" />
  </RouterView>
  <Transition name="toast">
    <div
      v-if="feedback"
      class="toast"
      :class="{ 'toast-error': feedback.error }"
      :role="feedback.error ? 'alert' : 'status'"
    >
      <AlertCircle v-if="feedback.error" class="size-5 shrink-0" />
      <CheckCircle2 v-else class="size-5 shrink-0" />
      <p>{{ feedback.text }}</p>
      <Button variant="ghost" size="icon" :aria-label="t('common.close')" @click="feedback = null">
        <X />
      </Button>
    </div>
  </Transition>
</template>
