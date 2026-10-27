<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { TabsRoot, TabsList, TabsTrigger, TabsContent } from 'reka-ui'
import { ChevronDown, Send } from 'lucide-vue-next'
import type { ChannelType, NotificationChannel, NotificationTemplate } from '@/types'
import { channelTypes, defaultChannel, templateIssue } from '@/lib/notifications'
import { send } from '@/lib/api'
import { useAction, notify } from '@/composables/useFeedback'
import Modal from '@/components/Modal.vue'
import Field from '@/components/Field.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

interface ChannelTemplate extends NotificationTemplate { format: string; certificateTemplate: NotificationTemplate }
export interface NotificationTemplates { variables: string[]; channels: Record<ChannelType, ChannelTemplate> }
const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ channel: NotificationChannel | null; templates: NotificationTemplates }>()
const emit = defineEmits<{ saved: [] }>()
const { t } = useI18n()
const { busy, run } = useAction()
const form = reactive(defaultChannel())
const editorForm = ref<HTMLFormElement>()
const testing = ref(false)
const testResult = ref<{ success: boolean; message: string }>()
const tab = ref('connection')
const recipients = ref('')
const headers = ref('')
const clearSecrets = ref<string[]>([])
const preview = ref<{ subject: string; body: string }>()
const previewType = ref('service_down')
const templateKind = ref<'service' | 'certificate'>('service')
const targetField = ref('body')
const activeVariable = ref('monitor_name')
const headersPlaceholder = '{"Authorization": "Bearer …"}'
const variableExample = '{{monitor_name}}'
const drafts = new Map<ChannelType, { subject: string; body: string; certificateTemplate: NotificationTemplate }>()
let loadingForm = false
const types = computed(() => channelTypes.map(value => ({ value, label: t(`notification.types.${value}`) })))
const eventTypes = computed(() => templateKind.value === 'certificate' ? ['certificate_expiring', 'certificate_expired', 'certificate_recovered'] : ['service_down', 'service_recovered'])
const currentTemplate = computed(() => props.templates.channels[form.type])
const activeTemplate = computed(() => templateKind.value === 'certificate' ? form.certificateTemplate! : form)
const subjectIssue = computed(() => form.type === 'webhook' ? undefined : templateIssue(activeTemplate.value.subject, props.templates.variables))
const bodyIssue = computed(() => templateIssue(activeTemplate.value.body, props.templates.variables, form.type === 'webhook'))
const issueText = (issue: NonNullable<ReturnType<typeof templateIssue>>) => t(`notification.syntaxErrors.${issue.kind}`, { name: issue.name, example: variableExample })
watch(open, value => {
  if (!value) return
  loadingForm = true
  drafts.clear()
  Object.assign(form, defaultChannel(), props.channel ? JSON.parse(JSON.stringify(props.channel)) : {})
  if (!props.channel) {
    form.subject = currentTemplate.value.subject
    form.body = currentTemplate.value.body
    form.certificateTemplate = { ...currentTemplate.value.certificateTemplate }
  } else form.certificateTemplate = { ...(props.channel.certificateTemplate || { subject: form.subject, body: form.body }) }
  recipients.value = (form.config.to || []).join('\n')
  headers.value = ''
  clearSecrets.value = []
  preview.value = undefined
  tab.value = 'connection'
  templateKind.value = 'service'
  previewType.value = 'service_down'
  targetField.value = 'body'
  activeVariable.value = 'monitor_name'
  testResult.value = undefined
  loadingForm = false
})
watch(() => form.type, (type, previous) => {
  if (loadingForm || !open.value || form.id) return
  // 新建时切换渠道保留各自草稿，不把邮件正文带入 Webhook 或机器人模板。
  drafts.set(previous, { subject: form.subject, body: form.body, certificateTemplate: { ...form.certificateTemplate! } })
  const next = drafts.get(type) || props.templates.channels[type]
  form.subject = next.subject
  form.body = next.body
  form.certificateTemplate = { ...next.certificateTemplate }
  targetField.value = 'body'
  preview.value = undefined
}, { flush: 'sync' })
watch(() => [activeTemplate.value.subject, activeTemplate.value.body, previewType.value], () => { preview.value = undefined })
watch(templateKind, () => {
  previewType.value = eventTypes.value[0]!
  preview.value = undefined
  testResult.value = undefined
  targetField.value = 'body'
})
watch(form, () => { testResult.value = undefined }, { deep: true })
watch([headers, recipients, clearSecrets], () => { testResult.value = undefined }, { deep: true })
function resetTemplate() {
  const defaults = templateKind.value === 'certificate' ? currentTemplate.value.certificateTemplate : currentTemplate.value
  activeTemplate.value.subject = defaults.subject
  activeTemplate.value.body = defaults.body
}
function previewBody() {
  if (!preview.value) return ''
  if (form.type === 'webhook') { try { return JSON.stringify(JSON.parse(preview.value.body), null, 2) } catch { return preview.value.body } }
  if (form.type === 'dingtalk') return `### ${preview.value.subject}\n\n${preview.value.body}`
  return preview.value.body
}
function stored(key: string) { return form.configured.includes(key) && !clearSecrets.value.includes(key) }
function placeholder(key: string) { return stored(key) ? t('notification.keepSecret') : '' }
function clear(key: string) {
  clearSecrets.value = clearSecrets.value.includes(key) ? clearSecrets.value.filter(value => value !== key) : [...clearSecrets.value, key]
}
async function insert(name: string) {
  const token = `{{${name}}}`
  const id = targetField.value === 'subject' ? 'channel-subject' : 'channel-body'
  const element = document.getElementById(id) as HTMLInputElement | HTMLTextAreaElement | null
  const text = targetField.value === 'subject' ? activeTemplate.value.subject : activeTemplate.value.body
  const start = element?.selectionStart ?? text.length
  const end = element?.selectionEnd ?? start
  const value = text.slice(0, start) + token + text.slice(end)
  if (targetField.value === 'subject') activeTemplate.value.subject = value
  else activeTemplate.value.body = value
  await nextTick()
  element?.focus()
  element?.setSelectionRange(start + token.length, start + token.length)
}
async function renderPreview() {
  if (!await validateTemplate()) return
  await run(async () => { preview.value = await send('/admin/notifications/preview', 'POST', { type: form.type, subject: activeTemplate.value.subject, body: activeTemplate.value.body, alertType: previewType.value }) })
}
async function validateTemplate() {
  if (!subjectIssue.value && !bodyIssue.value) return true
  tab.value = 'template'
  await nextTick()
  document.getElementById(subjectIssue.value ? 'channel-subject' : 'channel-body')?.focus()
  return false
}
async function validateForm(allTemplates = true) {
  // 保存时同时检查未展示的模板，错误定位到对应类别；测试仅检查当前模板。
  if (allTemplates) {
    for (const kind of ['service', 'certificate'] as const) {
      const template = kind === 'service' ? form : form.certificateTemplate!
      const subjectInvalid = form.type !== 'webhook' && (!template.subject.trim() || template.subject.length > 1000 || templateIssue(template.subject, props.templates.variables))
      if (subjectInvalid || !template.body.trim() || template.body.length > 12000 || templateIssue(template.body, props.templates.variables, form.type === 'webhook')) {
        templateKind.value = kind
        tab.value = 'template'
        await nextTick()
        const input = document.getElementById(subjectInvalid ? 'channel-subject' : 'channel-body') as HTMLInputElement | HTMLTextAreaElement | null
        input?.focus()
        input?.reportValidity()
        return false
      }
    }
  }
  if (!editorForm.value) return false
  const invalid = editorForm.value.querySelector<HTMLInputElement>('input:invalid, textarea:invalid')
  if (!invalid) return validateTemplate()
  tab.value = invalid.closest('[data-template]') ? 'template' : 'connection'
  await nextTick()
  invalid.focus()
  invalid.reportValidity()
  return false
}
function channelPayload() {
  let parsed: Record<string, string> = {}
  if (form.type === 'webhook' && headers.value.trim()) {
    try { parsed = JSON.parse(headers.value) }
    catch { throw new Error(t('notification.headersInvalid')) }
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object' || Object.values(parsed).some(value => typeof value !== 'string')) throw new Error(t('notification.headersInvalid'))
  }
  return { ...form, clearSecrets: clearSecrets.value, config: { ...form.config, port: Number(form.config.port), threadId: Number(form.config.threadId), to: recipients.value.split(/[,;\n，；]/).map(value => value.trim()).filter(Boolean), headers: parsed } }
}
async function testCurrent() {
  if (busy.value || !await validateForm(false)) return
  await run(async () => {
    testing.value = true
    testResult.value = undefined
    try {
      // 草稿测试只发送当前模板，不受另一套未完成的草稿影响，也不写回配置。
      await send('/admin/notification-channels/test', 'POST', { ...channelPayload(), subject: activeTemplate.value.subject, body: activeTemplate.value.body, certificateTemplate: { subject: activeTemplate.value.subject, body: activeTemplate.value.body }, alertType: previewType.value })
      testResult.value = { success: true, message: t('notification.testSent') }
    } catch (cause) {
      testResult.value = { success: false, message: cause instanceof Error ? cause.message : t('common.requestError') }
    } finally { testing.value = false }
  })
}
async function submit() {
  if (busy.value || !await validateForm()) return
  await run(async () => {
    const payload = channelPayload()
    await send('/admin/notification-channels' + (form.id ? `/${form.id}` : ''), form.id ? 'PUT' : 'POST', payload)
    notify(t('common.saved')); open.value = false; emit('saved')
  })
}
</script>
<template>
  <Modal v-model:open="open" :title="channel ? t('notification.edit') : t('notification.add')" :description="t('notification.editorHint')" :busy="busy" wide>
    <form ref="editorForm" class="editor-form monitor-editor-form notification-editor-form" novalidate @submit.prevent="submit">
      <TabsRoot v-model="tab" class="monitor-editor-tabs">
        <TabsList class="editor-tabs" :aria-label="t('notification.editorTabs')">
          <TabsTrigger value="connection" class="editor-tab" :disabled="busy">{{ t('notification.connection') }}</TabsTrigger>
          <TabsTrigger value="template" class="editor-tab" :disabled="busy">{{ t('notification.template') }}</TabsTrigger>
        </TabsList>
      <div v-if="testResult" class="notification-test-result" :data-success="testResult.success" :role="testResult.success ? 'status' : 'alert'">{{ testResult.message }}</div>
      <div class="editor-tab-scroll">
      <TabsContent value="connection" force-mount v-show="tab === 'connection'" as-child>
      <fieldset :disabled="busy" class="notification-connection-fields">
        <div class="form-grid">
          <Field id="channel-name" :label="t('common.name')" required><Input id="channel-name" v-model="form.name" required maxlength="200" /></Field>
          <Field id="channel-type" :label="t('notification.channelType')"><Select id="channel-type" v-model="form.type" :options="types" :disabled="!!form.id || busy" /></Field>
        </div>
        <div class="notification-toggle"><label for="channel-enabled">{{ t('common.enabled') }}</label><Switch id="channel-enabled" v-model="form.enabled" /></div>
        <template v-if="form.type === 'email'">
          <div class="form-grid">
            <Field id="smtp-host" :label="t('notification.smtpHost')" required><Input id="smtp-host" v-model="form.config.host" required /></Field>
            <Field id="smtp-port" :label="t('notification.port')" required><Input id="smtp-port" v-model="form.config.port" type="number" min="1" max="65535" required /></Field>
            <Field id="smtp-security" :label="t('notification.security')"><Select id="smtp-security" v-model="form.config.security" :options="[{ value: 'starttls', label: 'STARTTLS' }, { value: 'tls', label: 'TLS' }, { value: 'none', label: t('notification.noTLS') }]" /></Field>
            <Field id="smtp-username" :label="t('notification.username')"><Input id="smtp-username" v-model="form.config.username" autocomplete="off" /></Field>
            <Field id="smtp-password" :label="t('notification.password')"><Input id="smtp-password" v-model="form.config.password" type="password" autocomplete="new-password" :placeholder="placeholder('password')" /></Field>
            <Field id="smtp-from" :label="t('notification.from')" required><Input id="smtp-from" v-model="form.config.from" required /></Field>
          </div>
          <Field id="smtp-to" :label="t('notification.to')" :hint="t('notification.toHint')" required><Textarea id="smtp-to" v-model="recipients" rows="3" required /></Field>
        </template>
        <template v-else-if="form.type === 'telegram'">
          <Field id="bot-token" label="Bot Token" required><Input id="bot-token" v-model="form.config.token" type="password" autocomplete="new-password" :required="!stored('token')" :placeholder="placeholder('token')" /></Field>
          <div class="form-grid">
            <Field id="chat-id" label="Chat ID" required><Input id="chat-id" v-model="form.config.chatId" required /></Field>
            <Field id="thread-id" :label="t('notification.threadId')"><Input id="thread-id" v-model="form.config.threadId" type="number" min="0" /></Field>
          </div>
        </template>
        <template v-else>
          <Field id="webhook-url" label="Webhook URL" required><Input id="webhook-url" v-model="form.config.url" type="url" :required="!stored('url')" :placeholder="placeholder('url')" autocomplete="off" /></Field>
          <Field v-if="form.type !== 'webhook'" id="webhook-secret" :label="t('notification.signingSecret')" :hint="t('notification.optional')"><Input id="webhook-secret" v-model="form.config.secret" type="password" autocomplete="new-password" :placeholder="placeholder('secret')" /></Field>
          <Field v-else id="webhook-headers" :label="t('notification.headers')" :hint="t('notification.headersHint')"><Textarea id="webhook-headers" v-model="headers" rows="3" :placeholder="placeholder('headers') || headersPlaceholder" /></Field>
        </template>
        <Field v-if="form.type !== 'email'" id="notification-proxy" :label="t('notification.proxy')" :hint="t('notification.proxyHint')"><Input id="notification-proxy" v-model="form.config.proxyURL" type="url" autocomplete="off" :placeholder="placeholder('proxyURL')" /></Field>
        <div v-if="form.configured.length" class="notification-secret-controls">
          <p class="field-hint">{{ t('notification.secretHint') }}</p>
          <Button v-for="key in form.configured.filter(key => ['password', 'secret', 'headers', 'proxyURL'].includes(key))" :key="key" type="button" size="sm" variant="outline" :aria-pressed="clearSecrets.includes(key)" @click="clear(key)">{{ t(clearSecrets.includes(key) ? 'notification.undoClear' : 'notification.clearSecret', { field: key }) }}</Button>
        </div>
        <p class="field-hint">{{ t('notification.testDraftHint') }}</p>
      </fieldset>
      </TabsContent>
      <TabsContent value="template" force-mount v-show="tab === 'template'" as-child>
      <fieldset data-template :disabled="busy" class="notification-template-fields">
        <div class="notification-template-heading">
          <strong class="text-sm">{{ t(`notification.types.${form.type}`) }} · {{ t('notification.template') }}</strong>
          <Button type="button" size="sm" variant="outline" @click="resetTemplate">{{ t('notification.resetTemplate') }}</Button>
        </div>
        <p class="field-hint">{{ t('notification.templateHint') }}</p>
        <Field id="template-kind" :label="t('notification.templateKind')"><Select id="template-kind" v-model="templateKind" :options="(['service', 'certificate'] as const).map(value => ({ value, label: t(`notification.${value}Template`) }))" /></Field>
        <Field v-if="form.type !== 'webhook'" id="channel-subject" :label="t(`notification.templateFields.${form.type}.subject`)" required>
          <Input id="channel-subject" v-model="activeTemplate.subject" maxlength="1000" required :aria-invalid="!!subjectIssue" :aria-describedby="subjectIssue ? 'channel-subject-error' : undefined" @focus="targetField = 'subject'" />
          <p v-if="subjectIssue" id="channel-subject-error" class="notification-template-error" aria-live="polite">{{ issueText(subjectIssue) }}</p>
        </Field>
        <Field id="channel-body" :label="t(`notification.templateFields.${form.type}.body`)" :hint="t(`notification.templateFields.${form.type}.hint`)" required>
          <Textarea id="channel-body" v-model="activeTemplate.body" rows="9" maxlength="12000" required :class="{ 'font-mono': ['json', 'markdown', 'card'].includes(currentTemplate.format) }" :aria-invalid="!!bodyIssue" :aria-describedby="bodyIssue ? 'channel-body-error' : undefined" @focus="targetField = 'body'" />
          <p v-if="bodyIssue" id="channel-body-error" class="notification-template-error" aria-live="polite">{{ issueText(bodyIssue) }}</p>
        </Field>
        <div class="notification-variable-help">
          <strong class="field-label">{{ t('notification.insertVariable') }}</strong>
          <p class="field-hint">{{ t('notification.variableHint') }}</p>
          <div class="notification-variables" :aria-label="t('notification.insertVariable')"><Button v-for="name in templates.variables" :key="name" type="button" variant="outline" size="sm" :title="t(`notification.variables.${name}`)" :aria-label="t('notification.insertNamedVariable', { name, description: t(`notification.variables.${name}`) })" @mouseenter="activeVariable = name" @focus="activeVariable = name" @click="insert(name)">{{ name }}</Button></div>
          <p class="notification-variable-description" aria-live="polite"><code>{{ activeVariable }}</code> · {{ t(`notification.variables.${activeVariable}`) }}</p>
        </div>
        <div class="notification-preview-toolbar">
          <Field id="preview-event" :label="t('notification.previewType')"><Select id="preview-event" v-model="previewType" :options="eventTypes.map(value => ({ value, label: t(`notification.events.${value}`) }))" /></Field>
          <Button type="button" variant="outline" :disabled="busy" @click="renderPreview">{{ t('notification.preview') }}</Button>
        </div>
        <div v-if="preview" class="notification-preview" :class="`notification-preview-${form.type}`" aria-live="polite">
          <p class="field-hint">{{ t(`notification.templateFields.${form.type}.preview`) }}</p>
          <strong v-if="form.type !== 'webhook' && form.type !== 'dingtalk'">{{ preview.subject }}</strong>
          <pre>{{ previewBody() }}</pre>
        </div>
        <details class="notification-syntax">
          <summary>{{ t('notification.syntaxTitle') }}<span>{{ t(`notification.syntax.${form.type}.format`) }}</span><ChevronDown aria-hidden="true" /></summary>
          <div class="notification-syntax-content">
            <p>{{ t('notification.syntaxVariables', { example: variableExample }) }}</p>
            <p>{{ t(`notification.syntax.${form.type}.rules`) }}</p>
          </div>
        </details>
      </fieldset>
      </TabsContent>
      </div>
      </TabsRoot>
      <div class="form-actions notification-editor-actions">
        <Button type="button" variant="outline" :disabled="busy" @click="testCurrent"><Send />{{ t(testing ? 'notification.testing' : 'notification.test') }}</Button>
        <Button type="button" variant="outline" :disabled="busy" @click="open = false">{{ t('common.cancel') }}</Button>
        <Button type="submit" :disabled="busy">{{ busy && !testing ? t('common.saving') : t('common.save') }}</Button>
      </div>
    </form>
  </Modal>
</template>
