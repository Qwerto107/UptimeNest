<script setup lang="ts">
import { Activity } from 'lucide-vue-next'
import { siteName } from '@/composables/useApp'
</script>
<template>
  <span class="brand">
    <span class="brand-mark"><Activity :size="17" :stroke-width="1.7" /></span>
    <span>{{ siteName }}</span>
  </span>
</template>
