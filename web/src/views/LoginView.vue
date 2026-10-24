<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import type { Session } from '@/types'
import { request, send } from '@/lib/api'
import { useResource } from '@/composables/useResource'
import { useAction, notify } from '@/composables/useFeedback'
import { loadBootstrap } from '@/composables/useApp'
import Brand from '@/components/Brand.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import LoadState from '@/components/LoadState.vue'
import Field from '@/components/Field.vue'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
const { t } = useI18n()
const router = useRouter()
const password = ref('')
const confirmation = ref('')
const { data, loading, error, refresh } = useResource((signal) =>
  request<Session>('/admin/session', { signal }),
)
const { busy, run } = useAction()
async function submit() {
  if (!data.value) return
  if (!data.value.initialized && password.value !== confirmation.value) {
    notify(t('auth.mismatch'), true)
    return
  }
  await run(async () => {
    const path = '/admin/' + (data.value?.initialized ? 'session' : 'setup')
    await send(path, 'POST', { password: password.value })
    await loadBootstrap()
    await router.replace('/admin')
  })
}
</script>
<template>
  <div class="login-page">
    <header class="login-header">
      <RouterLink to="/"><Brand /></RouterLink>
      <ThemeToggle />
    </header>
    <main class="login-main">
      <LoadState :loading="loading" :error="error" @retry="refresh" />
      <form v-if="data" class="login-form" @submit.prevent="submit">
        <h1>{{ data.initialized ? t('auth.welcome') : t('auth.setup') }}</h1>
        <p class="login-description">
          {{ data.initialized ? t('auth.loginDescription') : t('auth.setupDescription') }}
        </p>
        <Field id="login-password" :label="t('auth.password')" required>
          <Input
            id="login-password"
            v-model="password"
            type="password"
            required
            :autocomplete="data.initialized ? 'current-password' : 'new-password'"
            autofocus
            :disabled="busy"
          />
        </Field>
        <Field
          v-if="!data.initialized"
          id="login-confirm"
          :label="t('auth.confirmPassword')"
          :hint="t('auth.passwordHint')"
          required
        >
          <Input
            id="login-confirm"
            v-model="confirmation"
            type="password"
            required
            autocomplete="new-password"
            :disabled="busy"
          />
        </Field>
        <Button type="submit" class="w-full" :disabled="busy">
          {{ data.initialized ? t('auth.login') : t('auth.initialize') }}
        </Button>
        <p v-if="!data.initialized" class="field-hint">
          {{ t('auth.localSetup') }}
        </p>
        <RouterLink class="text-link text-sm" to="/">{{ t('nav.public') }}</RouterLink>
      </form>
    </main>
    <footer class="login-footer">{{ t('common.brand') }}</footer>
  </div>
</template>
