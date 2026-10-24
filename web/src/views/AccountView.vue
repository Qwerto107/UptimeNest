<script setup lang="ts">
import { reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { send } from '@/lib/api'
import { useAction, notify } from '@/composables/useFeedback'
import Field from '@/components/Field.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
const { t } = useI18n()
const { busy, run } = useAction()
const form = reactive({ current: '', password: '', confirmation: '' })
async function save() {
  if (form.password !== form.confirmation) {
    notify(t('auth.mismatch'), true)
    return
  }
  await run(async () => {
    await send('/admin/password', 'PUT', {
      current: form.current,
      password: form.password,
    })
    Object.assign(form, { current: '', password: '', confirmation: '' })
    notify(t('settings.passwordChanged'))
  })
}
</script>
<template>
  <div class="page-heading">
    <div>
      <h1>{{ t('settings.accountTitle') }}</h1>
      <p>{{ t('settings.accountSubtitle') }}</p>
    </div>
  </div>
  <form class="settings-form" @submit.prevent="save">
    <section class="settings-section">
      <div class="settings-section-title">
        <h2>{{ t('settings.password') }}</h2>
        <p>{{ t('settings.passwordHint') }}</p>
      </div>
      <fieldset :disabled="busy" class="settings-fields">
        <Field id="current-password" :label="t('settings.currentPassword')" required>
          <Input
            id="current-password"
            v-model="form.current"
            type="password"
            autocomplete="current-password"
            required
          />
        </Field>
        <Field
          id="new-password"
          :label="t('settings.newPassword')"
          :hint="t('auth.passwordHint')"
          required
        >
          <Input
            id="new-password"
            v-model="form.password"
            type="password"
            autocomplete="new-password"
            required
          />
        </Field>
        <Field id="confirm-password" :label="t('settings.confirmPassword')" required>
          <Input
            id="confirm-password"
            v-model="form.confirmation"
            type="password"
            autocomplete="new-password"
            required
          />
        </Field>
      </fieldset>
    </section>
    <div class="settings-save">
      <Button type="submit" :disabled="busy">
        {{ busy ? t('common.saving') : t('settings.password') }}
      </Button>
    </div>
  </form>
</template>
