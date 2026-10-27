<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { HTTPOptions } from '@/types'
import Field from '@/components/Field.vue'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select } from '@/components/ui/select'

const model = defineModel<HTTPOptions>({ required: true })
withDefaults(defineProps<{ busy: boolean; configured?: string[] }>(), { configured: () => [] })
const { t } = useI18n()
</script>

<template>
  <fieldset :disabled="busy" class="form-section">
    <legend>{{ t('http.authentication') }}</legend>
    <div class="form-grid">
      <Field id="http-auth" :label="t('http.authType')" class="col-span-full">
        <Select
          id="http-auth"
          v-model="model.auth.type"
          :disabled="busy"
          :options="(['none', 'basic', 'oauth2', 'mtls'] as const).map((type) => ({ value: type, label: t('http.auth_' + type) }))"
        />
      </Field>
      <template v-if="model.auth.type === 'basic'">
        <Field id="http-username" :label="t('http.username')" required>
          <Input id="http-username" v-model="model.auth.username" required autocomplete="off" />
        </Field>
        <Field id="http-password" :label="t('http.password')">
          <Input
            id="http-password"
            v-model="model.auth.password"
            :placeholder="configured.includes('http.auth.password') ? t('monitorSecrets.placeholder') : undefined"
            type="password"
            autocomplete="new-password"
          />
        </Field>
      </template>
      <template v-if="model.auth.type === 'oauth2'">
        <p class="field-hint col-span-full">{{ t('http.oauthHint') }}</p>
        <Field id="oauth-token-url" :label="t('http.tokenURL')" class="col-span-full" required>
          <Input id="oauth-token-url" v-model="model.auth.tokenURL" type="url" required />
        </Field>
        <Field id="oauth-client-id" :label="t('http.clientID')" required>
          <Input id="oauth-client-id" v-model="model.auth.clientID" required autocomplete="off" />
        </Field>
        <Field id="oauth-client-secret" :label="t('http.clientSecret')" :required="!configured.includes('http.auth.clientSecret')">
          <Input
            id="oauth-client-secret"
            v-model="model.auth.clientSecret"
            :placeholder="configured.includes('http.auth.clientSecret') ? t('monitorSecrets.placeholder') : undefined"
            type="password"
            :required="!configured.includes('http.auth.clientSecret')"
            autocomplete="new-password"
          />
        </Field>
        <Field id="oauth-scope" :label="t('http.scope')" :hint="t('http.scopeHint')">
          <Input id="oauth-scope" v-model="model.auth.scope" />
        </Field>
        <Field id="oauth-auth-method" :label="t('http.tokenAuthMethod')">
          <Select
            id="oauth-auth-method"
            v-model="model.auth.tokenAuthMethod"
            :disabled="busy"
            :options="(['header', 'body'] as const).map((method) => ({ value: method, label: t('http.tokenAuth_' + method) }))"
          />
        </Field>
      </template>
      <template v-if="model.auth.type === 'mtls'">
        <Field
          id="mtls-cert"
          :label="t('http.clientCert')"
          :hint="t('http.clientCertHint')"
          class="col-span-full"
          required
        >
          <Textarea
            id="mtls-cert"
            v-model="model.auth.clientCert"
            :rows="4"
            :maxlength="65536"
            spellcheck="false"
            required
          />
        </Field>
        <Field
          id="mtls-key"
          :label="t('http.clientKey')"
          :hint="t('http.clientKeyHint')"
          class="col-span-full"
          :required="!configured.includes('http.auth.clientKey')"
        >
          <Textarea
            id="mtls-key"
            v-model="model.auth.clientKey"
            :placeholder="configured.includes('http.auth.clientKey') ? t('monitorSecrets.placeholder') : undefined"
            :rows="4"
            :maxlength="65536"
            spellcheck="false"
            :required="!configured.includes('http.auth.clientKey')"
          />
        </Field>
        <Field
          id="mtls-ca"
          :label="t('http.caCert')"
          :hint="t('http.caCertHint')"
          class="col-span-full"
        >
          <Textarea
            id="mtls-ca"
            v-model="model.auth.caCert"
            :rows="3"
            :maxlength="65536"
            spellcheck="false"
          />
        </Field>
      </template>
    </div>
  </fieldset>
</template>
