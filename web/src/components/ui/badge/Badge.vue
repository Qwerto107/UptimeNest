<script setup lang="ts">
import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'vue'
const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>
<template>
  <span
    :class="
      cn(
        'inline-flex items-center rounded-md bg-secondary px-2 py-0.5 text-xs text-secondary-foreground',
        props.class,
      )
    "
  >
    <slot />
  </span>
</template>
