<script setup lang="ts">
import { Activity } from 'lucide-vue-next'
defineProps<{ title: string; description?: string }>()
</script>
<template>
  <div class="empty-state">
    <div class="empty-icon">
      <slot name="icon"><Activity /></slot>
    </div>
    <h2>{{ title }}</h2>
    <p v-if="description">{{ description }}</p>
    <div v-if="$slots.default" class="mt-6"><slot /></div>
  </div>
</template>
