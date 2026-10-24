<script setup lang="ts">
import { Activity } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>
<template>
  <span class="brand">
    <span class="brand-mark"><Activity :size="20" :stroke-width="2.5" /></span>
    <span>{{ t('common.brand') }}</span>
  </span>
</template>
