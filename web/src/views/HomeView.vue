<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { loadBootstrap } from '@/composables/useApp'
import { useResource } from '@/composables/useResource'
import Brand from '@/components/Brand.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import LoadState from '@/components/LoadState.vue'
import EmptyState from '@/components/EmptyState.vue'
import { Button } from '@/components/ui/button'
const { t } = useI18n()
const router = useRouter()
const { data, loading, error, refresh } = useResource(async () => {
  const value = await loadBootstrap()
  if (value.entryPage === 'admin') await router.replace('/admin')
  else if (value.defaultPanel) await router.replace(`/status/${value.defaultPanel}`)
  return value
})
</script>
<template>
  <div class="public-page">
    <header class="public-header">
      <div class="public-header-inner">
        <Brand />
        <ThemeToggle />
      </div>
    </header>
    <main class="public-main">
      <LoadState :loading="loading" :error="error" @retry="refresh" />
      <EmptyState
        v-if="data && !data.defaultPanel && data.entryPage !== 'admin'"
        :title="t('status.noPanelTitle')"
        :description="t('status.noPanelDescription')"
      >
        <Button as-child>
          <RouterLink to="/admin">{{ t('nav.admin') }}</RouterLink>
        </Button>
      </EmptyState>
    </main>
  </div>
</template>
