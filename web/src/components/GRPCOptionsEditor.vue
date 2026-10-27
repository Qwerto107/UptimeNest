<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { GRPCOptions } from '@/types'
import Field from '@/components/Field.vue'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select } from '@/components/ui/select'
const model = defineModel<GRPCOptions>({ required: true })
withDefaults(defineProps<{ busy: boolean; configured?: string[] }>(), { configured: () => [] })
const { t } = useI18n()
const metadataExample = '{"authorization": "Bearer …"}'
</script>
<template>
  <fieldset :disabled="busy" class="form-section">
    <legend>{{ t('grpc.options') }}</legend>
    <div class="form-grid">
      <p class="field-hint col-span-full">{{ t('grpc.healthHint') }}</p>
      <Field id="grpc-security" :label="t('grpc.security')" :hint="t('grpc.securityHint')">
        <Select id="grpc-security" v-model="model.security" :disabled="busy" :options="[{ value: 'tls', label: 'TLS' }, { value: 'none', label: t('grpc.plaintext') }]" />
      </Field>
      <Field id="grpc-service" :label="t('grpc.serviceName')" :hint="t('grpc.serviceHint')">
        <Input id="grpc-service" v-model="model.serviceName" maxlength="1000" placeholder="example.Service" spellcheck="false" />
      </Field>
      <Field id="grpc-metadata" :label="t('grpc.metadata')" :hint="t('grpc.metadataHint')" class="col-span-full">
        <Textarea id="grpc-metadata" v-model="model.metadata" rows="5" maxlength="65536" :placeholder="configured.includes('grpc.metadata') ? t('monitorSecrets.placeholder') : metadataExample" class="font-mono" spellcheck="false" />
      </Field>
      <Field v-if="model.security === 'tls'" id="grpc-ca" :label="t('grpc.caCert')" :hint="t('grpc.caHint')" class="col-span-full">
        <Textarea id="grpc-ca" v-model="model.caCert" rows="5" maxlength="65536" placeholder="-----BEGIN CERTIFICATE-----" class="font-mono" spellcheck="false" />
      </Field>
      <p class="field-hint col-span-full">{{ t('grpc.latencyHint') }}</p>
    </div>
  </fieldset>
</template>
