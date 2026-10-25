<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Sun, Moon, Trash2 } from 'lucide-vue-next'
import type { Settings } from '@/types'
import { request, send } from '@/lib/api'
import { dark, setTheme, loadBootstrap } from '@/composables/useApp'
import { useResource } from '@/composables/useResource'
import { useAction, notify } from '@/composables/useFeedback'
import Field from '@/components/Field.vue'
import LoadState from '@/components/LoadState.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select } from '@/components/ui/select'
const { t } = useI18n()
const { busy, run } = useAction()
const cleanupOpen = ref(false)
const savedDays = ref(30)
const trustedProxies = ref('')
const form = reactive<Settings>({
  siteName: t('common.brand'),
  timezone: 'Asia/Singapore',
  language: 'en-US',
  entryPage: 'status',
  userAgent: '',
  retentionDays: 30,
  trustedProxies: [],
})
const commonTimezones = [
  { value: 'Asia/Singapore', name: 'singapore' },
  { value: 'Asia/Shanghai', name: 'shanghai' },
  { value: 'Asia/Hong_Kong', name: 'hongKong' },
  { value: 'Asia/Tokyo', name: 'tokyo' },
  { value: 'UTC', name: 'utc' },
  { value: 'Europe/London', name: 'london' },
  { value: 'America/New_York', name: 'newYork' },
  { value: 'America/Los_Angeles', name: 'losAngeles' },
]
const supportedTimezones =
  typeof Intl.supportedValuesOf === 'function' ? Intl.supportedValuesOf('timeZone') : []
