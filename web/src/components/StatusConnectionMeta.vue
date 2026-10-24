<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { LockKeyhole, BadgeCheck, ShieldCheck, Zap, Timer } from 'lucide-vue-next'
import type { Heartbeat } from '@/types'
import { certificate, formatTime, latency } from '@/lib/format'

const props = defineProps<{ beat: Heartbeat | null }>()
const { t } = useI18n()
const hasMeta = computed(
  () =>
    props.beat?.httpVersion ||
    props.beat?.tlsVersion ||
    props.beat?.certExpiry ||
    props.beat?.certIssuerOrg,
)
const certificateSoon = computed(() => {
  if (!props.beat?.certExpiry) return false
  return props.beat.certExpiry - Date.now() < 30 * 86400000
})
</script>

<template>
  <div v-if="beat && hasMeta" class="pulse-connection-meta pulse-mono">
    <span
      v-if="beat.certExpiry"
      class="pulse-meta-pill pulse-meta-certificate"
      :class="{ 'pulse-certificate-soon': certificateSoon }"
      :title="t('status.certificate') + ' · ' + formatTime(beat.certExpiry)"
    >
      <LockKeyhole />
      {{ certificate(beat.certExpiry) }}
    </span>
    <span v-if="beat.certIssuerOrg" class="pulse-meta-pill pulse-meta-ca" :title="t('status.issuer')">
      <BadgeCheck />
      {{ beat.certIssuerOrg }}
    </span>
    <span v-if="beat.tlsVersion" class="pulse-meta-pill pulse-meta-tls" :title="t('status.tlsVersion')">
      <ShieldCheck />
      {{ beat.tlsVersion }}
    </span>
    <span v-if="beat.httpVersion" class="pulse-meta-pill pulse-meta-http" :title="t('status.httpVersion')">
      <Zap />
      {{ beat.httpVersion }}
    </span>
    <span
      v-if="beat.tlsVersion && beat.tlsLatency !== undefined"
      class="pulse-meta-pill pulse-meta-latency"
      :title="t('status.handshake')"
    >
      <Timer />
      {{ latency(beat.tlsLatency) }}
    </span>
  </div>
</template>
