<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { HTTPOptions } from '@/types'
import Field from '@/components/Field.vue'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

const model = defineModel<HTTPOptions>({ required: true })
defineProps<{ busy: boolean }>()
const { t } = useI18n()
const ranges = ['100-199', '200-299', '300-399', '400-499', '500-599']
const methods = ['GET', 'POST', 'HEAD', 'PUT', 'PATCH', 'DELETE', 'OPTIONS', 'TRACE', 'CONNECT']
const customCodes = ref('')
watch(
  () => model.value,
  (value) => {
    customCodes.value = value.validStatusCodes.filter((code) => !ranges.includes(code)).join(', ')
  },
  { immediate: true },
)
// 保留输入中的逗号和空格，不在逐字输入时重写显示值。
watch(customCodes, (value) => {
  const selected = model.value.validStatusCodes.filter((code) => ranges.includes(code))
  const custom = value
    .split(/[,，]/)
    .map((code) => code.trim())
    .filter(Boolean)
  model.value.validStatusCodes = [...new Set([...selected, ...custom])]
})
</script>

<template>
  <fieldset :disabled="busy" class="form-section">
    <legend>{{ t('http.options') }}</legend>
    <div class="form-grid">
      <Field id="http-version" :label="t('http.version')" :hint="t('http.versionHint')">
        <select id="http-version" v-model="model.version" class="control">
          <option v-for="version in ['standard', 'auto', 'http3']" :key="version" :value="version">
            {{ t('http.version_' + version) }}
          </option>
        </select>
      </Field>
      <Field id="http-redirects" :label="t('http.redirects')" :hint="t('http.redirectsHint')">
        <Input
          id="http-redirects"
          v-model.number="model.maxRedirects"
          type="number"
          min="0"
          max="100"
          required
        />
      </Field>
      <div class="field col-span-full">
        <span id="http-codes-label" class="field-label">{{ t('http.statusCodes') }}</span>
        <div class="flex flex-wrap gap-x-4 gap-y-2" role="group" aria-labelledby="http-codes-label">
          <label
            v-for="range in ranges"
            :key="range"
            class="inline-flex items-center gap-2 text-sm"
          >
            <input
              v-model="model.validStatusCodes"
              type="checkbox"
              :value="range"
              class="accent-primary"
            />
            {{ range }}
          </label>
        </div>
      </div>
      <Field
        id="http-custom-codes"
        :label="t('http.customCodes')"
        :hint="t('http.statusCodesHint')"
        class="col-span-full"
      >
        <Input id="http-custom-codes" v-model="customCodes" :placeholder="t('http.codesExample')" />
      </Field>
      <Field id="http-method" :label="t('http.method')">
        <select id="http-method" v-model="model.method" class="control">
          <option v-for="method in methods" :key="method">{{ method }}</option>
        </select>
      </Field>
      <Field id="http-encoding" :label="t('http.bodyEncoding')">
        <select id="http-encoding" v-model="model.bodyEncoding" class="control">
          <option
            v-for="encoding in ['json', 'form', 'xml', 'text']"
            :key="encoding"
            :value="encoding"
          >
            {{ t('http.encoding_' + encoding) }}
          </option>
        </select>
      </Field>
      <Field
        id="http-body"
        :label="t('http.body')"
        :hint="t('http.bodyHint')"
        class="col-span-full"
      >
        <Textarea
          id="http-body"
          v-model="model.body"
          :rows="5"
          :maxlength="262144"
          spellcheck="false"
        />
      </Field>
      <Field
        id="http-headers"
        :label="t('http.headers')"
        :hint="t('http.headersHint')"
        class="col-span-full"
      >
        <Textarea
          id="http-headers"
          v-model="model.headers"
          :rows="4"
          :maxlength="65536"
          :placeholder="t('http.headersExample')"
          spellcheck="false"
        />
      </Field>
    </div>
  </fieldset>

  <fieldset :disabled="busy" class="form-section">
    <legend>{{ t('http.authentication') }}</legend>
    <div class="form-grid">
      <Field id="http-auth" :label="t('http.authType')" class="col-span-full">
        <select id="http-auth" v-model="model.auth.type" class="control">
          <option v-for="type in ['none', 'basic', 'oauth2', 'mtls']" :key="type" :value="type">
            {{ t('http.auth_' + type) }}
          </option>
        </select>
      </Field>
      <template v-if="model.auth.type === 'basic'">
        <Field id="http-username" :label="t('http.username')" required>
          <Input id="http-username" v-model="model.auth.username" required autocomplete="off" />
        </Field>
        <Field id="http-password" :label="t('http.password')">
          <Input
            id="http-password"
            v-model="model.auth.password"
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
        <Field id="oauth-client-secret" :label="t('http.clientSecret')" required>
          <Input
            id="oauth-client-secret"
            v-model="model.auth.clientSecret"
            type="password"
            required
            autocomplete="new-password"
          />
        </Field>
        <Field id="oauth-scope" :label="t('http.scope')" :hint="t('http.scopeHint')">
          <Input id="oauth-scope" v-model="model.auth.scope" />
        </Field>
        <Field id="oauth-auth-method" :label="t('http.tokenAuthMethod')">
          <select id="oauth-auth-method" v-model="model.auth.tokenAuthMethod" class="control">
            <option value="header">{{ t('http.tokenAuth_header') }}</option>
            <option value="body">{{ t('http.tokenAuth_body') }}</option>
          </select>
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
          required
        >
          <Textarea
            id="mtls-key"
            v-model="model.auth.clientKey"
            :rows="4"
            :maxlength="65536"
            spellcheck="false"
            required
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