const otherTimezones = computed(() => {
  const common = new Set(commonTimezones.map((zone) => zone.value))
  // 兼容旧配置及浏览器目录之外的合法别名，避免下拉列表丢失当前选择。
  return [...new Set([...supportedTimezones, form.timezone])]
    .filter((zone) => zone && !common.has(zone))
    .sort()
})
const timezoneGroups = computed(() => {
  const groups = [{
    label: t('settings.commonTimezones'),
    options: commonTimezones.map((zone) => ({
      value: zone.value,
      label: `${t(`settings.timezoneNames.${zone.name}`)} · ${zone.value}`,
    })),
  }]
  if (otherTimezones.value.length) {
    groups.push({
      label: t('settings.otherTimezones'),
      options: otherTimezones.value.map((zone) => ({ value: zone, label: zone })),
    })
  }
  return groups
})
const { data, loading, error, refresh } = useResource((signal) =>
  request<Settings & { clientIP: string }>('/admin/settings', { signal }),
)
watch(data, (value) => {
  if (value) {
    // 客户端 IP 只用于展示，不能混入设置保存请求。
    const { clientIP, ...settings } = value
    Object.assign(form, settings)
    savedDays.value = value.retentionDays
    trustedProxies.value = (value.trustedProxies || []).join('\n')
  }
})
async function save() {
  await run(async () => {
    await send('/admin/settings', 'PUT', {
      ...form,
      retentionDays: Number(form.retentionDays),
      trustedProxies: trustedProxies.value.split(/\r?\n/).map((value) => value.trim()).filter(Boolean),
    })
    savedDays.value = Number(form.retentionDays)
    await refresh()
    await loadBootstrap()
    notify(t('common.saved'))
  })
}
async function cleanup() {
  await run(async () => {
    const counts = await send<{ heartbeats: number; logs: number }>('/admin/cleanup', 'POST')
    cleanupOpen.value = false
    notify(t('settings.cleaned', counts))
  })
}
</script>
<template>
  <div class="page-heading">
    <div>
      <h1>{{ t('settings.title') }}</h1>
      <p>{{ t('settings.subtitle') }}</p>
    </div>
  </div>
  <LoadState :loading="loading" :error="error" :has-data="!!data" @retry="refresh" />
  <form v-if="data" class="settings-form" @submit.prevent="save">
    <section class="settings-section">
      <div class="settings-section-title">
        <span class="settings-code">{{ t('settings.displayCode') }}</span>
        <h2>{{ t('settings.display') }}</h2>
      </div>
      <fieldset :disabled="busy" class="settings-fields">
        <Field id="site-name" :label="t('settings.siteName')" :hint="t('settings.siteNameHint')">
          <Input
            id="site-name"
            v-model="form.siteName"
            maxlength="100"
            :placeholder="t('common.brand')"
          />
        </Field>
        <Field id="timezone" :label="t('settings.timezone')" :hint="t('settings.timezoneHint')">
          <Select id="timezone" v-model="form.timezone" :groups="timezoneGroups" :disabled="busy" required />
        </Field>
        <div class="settings-row">
          <Field id="entry-page" :label="t('settings.entry')">
            <Select
              id="entry-page"
              v-model="form.entryPage"
              :disabled="busy"
              :options="([{ value: 'status', label: t('settings.statusEntry') }, { value: 'admin', label: t('settings.adminEntry') }] as const)"
            />
          </Field>
          <Field id="language" :label="t('settings.language')" :hint="t('settings.languageHint')">
            <Select
              id="language"
              v-model="form.language"
              :disabled="busy"
              :options="[{ value: 'en-US', label: t('settings.enUS') }, { value: 'zh-CN', label: t('settings.zhCN') }]"
            />
          </Field>
        </div>
        <div class="field">
          <span class="field-label">{{ t('settings.theme') }}</span>
          <div class="theme-options" role="group" :aria-label="t('settings.theme')">
            <Button
              type="button"
              :variant="dark ? 'outline' : 'default'"
              :aria-pressed="!dark"
              @click="setTheme(false)"
            >
              <Sun />
              {{ t('settings.light') }}
            </Button>
            <Button
              type="button"
              :variant="dark ? 'default' : 'outline'"
              :aria-pressed="dark"
              @click="setTheme(true)"
            >
              <Moon />
              {{ t('settings.dark') }}
            </Button>
          </div>
          <p class="field-hint">{{ t('settings.themeHint') }}</p>
        </div>
      </fieldset>
    </section>
    <section class="settings-section">
      <div class="settings-section-title">
        <span class="settings-code">{{ t('settings.monitoringCode') }}</span>
        <h2>{{ t('settings.monitoring') }}</h2>
      </div>
      <fieldset :disabled="busy" class="settings-fields">
        <Field id="user-agent" :label="t('settings.ua')" :hint="t('settings.uaHint')">
          <Input
            id="user-agent"
            v-model="form.userAgent"
            placeholder="UptimeNest"
            maxlength="500"
          />
        </Field>
      </fieldset>
    </section>
    <section class="settings-section">
      <div class="settings-section-title">
        <span class="settings-code">{{ t('settings.historyCode') }}</span>
        <h2>{{ t('settings.history') }}</h2>
      </div>
      <fieldset :disabled="busy" class="settings-fields">
        <Field id="retention" :label="t('settings.retention')" :hint="t('settings.retentionHint')">
          <Input
            id="retention"
            v-model.number="form.retentionDays"
            type="number"
            min="1"
            max="3650"
            required
          />
        </Field>
        <div>
          <Button type="button" variant="outline" :disabled="busy" @click="cleanupOpen = true">
            <Trash2 />
            {{ t('settings.cleanup') }}
          </Button>
          <p class="field-hint mt-2">{{ t('settings.cleanupSaveFirst') }}</p>
        </div>
      </fieldset>
    </section>
    <section class="settings-section">
      <div class="settings-section-title">
        <span class="settings-code">{{ t('settings.accessSecurityCode') }}</span>
        <h2>{{ t('settings.accessSecurity') }}</h2>
      </div>
      <fieldset :disabled="busy" class="settings-fields">
        <Field id="current-ip" :label="t('settings.currentIP')" :hint="t('settings.currentIPHint')">
          <output id="current-ip" class="font-mono text-sm break-all">{{ data.clientIP }}</output>
        </Field>
        <Field id="trusted-proxies" :label="t('settings.trustedProxies')" :hint="t('settings.trustedProxiesHint')">
          <Textarea
            id="trusted-proxies"
            v-model="trustedProxies"
            :rows="4"
            maxlength="10100"
            :placeholder="t('settings.trustedProxiesExample')"
          />
        </Field>
      </fieldset>
    </section>
    <div class="settings-save">
      <Button type="submit" :disabled="busy">
        {{ busy ? t('common.saving') : t('common.save') }}
      </Button>
    </div>
  </form>
  <ConfirmDialog
    v-model:open="cleanupOpen"
    :title="t('settings.cleanupTitle')"
    :description="t('settings.cleanupDescription', { days: savedDays })"
    :busy="busy"
    :label="t('settings.cleanup')"
    @confirm="cleanup"
  />
</template>
