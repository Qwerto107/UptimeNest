<script setup lang="ts">
import type { PublicMonitor } from '@/types'
import { bootstrap } from '@/composables/useApp'
import { useMotion } from '@/composables/useMotion'
import HeartbeatStrip from '@/components/HeartbeatStrip.vue'
import LatencyHistoryChart from '@/components/LatencyHistoryChart.vue'
import ConnectionDetails from '@/components/ConnectionDetails.vue'
import StatusConnectionMeta from '@/components/StatusConnectionMeta.vue'
const props = defineProps<{ monitor: PublicMonitor; open: boolean; condensed?: boolean; grid?: boolean; column?: number }>()
const emit = defineEmits<{ capacity: [count: number] }>()
const { animate, reducedMotion } = useMotion()
function finish(animation: Animation | undefined, done: () => void) {
  if (animation) void animation.finished.then(done, done)
  else done()
}
function resetDetails(element: Element) {
  const style = (element as HTMLElement).style
  for (const property of ['position', 'top', 'left', 'width', 'margin-top', 'pointer-events']) style.removeProperty(property)
}
function enterDetails(element: Element, done: () => void) {
  resetDetails(element)
  finish(animate(element, [{ opacity: 0, transform: 'translateY(6px)' }, { opacity: 1, transform: 'translateY(0)' }], 200, 60), done)
}
function leaveDetails(element: Element, done: () => void) {
  const node = element as HTMLElement
  if (reducedMotion.value === 'reduce' || !node.parentElement) { done(); return }
  const rect = node.getBoundingClientRect()
  const parent = node.parentElement.getBoundingClientRect()
  // 离场内容脱离文档流，下方各行由状态页的 FLIP 动效移动到新位置。
  Object.assign(node.style, { position: 'absolute', top: `${rect.top - parent.top}px`, left: `${rect.left - parent.left}px`, width: `${rect.width}px`, marginTop: '0', pointerEvents: 'none' })
  finish(animate(node, [{ opacity: 1, transform: 'translateY(0)' }, { opacity: 0, transform: 'translateY(-4px)' }], 180), done)
}
</script>
<template>
  <Transition :css="false" @enter="enterDetails" @leave="leaveDetails" @leave-cancelled="resetDetails">
    <div v-if="props.open" :id="'connection-' + monitor.id" class="pulse-expanded" :class="{ 'pulse-condensed-details': condensed, 'pulse-card-details': grid }" :style="grid ? { '--detail-column': column ?? 0 } : undefined" @click.stop>
      <div class="pulse-expanded-content">
        <strong v-if="grid" class="pulse-card-detail-title">{{ monitor.name }}</strong>
        <template v-if="condensed">
          <p v-if="monitor.description" class="pulse-service-description">{{ monitor.description }}</p>
          <div v-if="monitor.tags.length" class="pulse-tags"><span v-for="tag in monitor.tags" :key="tag">{{ tag }}</span></div>
          <div class="pulse-details-meta">
            <span class="pulse-protocol pulse-mono">{{ monitor.protocol }}</span>
            <StatusConnectionMeta :beat="monitor.summary.latest" :tls-latency-warning-ms="bootstrap?.tlsLatencyWarningMs ?? 0" :cert-expiry-warning-days="monitor.certExpiryWarningDays ?? bootstrap?.certExpiryWarningDays ?? 30" />
          </div>
          <HeartbeatStrip :beats="monitor.summary.heartbeats" @capacity="emit('capacity', $event)" />
        </template>
        <LatencyHistoryChart :beats="monitor.summary.heartbeats" />
        <ConnectionDetails :beat="monitor.summary.latest" />
      </div>
    </div>
  </Transition>
</template>
