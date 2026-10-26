<script setup lang="ts">
defineProps<{ value: string | number }>()
</script>

<template>
  <span class="motion-value">
    <Transition name="metric">
      <span :key="value" class="motion-value-text">{{ value }}</span>
    </Transition>
  </span>
</template>
