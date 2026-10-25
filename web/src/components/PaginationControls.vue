<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'

const page = defineModel<number>('page', { required: true })
const pageSize = defineModel<number>('pageSize', { required: true })
const props = defineProps<{ total: number; label: string }>()
const { t } = useI18n()
const pages = computed(() => Math.max(1, Math.ceil(props.total / pageSize.value)))
</script>

<template>
  <nav v-if="total > 25" class="list-pagination" :aria-label="t('pagination.label', { name: label })">
    <span>{{ label }} {{ (page - 1) * pageSize + 1 }}–{{ Math.min(page * pageSize, total) }} / {{ total }}</span>
    <div class="pagination-actions">
      <Select
        v-model="pageSize"
        side="top"
        :aria-label="t('pagination.size')"
        :options="[25, 50, 100].map((size) => ({ value: size, label: t('pagination.perPage', { count: size }) }))"
      />
      <Button type="button" variant="outline" size="sm" :disabled="page <= 1" @click="page--">
        {{ t('pagination.previous') }}
      </Button>
      <span class="pagination-current" aria-live="polite">{{ t('pagination.page', { page, pages }) }}</span>
      <Button type="button" variant="outline" size="sm" :disabled="page >= pages" @click="page++">
        {{ t('pagination.next') }}
      </Button>
    </div>
  </nav>
</template>
