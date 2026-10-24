<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { Primitive, type PrimitiveProps } from 'reka-ui'
import { cva } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<
    PrimitiveProps & {
      variant?: 'default' | 'outline' | 'ghost' | 'destructive'
      size?: 'default' | 'sm' | 'icon'
      class?: HTMLAttributes['class']
    }
  >(),
  { as: 'button', variant: 'default', size: 'default' },
)

const variants = cva(
  'ui-button inline-flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-lg text-[13px] font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-3.5',
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground hover:bg-primary/90',
        outline: 'border border-input bg-background hover:bg-accent hover:text-accent-foreground',
        ghost: 'hover:bg-accent hover:text-accent-foreground',
        destructive: 'bg-destructive text-white hover:bg-destructive/90',
      },
      size: {
        default: 'h-9 px-4 py-2',
        sm: 'h-8 px-3 text-xs',
        icon: 'size-8',
      },
    },
  },
)
</script>

<template>
  <Primitive
    :as="as"
    :as-child="asChild"
    :data-variant="variant"
    :class="cn(variants({ variant, size }), props.class)"
  >
    <slot />
  </Primitive>
</template>
