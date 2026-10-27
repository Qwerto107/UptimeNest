<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AlertOptions, NotificationChannel } from '@/types'
import { request } from '@/lib/api'
import { defaultAlerts } from '@/lib/notifications'
import Field from '@/components/Field.vue'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
const props = defineProps<{ modelValue?: AlertOptions; active: boolean; certificateAvailable: boolean; busy: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: AlertOptions] }>()
const { t } = useI18n()
const channels = ref<NotificationChannel[]>([])
const loading = ref(false)
const error = ref('')
const rules = computed(() => props.modelValue || defaultAlerts())
function update(kind: keyof AlertOptions, patch: Partial<AlertOptions['service']>) {
  emit('update:modelValue', { ...rules.value, [kind]: { ...rules.value[kind], ...patch } })
}
function select(kind: keyof AlertOptions, id: number, checked: boolean) {
  const ids = rules.value[kind].channelIds.filter(value => value !== id)
  if (checked) ids.push(id)
  update(kind, { channelIds: ids })
}
async function load() {
  loading.value = true
  try { channels.value = await request<NotificationChannel[]>('/admin/notification-channels'); error.value = '' }
  catch (cause) { error.value = cause instanceof Error ? cause.message : t('common.requestError') }
  finally { loading.value = false }
}
watch(() => props.active, value => { if (value) void load() }, { immediate: true })
watch(() => props.certificateAvailable, value => { if (!value && rules.value.certificate.enabled) update('certificate', { enabled: false }) })
</script>
<template>
  <div class="monitor-alert-rules">
    <p class="field-hint">{{ t('notification.monitorHint') }}</p>
    <p v-if="error" role="alert" class="text-destructive">{{ error }} <Button type="button" variant="outline" size="sm" @click="load">{{ t('common.retry') }}</Button></p>
    <p v-if="loading" role="status" class="field-hint">{{ t('common.loading') }}</p>
    <fieldset v-for="kind in (['service', 'certificate'] as const)" :key="kind" class="notification-rule" :disabled="busy">
      <legend>{{ t(`notification.${kind}`) }}</legend>
      <div class="notification-toggle">
        <label :for="`alert-${kind}`">{{ t('notification.enableAlert') }}</label>
        <Switch :id="`alert-${kind}`" :model-value="rules[kind].enabled" :disabled="kind === 'certificate' && !certificateAvailable" @update:model-value="update(kind, { enabled: $event })" />
      </div>
      <p v-if="kind === 'certificate' && !certificateAvailable" class="field-hint">{{ t('notification.tlsOnly') }}</p>
      <div v-if="rules[kind].enabled" class="mt-4 space-y-4">
        <p v-if="!loading && !channels.length" class="field-hint">{{ t('notification.noChannels') }}</p>
        <div class="notification-channel-choices" role="group" :aria-label="t('notification.channels')">
          <label v-for="channel in channels" :key="channel.id" class="notification-check">
            <input type="checkbox" :checked="rules[kind].channelIds.includes(channel.id)" @change="select(kind, channel.id, ($event.target as HTMLInputElement).checked)">
            {{ channel.name }} <span class="field-hint">{{ t(`notification.types.${channel.type}`) }}{{ channel.enabled ? '' : ` · ${t('common.disabled')}` }}</span>
          </label>
        </div>
        <div class="form-grid">
          <Field v-if="kind === 'service'" id="alert-failure-threshold" :label="t('notification.failureThreshold')" :hint="t('notification.failureThresholdHint')" required>
            <Input id="alert-failure-threshold" type="number" min="1" max="1000" :model-value="rules.service.failureThreshold ?? 1" required @update:model-value="update('service', { failureThreshold: Number($event) })" />
          </Field>
          <Field v-if="kind === 'certificate'" id="alert-days" :label="t('notification.days')" :hint="t('notification.daysHint')" required>
            <Input id="alert-days" type="number" min="1" max="3650" :model-value="rules.certificate.days || ''" required @update:model-value="update('certificate', { days: Number($event) })" />
          </Field>
          <Field :id="`repeat-${kind}`" :label="t('notification.repeatMinutes')" :hint="t('notification.repeatHint')">
            <Input :id="`repeat-${kind}`" type="number" min="0" max="525600" :model-value="rules[kind].repeatMinutes" @update:model-value="update(kind, { repeatMinutes: Number($event) })" />
          </Field>
        </div>
        <div class="notification-toggle">
          <label :for="`recovery-${kind}`">{{ t('notification.recovery') }}</label>
          <Switch :id="`recovery-${kind}`" :model-value="rules[kind].recovery" @update:model-value="update(kind, { recovery: $event })" />
        </div>
      </div>
    </fieldset>
  </div>
</template>
