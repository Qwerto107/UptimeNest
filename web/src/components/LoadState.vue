<script setup lang="ts">
import { AlertCircle, LoaderCircle } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { onBeforeUnmount, ref, watch } from 'vue'
import PageSkeleton from '@/components/PageSkeleton.vue'

const props = defineProps<{ loading?: boolean; error?: string; hasData?: boolean; skeleton?: 'status' | 'monitors' | 'panels' }>()
const showSkeleton = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
watch(() => props.loading, (loading) => {
  clearTimeout(timer)
  showSkeleton.value = false
  // 只延迟占位出现，不延迟真实数据，也不要求骨架屏至少展示多久。
  if (loading && props.skeleton) timer = setTimeout(() => { showSkeleton.value = true }, 150)
}, { immediate: true })
onBeforeUnmount(() => clearTimeout(timer))
defineEmits<{ retry: [] }>()
const { t } = useI18n()
</script>
<template>
  <div v-if="loading && (!skeleton || showSkeleton)" :class="{ 'load-state': !skeleton }" role="status" :aria-label="t('common.loading')">
    <PageSkeleton v-if="skeleton" :layout="skeleton" />
    <template v-else><LoaderCircle class="size-5" />{{ t('common.loading') }}</template>
  </div>
  <div v-else-if="!loading && error" class="error-state" role="alert">
    <AlertCircle class="size-5 shrink-0" />
    <div>
      <p>{{ error }}</p>
      <p v-if="hasData" class="mt-1 text-xs">{{ t('common.previousData') }}</p>
    </div>
    <Button variant="outline" size="sm" @click="$emit('retry')">{{ t('common.retry') }}</Button>
  </div>
</template>
