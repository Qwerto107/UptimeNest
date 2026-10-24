<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'
const props = defineProps<{ class?: HTMLAttributes['class'] }>()
const model = defineModel<string>()
</script>
<template>
  <textarea
    v-model="model"
    :class="
      cn(
        'flex min-h-24 w-full rounded-lg border border-input bg-background px-3 py-2 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/20 disabled:opacity-50',
        props.class,
      )
    "
  />
</template>
