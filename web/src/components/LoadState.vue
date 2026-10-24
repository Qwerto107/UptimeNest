<script setup lang="ts">
import { AlertCircle, LoaderCircle } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
defineProps<{ loading?: boolean; error?: string; hasData?: boolean }>()
defineEmits<{ retry: [] }>()
const { t } = useI18n()
</script>
<template>
  <div v-if="loading" class="load-state" role="status">
    <LoaderCircle class="size-5 animate-spin" />
    {{ t('common.loading') }}
  </div>
  <div v-else-if="error" class="error-state" role="alert">
    <AlertCircle class="size-5 shrink-0" />
    <div>
      <p>{{ error }}</p>
      <p v-if="hasData" class="mt-1 text-xs">{{ t('common.previousData') }}</p>
    </div>
    <Button variant="outline" size="sm" @click="$emit('retry')">{{ t('common.retry') }}</Button>
  </div>
</template>
