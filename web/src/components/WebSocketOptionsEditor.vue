<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WebSocketOptions } from '@/types'
import Field from '@/components/Field.vue'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select } from '@/components/ui/select'

const model = defineModel<WebSocketOptions>({ required: true })
withDefaults(defineProps<{ busy: boolean; configured?: string[] }>(), { configured: () => [] })
const { t } = useI18n()
const protocols = ref('')
watch(() => model.value, (value) => { protocols.value = value.subprotocols.join(', ') }, { immediate: true })
// 保留逗号和空格的输入状态，只规范化提交给后端的协议列表。
watch(protocols, (value) => { model.value.subprotocols = value.split(',').map((item) => item.trim()).filter(Boolean) })
</script>

<template>
  <fieldset :disabled="busy" class="form-section">
    <legend>{{ t('websocket.options') }}</legend>
    <div class="form-grid">
      <Field id="websocket-mode" :label="t('websocket.mode')" :hint="t('websocket.modeHint')" class="col-span-full">
        <Select id="websocket-mode" v-model="model.mode" :disabled="busy" :options="(['handshake', 'ping'] as const).map((mode) => ({ value: mode, label: t(`websocket.mode_${mode}`) }))" />
      </Field>
      <Field id="websocket-origin" label="Origin" :hint="t('websocket.originHint')">
        <Input id="websocket-origin" v-model="model.origin" placeholder="https://example.com" maxlength="2048" spellcheck="false" />
      </Field>
      <Field id="websocket-subprotocols" :label="t('websocket.subprotocols')" :hint="t('websocket.subprotocolsHint')">
        <Input id="websocket-subprotocols" v-model="protocols" placeholder="graphql-transport-ws" spellcheck="false" />
      </Field>
      <Field id="websocket-connect-ip" :label="t('http.connectIP')" :hint="t('websocket.connectIPHint')" class="col-span-full">
        <Input id="websocket-connect-ip" v-model="model.connectIP" :placeholder="t('http.connectIPExample')" maxlength="45" spellcheck="false" />
      </Field>
      <Field id="websocket-headers" :label="t('http.headers')" :hint="t('websocket.headersHint')" class="col-span-full">
        <Textarea id="websocket-headers" v-model="model.headers" :placeholder="configured.includes('websocket.headers') ? t('monitorSecrets.placeholder') : t('http.headersExample')" maxlength="65536" rows="5" spellcheck="false" />
      </Field>
    </div>
    <p class="field-hint">{{ t('websocket.tlsHint') }}</p>
  </fieldset>
</template>
