<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { MonitorStatus } from '@/types'
defineProps<{ status: MonitorStatus }>()
const { t } = useI18n()
</script>
<template>
  <span class="status-badge" :class="`state-${status}`">
    <span class="status-dot" />
    {{ t(`state.${status}`) }}
  </span>
</template>
