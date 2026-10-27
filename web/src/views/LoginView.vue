<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowUpRight, Eye, EyeOff, LockKeyhole } from 'lucide-vue-next'
import type { Session } from '@/types'
import { request, send } from '@/lib/api'
import { useResource } from '@/composables/useResource'
import { useAction, notify } from '@/composables/useFeedback'
import { loadBootstrap, siteName } from '@/composables/useApp'
import Brand from '@/components/Brand.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import LoadState from '@/components/LoadState.vue'
import Field from '@/components/Field.vue'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
const { t } = useI18n()
const router = useRouter()
const password = ref('')
const setupToken = ref('')
const confirmation = ref('')
const showPassword = ref(false)
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
    const body = data.value?.initialized
      ? { password: password.value }
      : { password: password.value, token: setupToken.value.trim() }
    await send(path, 'POST', body)
    setupToken.value = ''
    await loadBootstrap(0)
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
        <h1>{{ data.initialized ? t('auth.welcome') : t('auth.setup', { name: siteName }) }}</h1>
        <p class="login-description">
          {{ data.initialized ? t('auth.loginDescription') : t('auth.setupDescription') }}
        </p>
        <Field
          v-if="!data.initialized"
          id="setup-token"
          :label="t('auth.setupToken')"
          :hint="t('auth.setupTokenHint')"
          required
        >
          <Input
            id="setup-token"
            v-model="setupToken"
            type="password"
            required
            autocomplete="off"
            autofocus
            :spellcheck="false"
            :disabled="busy"
          />
        </Field>
        <Field id="login-password" :label="t('auth.password')" required>
          <div class="password-control">
            <LockKeyhole :size="14" aria-hidden="true" />
            <Input
              id="login-password"
              v-model="password"
              :type="showPassword ? 'text' : 'password'"
              :placeholder="t('auth.passwordPlaceholder')"
              required
              :autocomplete="data.initialized ? 'current-password' : 'new-password'"
              :autofocus="data.initialized"
              :disabled="busy"
            />
            <Button
              type="button"
              variant="ghost"
              size="icon"
              :disabled="busy"
              :aria-label="showPassword ? t('auth.hidePassword') : t('auth.showPassword')"
              :aria-pressed="showPassword"
              @click="showPassword = !showPassword"
            >
              <EyeOff v-if="showPassword" />
              <Eye v-else />
            </Button>
          </div>
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
        <RouterLink class="text-link login-public" to="/">
          {{ t('nav.public') }}
          <ArrowUpRight :size="13" />
        </RouterLink>
      </form>
    </main>
    <footer class="login-footer">{{ siteName }}</footer>
  </div>
</template>
