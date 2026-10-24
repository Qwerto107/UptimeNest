<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import EmptyState from '@/components/EmptyState.vue'
import { Button } from '@/components/ui/button'
const { t } = useI18n()
</script>
<template>
  <main class="public-main">
    <EmptyState :title="t('common.notFound')">
      <Button as-child>
        <RouterLink to="/">{{ t('common.home') }}</RouterLink>
      </Button>
    </EmptyState>
  </main>
</template>
