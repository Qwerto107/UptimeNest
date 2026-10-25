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
const { t } = useI18n()
const { busy, run } = useAction()
const cleanupOpen = ref(false)
const savedDays = ref(30)
const trustedProxies = ref('')
const form = reactive<Settings>({
  siteName: t('common.brand'),
  timezone: 'Asia/Singapore',
  language: 'zh-CN',
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
const { data, loading, error, refresh } = useResource((signal) =>
  request<Settings>('/admin/settings', { signal }),
)
watch(data, (value) => {
  if (value) {
    Object.assign(form, value)
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
    notify(t('common.saved'))
    await loadBootstrap()
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
          <select id="timezone" v-model="form.timezone" class="control" required>
            <optgroup :label="t('settings.commonTimezones')">
              <option v-for="zone in commonTimezones" :key="zone.value" :value="zone.value">
                {{ t(`settings.timezoneNames.${zone.name}`) }} · {{ zone.value }}
              </option>
            </optgroup>
            <optgroup v-if="otherTimezones.length" :label="t('settings.otherTimezones')">
              <option v-for="zone in otherTimezones" :key="zone" :value="zone">{{ zone }}</option>
            </optgroup>
          </select>
        </Field>
        <div class="settings-row">
          <Field id="entry-page" :label="t('settings.entry')">
            <select id="entry-page" v-model="form.entryPage" class="control">
              <option value="status">{{ t('settings.statusEntry') }}</option>
              <option value="admin">{{ t('settings.adminEntry') }}</option>
            </select>
          </Field>
          <Field id="language" :label="t('settings.language')" :hint="t('settings.languageHint')">
            <select id="language" v-model="form.language" class="control">
              <option value="zh-CN">{{ t('settings.zhCN') }}</option>
            </select>
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
        <span class="settings-code">{{ t('settings.securityCode') }}</span>
        <h2>{{ t('settings.accessSecurity') }}</h2>
      </div>
      <fieldset :disabled="busy" class="settings-fields">
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
