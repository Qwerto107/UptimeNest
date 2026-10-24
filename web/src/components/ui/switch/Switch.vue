<script setup lang="ts">
import { SwitchRoot, SwitchThumb } from 'reka-ui'
const model = defineModel<boolean>()
defineProps<{ id?: string; disabled?: boolean; ariaLabel?: string }>()
</script>
<template>
  <SwitchRoot
    :id="id"
    v-model="model"
    :disabled="disabled"
    :aria-label="ariaLabel"
    class="inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full bg-input p-0.5 outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:opacity-50 data-[state=checked]:bg-primary"
  >
    <SwitchThumb
      class="pointer-events-none block size-5 rounded-full bg-white shadow-sm transition-transform data-[state=checked]:translate-x-5"
    />
  </SwitchRoot>
</template>
