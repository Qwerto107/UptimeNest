<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Monitor, Group } from '@/types'
import { send } from '@/lib/api'
import { defaultHTTPOptions } from '@/lib/http'
import { groupChoices } from '@/lib/groups'
import HTTPOptionsEditor from '@/components/HTTPOptionsEditor.vue'
import { useAction, notify } from '@/composables/useFeedback'
import Modal from '@/components/Modal.vue'
import Field from '@/components/Field.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ monitor?: Monitor | null; groups: Group[] }>()
const emit = defineEmits<{ saved: [] }>()
const { t } = useI18n()
const groupOptions = computed(() => groupChoices(props.groups))
const { busy, run } = useAction()
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
  }
}
const form = reactive<Monitor>(defaults())
const tags = ref('')
watch(open, (value) => {
  if (!value) return
  const next = props.monitor || defaults()
  // 逐字段复制，避免把列表接口的 summary 等只读数据发送到配置接口。
  for (const key of Object.keys(defaults()) as (keyof Monitor)[]) {
    if (key === 'http') continue
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
  tags.value = next.tags.join(', ')
})
async function submit() {
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
    :title="monitor ? t('monitor.edit') : t('monitor.add')"
    :description="t('monitor.privateHint')"
    wide
    :busy="busy"
  >
    <form class="editor-form" @submit.prevent="submit">
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
            <select id="monitor-type" v-model="form.type" class="control">
              <option v-for="kind in ['http', 'tcp', 'ping', 'dns']" :key="kind" :value="kind">
                {{ t(`monitor.${kind}`) }}
              </option>
            </select>
          </Field>
          <Field
            id="monitor-target"
            :label="form.type === 'http' ? t('monitor.target') : t('monitor.host')"
            :hint="form.type === 'ping' ? t('monitor.pingHint') : undefined"
            required
            class="col-span-full"
          >
            <Input
              id="monitor-target"
              v-model="form.target"
              required
              :type="form.type === 'http' ? 'url' : 'text'"
              :placeholder="
                form.type === 'http' ? t('monitor.targetExample') : t('monitor.hostExample')
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
          <Field v-if="form.type === 'tcp'" id="monitor-port" :label="t('monitor.port')" required>
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
              <select id="dns-record" v-model="form.dnsRecord" class="control">
                <option v-for="kind in ['A', 'AAAA', 'CNAME', 'MX', 'NS', 'TXT']" :key="kind">
                  {{ kind }}
                </option>
              </select>
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
            <select id="monitor-group" v-model.number="form.groupId" class="control">
              <option :value="0">{{ t('common.ungrouped') }}</option>
              <option v-for="group in groupOptions" :key="group.id" :value="group.id">
                {{ group.label }}
              </option>
            </select>
          </Field>
          <Field id="monitor-tags" :label="t('monitor.tags')" :hint="t('monitor.tagsHint')">
            <Input id="monitor-tags" v-model="tags" />
          </Field>
          <Field id="monitor-description" :label="t('common.description')" class="col-span-full">
            <Textarea id="monitor-description" v-model="form.description" maxlength="4000" />
          </Field>
        </div>
      </fieldset>
      <HTTPOptionsEditor
        v-if="form.type === 'http' && form.http"
        v-model="form.http"
        :busy="busy"
      />
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
          <Field id="monitor-timeout" :label="`${t('monitor.timeout')} (${t('common.seconds')})`">
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
