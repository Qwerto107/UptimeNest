<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { RefreshCw } from 'lucide-vue-next'
import { useMotion } from '@/composables/useMotion'

const props = defineProps<{ refreshing: boolean; size?: number }>()
const icon = ref<HTMLElement>()
const { animate } = useMotion()
function spin() { animate(icon.value, [{ transform: 'rotate(0deg)' }, { transform: 'rotate(360deg)' }], 500) }
watch(() => props.refreshing, (value) => { if (value) spin() }, { flush: 'post' })
onMounted(() => { if (props.refreshing) spin() })
</script>

<template>
  <span ref="icon" class="refresh-icon" aria-hidden="true"><RefreshCw :size="size" /></span>
</template>
