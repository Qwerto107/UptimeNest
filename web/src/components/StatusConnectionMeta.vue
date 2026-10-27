<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { LockKeyhole, BadgeCheck, ShieldCheck, Zap, Timer } from 'lucide-vue-next'
import type { Heartbeat } from '@/types'
import { certificate, formatTime, latency } from '@/lib/format'
import AnimatedValue from '@/components/AnimatedValue.vue'

const props = withDefaults(defineProps<{ beat: Heartbeat | null; tlsLatencyWarningMs?: number; certExpiryWarningDays?: number }>(), { tlsLatencyWarningMs: 0, certExpiryWarningDays: 30 })
const { t } = useI18n()
const hasMeta = computed(
  () =>
    props.beat?.httpVersion ||
    props.beat?.tlsVersion ||
    props.beat?.certExpiry ||
    props.beat?.certIssuerOrg,
)
const certificateState = computed(() => {
  if (!props.beat?.certExpiry) return 'normal'
  const remaining = props.beat.certExpiry - Date.now()
  if (remaining <= 0) return 'expired'
  return props.certExpiryWarningDays > 0 && remaining <= props.certExpiryWarningDays * 86400000 ? 'soon' : 'normal'
})
const latencyWarning = computed(() => props.tlsLatencyWarningMs > 0 && Number.isFinite(props.beat?.tlsLatency) && (props.beat?.tlsLatency ?? 0) > props.tlsLatencyWarningMs)
const handshakeTitle = computed(() => latencyWarning.value
  ? t('status.handshakeWarning', { value: latency(props.beat?.tlsLatency), threshold: latency(props.tlsLatencyWarningMs) })
  : t('status.handshake'))
const httpVersionClass = computed(() => {
  const major = props.beat?.httpVersion?.split('.')[0]
  if (major === 'HTTP/2') return 'pulse-http2'
  if (major === 'HTTP/3') return 'pulse-http3'
  return undefined
})
</script>

<template>
  <div v-if="beat && hasMeta" class="pulse-connection-meta pulse-mono">
    <span
      v-if="beat.certExpiry"
      class="pulse-meta-pill pulse-meta-certificate"
      :class="{ 'pulse-certificate-soon': certificateState === 'soon', 'pulse-certificate-expired': certificateState === 'expired' }"
      :title="t('status.certificate') + ' · ' + formatTime(beat.certExpiry)"
    >
      <LockKeyhole />
      <AnimatedValue :value="certificate(beat.certExpiry)" />
    </span>
    <span v-if="beat.certIssuerOrg" class="pulse-meta-pill pulse-meta-ca" :title="t('status.issuer')">
      <BadgeCheck />
      {{ beat.certIssuerOrg }}
    </span>
    <span v-if="beat.tlsVersion" class="pulse-meta-pill pulse-meta-tls" :class="{ 'pulse-tls-warning': !['TLS 1.2', 'TLS 1.3'].includes(beat.tlsVersion) }" :title="t('status.tlsVersion')">
      <ShieldCheck />
      {{ beat.tlsVersion }}
    </span>
    <span v-if="beat.httpVersion" class="pulse-meta-pill pulse-meta-http" :class="httpVersionClass" :title="t('status.httpVersion')">
      <Zap />
      {{ beat.httpVersion }}
    </span>
    <span
      v-if="beat.tlsVersion && beat.tlsLatency !== undefined"
      class="pulse-meta-pill pulse-meta-latency"
      :class="{ 'pulse-latency-warning': latencyWarning }"
      :title="handshakeTitle"
      :aria-label="latencyWarning ? handshakeTitle : undefined"
    >
      <Timer />
      <AnimatedValue :value="latency(beat.tlsLatency)" />
    </span>
  </div>
</template>
