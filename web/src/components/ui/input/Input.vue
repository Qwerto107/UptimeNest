<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'
const props = defineProps<{ class?: HTMLAttributes['class']; type?: string }>()
const [model, modifiers] = defineModel<string | number>({
  set(value) {
    if (modifiers.number && value !== '') return Number(value)
    return value
  },
})
</script>

<template>
  <input
    v-model="model"
    :type="type || 'text'"
    :class="
      cn(
        'flex h-10 w-full min-w-0 rounded-lg border border-input bg-background px-3 py-2 text-sm outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/20 disabled:cursor-not-allowed disabled:opacity-50',
        props.class,
      )
    "
  />
</template>
