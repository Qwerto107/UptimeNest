<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Heartbeat } from '@/types'
import { certificate, formatTime, latency } from '@/lib/format'
defineProps<{ beat: Heartbeat | null }>()
const { t } = useI18n()
</script>
<template>
  <dl class="connection-details">
    <div>
      <dt>{{ t('status.latest') }}</dt>
      <dd>{{ formatTime(beat?.time) }}</dd>
    </div>
    <div v-if="beat?.httpVersion">
      <dt>{{ t('status.httpVersion') }}</dt>
      <dd>{{ beat.httpVersion }}</dd>
    </div>
    <div v-if="beat?.tlsVersion">
      <dt>{{ t('status.tlsVersion') }}</dt>
      <dd>{{ beat.tlsVersion }}</dd>
    </div>
    <div v-if="beat?.tlsVersion">
      <dt>{{ t('status.handshake') }}</dt>
      <dd>{{ latency(beat.tlsLatency) }}</dd>
    </div>
    <div v-if="beat?.tlsKeyExchange">
      <dt>{{ t('status.keyExchange') }}</dt>
      <dd>{{ beat.tlsKeyExchange }}</dd>
    </div>
    <div v-if="beat?.tlsCipherSuite">
      <dt>{{ t('status.cipherSuite') }}</dt>
      <dd>{{ beat.tlsCipherSuite }}</dd>
    </div>
    <div v-if="beat?.httpVersion?.startsWith('HTTP/3') && beat.quicVersion">
      <dt>{{ t('status.quicVersion') }}</dt>
      <dd>{{ beat.quicVersion }}</dd>
    </div>
    <div v-if="beat?.httpVersion?.startsWith('HTTP/3') && beat.zeroRTT !== undefined">
      <dt>{{ t('status.zeroRTT') }}</dt>
      <dd>{{ t(beat.zeroRTT ? 'status.used' : 'status.notUsed') }}</dd>
    </div>
    <div v-if="beat?.certExpiry">
      <dt>{{ t('status.certificate') }}</dt>
      <dd :title="formatTime(beat.certExpiry)">
        {{ certificate(beat.certExpiry) }}
      </dd>
    </div>
    <div v-if="beat?.certIssuerOrg">
      <dt>{{ t('status.issuer') }}</dt>
      <dd>{{ beat.certIssuerOrg }}</dd>
    </div>
  </dl>
</template>
