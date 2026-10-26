<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { TabsRoot, TabsList, TabsTrigger, TabsContent } from 'reka-ui'
import type { Monitor, Group, WebSocketOptions } from '@/types'
import { send } from '@/lib/api'
import { defaultHTTPOptions } from '@/lib/http'
import { groupChoices } from '@/lib/groups'
import HTTPOptionsEditor from '@/components/HTTPOptionsEditor.vue'
import HTTPAuthenticationEditor from '@/components/HTTPAuthenticationEditor.vue'
import WebSocketOptionsEditor from '@/components/WebSocketOptionsEditor.vue'
import { useAction, notify } from '@/composables/useFeedback'
import Modal from '@/components/Modal.vue'
import Field from '@/components/Field.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Select } from '@/components/ui/select'
const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ monitor?: Monitor | null; groups: Group[]; duplicate?: boolean }>()
const emit = defineEmits<{ saved: [] }>()
const { t } = useI18n()
const groupOptions = computed(() => groupChoices(props.groups))
const { busy, run } = useAction()
function defaultWebSocketOptions(): WebSocketOptions {
  return { mode: 'handshake', connectIP: '', origin: '', subprotocols: [], headers: '' }
}
function defaults(): Monitor {
  return {
    id: 0,
    name: '',
    type: 'http',
    target: '',
    hideAddress: false,
    port: 443,
    dnsRecord: 'A',
    dnsServer: '',
    expected: '',
    interval: 60,
    retries: 2,
    retryInterval: 10,
    timeout: 10,
    invert: false,
    description: '',
    tags: [],
    groupId: 0,
    enabled: true,
    http: defaultHTTPOptions(),
    websocket: defaultWebSocketOptions(),
  }
}
const form = reactive<Monitor>(defaults())
const tags = ref('')
const tab = ref('basics')
watch(
  () => form.type,
  (type) => {
    if (type !== 'http' && (tab.value === 'http' || tab.value === 'auth')) tab.value = 'basics'
    if (type !== 'websocket' && tab.value === 'websocket') tab.value = 'basics'
  },
)
watch(open, (value) => {
  if (!value) return
  tab.value = 'basics'
  const next = props.monitor || defaults()
  // 逐字段复制，避免把列表接口的 summary 等只读数据发送到配置接口。
  for (const key of Object.keys(defaults()) as (keyof Monitor)[]) {
    if (key === 'http' || key === 'websocket') continue
    const value = next[key]
    if (Array.isArray(value)) Object.assign(form, { [key]: [...value] })
    else Object.assign(form, { [key]: value })
  }
  const httpDefaults = defaultHTTPOptions()
  form.http = {
    ...httpDefaults,
    ...next.http,
    validStatusCodes: [...(next.http?.validStatusCodes || httpDefaults.validStatusCodes)],
    auth: { ...httpDefaults.auth, ...next.http?.auth },
  }
  form.websocket = {
    ...defaultWebSocketOptions(),
    ...next.websocket,
    subprotocols: [...(next.websocket?.subprotocols || [])],
  }
  if (props.duplicate && props.monitor) {
    // 副本使用新增接口；按后端的 200 字节上限预留名称后缀，避免截断 Unicode 字符。
    form.id = 0
    const suffix = t('monitor.copyNameSuffix')
    const encoder = new TextEncoder()
    let name = ''
    for (const character of next.name) {
      if (encoder.encode(name + character + suffix).length > 200) break
      name += character
    }
    form.name = name + suffix
  }
  tags.value = next.tags.join(', ')
})
async function submit(event: Event) {
  if (busy.value) return
  const element = event.currentTarget as HTMLFormElement
  const invalid = element.querySelector<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>(
    'input:invalid, select:invalid, textarea:invalid',
  )
  if (invalid) {
    // 隐藏页签也保留配置；校验失败时先切到对应页签，再定位具体字段。
    const panel = invalid.closest<HTMLElement>('[data-editor-panel]')
    tab.value = panel?.dataset.editorPanel || 'basics'
    await nextTick()
    invalid.focus()
    invalid.reportValidity()
    return
  }
  await run(async () => {
    const payload = {
      ...form,
      port: Number(form.port),
      groupId: Number(form.groupId),
      interval: Number(form.interval),
      retries: Number(form.retries),
      retryInterval: Number(form.retryInterval),
      timeout: Number(form.timeout),
      tags: tags.value
        .split(/[,，]/)
        .map((v) => v.trim())
        .filter(Boolean),
    }
    if (payload.http)
      payload.http = { ...payload.http, maxRedirects: Number(payload.http.maxRedirects) }
    if (form.type !== 'http') delete payload.http
    if (form.type !== 'websocket') delete payload.websocket
    const path = '/admin/monitors' + (form.id ? `/${form.id}` : '')
    await send(path, form.id ? 'PUT' : 'POST', payload)
    notify(t('common.saved'))
    open.value = false
    emit('saved')
  })
}
</script>
<template>
  <Modal
    v-model:open="open"
    :title="duplicate ? t('monitor.duplicate') : monitor ? t('monitor.edit') : t('monitor.add')"
    :description="t('monitor.privateHint')"
    wide
    :busy="busy"
  >
    <form class="editor-form monitor-editor-form" novalidate @submit.prevent="submit">
      <TabsRoot v-model="tab" class="monitor-editor-tabs">
        <TabsList class="editor-tabs" :aria-label="t('monitor.editorTabs')">
          <TabsTrigger value="basics" class="editor-tab">{{ t('monitor.basics') }}</TabsTrigger>
          <TabsTrigger v-if="form.type === 'http'" value="http" class="editor-tab">
            {{ t('http.options') }}
          </TabsTrigger>
          <TabsTrigger v-if="form.type === 'websocket'" value="websocket" class="editor-tab">{{ t('websocket.options') }}</TabsTrigger>
          <TabsTrigger value="probe" class="editor-tab">{{ t('monitor.advanced') }}</TabsTrigger>
          <TabsTrigger v-if="form.type === 'http'" value="auth" class="editor-tab">
            {{ t('http.authentication') }}
          </TabsTrigger>
        </TabsList>
        <div class="editor-tab-scroll">
          <TabsContent
            value="basics"
            force-mount
            v-show="tab === 'basics'"
            data-editor-panel="basics"
          >
            <fieldset :disabled="busy" class="form-section">
              <legend>{{ t('monitor.basics') }}</legend>
              <div class="form-grid">
                <Field id="monitor-name" :label="t('common.name')" required>
                  <Input
                    id="monitor-name"
                    v-model="form.name"
                    required
                    maxlength="200"
                    :placeholder="t('monitor.nameExample')"
                  />
                </Field>
                <Field id="monitor-type" :label="t('monitor.type')">
                  <Select
                    id="monitor-type"
                    v-model="form.type"
                    :disabled="busy"
                    :options="(['http', 'websocket', 'tcp', 'ping', 'dns'] as const).map((kind) => ({ value: kind, label: t(`monitor.${kind}`) }))"
                  />
                </Field>
                <Field
                  id="monitor-target"
                  :label="form.type === 'http' || form.type === 'websocket' ? t('monitor.target') : t('monitor.host')"
                  :hint="form.type === 'ping' ? t('monitor.pingHint') : undefined"
                  required
                  class="col-span-full"
                >
                  <Input
                    id="monitor-target"
                    v-model="form.target"
                    required
                    :type="form.type === 'http' || form.type === 'websocket' ? 'url' : 'text'"
                    :placeholder="
                      form.type === 'websocket' ? t('websocket.targetExample') : form.type === 'http' ? t('monitor.targetExample') : t('monitor.hostExample')
                    "
                  />
                </Field>
                <div class="toggle-row col-span-full">
                  <div>
                    <label for="monitor-hide-address">{{ t('monitor.hideAddress') }}</label>
                    <p>{{ t('monitor.hideAddressHint') }}</p>
                  </div>
                  <Switch id="monitor-hide-address" v-model="form.hideAddress" />
                </div>
                <Field
                  v-if="form.type === 'tcp'"
                  id="monitor-port"
                  :label="t('monitor.port')"
                  required
                >
                  <Input
                    id="monitor-port"
                    v-model.number="form.port"
                    type="number"
                    min="1"
                    max="65535"
                    required
                  />
                </Field>
                <template v-if="form.type === 'dns'">
                  <Field id="dns-record" :label="t('monitor.dnsRecord')">
                    <Select
                      id="dns-record"
                      v-model="form.dnsRecord"
                      :disabled="busy"
                      :options="['A', 'AAAA', 'CNAME', 'MX', 'NS', 'TXT'].map((kind) => ({ value: kind, label: kind }))"
                    />
                  </Field>
                  <Field
                    id="dns-server"
                    :label="t('monitor.dnsServer')"
                    :hint="t('monitor.dnsServerHint')"
                  >
                    <Input
                      id="dns-server"
                      v-model="form.dnsServer"
                      :placeholder="t('monitor.dnsExample')"
                    />
                  </Field>
                  <Field
                    id="dns-expected"
                    :label="t('monitor.expected')"
                    :hint="t('monitor.expectedHint')"
                    class="col-span-full"
                  >
                    <Input id="dns-expected" v-model="form.expected" />
                  </Field>
                </template>
                <Field id="monitor-group" :label="t('monitor.group')">
                  <Select
                    id="monitor-group"
                    v-model="form.groupId"
                    :disabled="busy"
                    :options="[{ value: 0, label: t('common.ungrouped') }, ...groupOptions.map((group) => ({ value: group.id, label: group.label }))]"
                  />
                </Field>
                <Field id="monitor-tags" :label="t('monitor.tags')" :hint="t('monitor.tagsHint')">
                  <Input id="monitor-tags" v-model="tags" />
                </Field>
                <Field
                  id="monitor-description"
                  :label="t('common.description')"
                  class="col-span-full"
                >
                  <Textarea id="monitor-description" v-model="form.description" maxlength="4000" />
                </Field>
              </div>
            </fieldset>
          </TabsContent>
          <TabsContent
            v-if="form.type === 'http' && form.http"
            value="http"
            force-mount
            v-show="tab === 'http'"
            data-editor-panel="http"
          >
            <HTTPOptionsEditor v-model="form.http" :busy="busy" />
          </TabsContent>
          <TabsContent value="probe" force-mount v-show="tab === 'probe'" data-editor-panel="probe">
            <fieldset :disabled="busy" class="form-section">
              <legend>{{ t('monitor.advanced') }}</legend>
              <div class="form-grid">
                <Field
                  id="monitor-interval"
                  :label="`${t('monitor.interval')} (${t('common.seconds')})`"
                  :hint="t('monitor.intervalHint')"
                >
                  <Input
                    id="monitor-interval"
                    v-model.number="form.interval"
                    type="number"
                    min="5"
                    max="86400"
                    required
                  />
                </Field>
                <Field
                  id="monitor-timeout"
                  :label="`${t('monitor.timeout')} (${t('common.seconds')})`"
                >
                  <Input
                    id="monitor-timeout"
                    v-model.number="form.timeout"
                    type="number"
                    min="1"
                    max="120"
                    required
                  />
                </Field>
                <Field
                  id="monitor-retries"
                  :label="t('monitor.retries')"
                  :hint="t('monitor.retriesHint')"
                >
                  <Input
                    id="monitor-retries"
                    v-model.number="form.retries"
                    type="number"
                    min="0"
                    max="10"
                    required
                  />
                </Field>
                <Field
                  id="monitor-retry-interval"
                  :label="`${t('monitor.retryInterval')} (${t('common.seconds')})`"
                >
                  <Input
                    id="monitor-retry-interval"
                    v-model.number="form.retryInterval"
                    type="number"
                    min="1"
                    max="3600"
                    required
                  />
                </Field>
              </div>
              <div class="toggle-row">
                <div>
                  <label for="monitor-invert">{{ t('monitor.invert') }}</label>
                  <p>{{ t('monitor.invertHint') }}</p>
                </div>
                <Switch id="monitor-invert" v-model="form.invert" />
              </div>
              <div class="toggle-row">
                <div>
                  <label for="monitor-enabled">{{ t('monitor.start') }}</label>
                  <p>{{ t('monitor.activeHint') }}</p>
                </div>
                <Switch id="monitor-enabled" v-model="form.enabled" />
              </div>
            </fieldset>
          </TabsContent>
          <TabsContent v-if="form.type === 'websocket' && form.websocket" value="websocket" force-mount v-show="tab === 'websocket'" data-editor-panel="websocket">
            <WebSocketOptionsEditor v-model="form.websocket" :busy="busy" />
          </TabsContent>
          <TabsContent
            v-if="form.type === 'http' && form.http"
            value="auth"
            force-mount
            v-show="tab === 'auth'"
            data-editor-panel="auth"
          >
            <HTTPAuthenticationEditor v-model="form.http" :busy="busy" />
          </TabsContent>
        </div>
      </TabsRoot>
      <div class="form-actions">
        <Button type="button" variant="outline" :disabled="busy" @click="open = false">
          {{ t('common.cancel') }}
        </Button>
        <Button type="submit" :disabled="busy">
          {{ busy ? t('common.saving') : t('common.save') }}
        </Button>
      </div>
    </form>
  </Modal>
</template>
