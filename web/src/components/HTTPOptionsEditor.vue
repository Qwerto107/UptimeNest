<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { HTTPOptions } from '@/types'
import Field from '@/components/Field.vue'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Select } from '@/components/ui/select'

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
        <Select
          id="http-version"
          v-model="model.version"
          :disabled="busy"
          :options="(['http1', 'http2', 'http3', 'standard', 'auto'] as const).map((version) => ({ value: version, label: t('http.version_' + version) }))"
        />
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
      <Field
        id="http-connect-ip"
        :label="t('http.connectIP')"
        :hint="t('http.connectIPHint')"
        class="col-span-full"
      >
        <Input
          id="http-connect-ip"
          v-model="model.connectIP"
          :placeholder="t('http.connectIPExample')"
          maxlength="45"
          spellcheck="false"
        />
      </Field>
      <div class="toggle-row col-span-full">
        <div>
          <label for="http-ignore-tls">{{ t('http.ignoreTLS') }}</label>
          <p id="http-ignore-tls-hint">{{ t('http.ignoreTLSHint') }}</p>
        </div>
        <Switch
          id="http-ignore-tls"
          v-model="model.ignoreTLS"
          aria-describedby="http-ignore-tls-hint"
        />
      </div>
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
        <Select
          id="http-method"
          v-model="model.method"
          :disabled="busy"
          :options="methods.map((method) => ({ value: method, label: method }))"
        />
      </Field>
      <Field id="http-encoding" :label="t('http.bodyEncoding')">
        <Select
          id="http-encoding"
          v-model="model.bodyEncoding"
          :disabled="busy"
          :options="(['json', 'form', 'xml', 'text'] as const).map((encoding) => ({ value: encoding, label: t('http.encoding_' + encoding) }))"
        />
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
</template>
