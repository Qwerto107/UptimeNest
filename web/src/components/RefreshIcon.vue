<script setup lang="ts">
import { ref, watch } from 'vue'
import { RefreshCw } from 'lucide-vue-next'
import { useMotion } from '@/composables/useMotion'

const props = defineProps<{ refreshing: boolean; size?: number }>()
const icon = ref<HTMLElement>()
const { animate, reducedMotion } = useMotion()
const duration = 700
let rotation: Animation | undefined

watch([icon, () => props.refreshing, reducedMotion], ([element, refreshing, motion]) => {
  if (!element || motion === 'reduce') return
  if (refreshing) {
    // 连续刷新复用当前角度，避免上一圈尚未结束时突然归零。
    if (rotation?.playState !== 'running') {
      rotation = animate(element, [{ transform: 'rotate(0deg)' }, { transform: 'rotate(360deg)' }], duration, 0, 'linear')
    }
    rotation?.effect?.updateTiming({ iterations: Infinity })
    return
  }
  if (rotation?.playState !== 'running') return
  // 请求结束后完成当前整圈；完整旋转与默认朝向一致，无需强制复位。
  const elapsed = Number(rotation.currentTime) || 0
  rotation.effect?.updateTiming({ iterations: Math.max(1, Math.ceil(elapsed / duration)) })
}, { flush: 'post' })
</script>

<template>
  <span ref="icon" class="refresh-icon" aria-hidden="true"><RefreshCw :size="size" /></span>
</template>
