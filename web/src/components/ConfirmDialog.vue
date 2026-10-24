<script setup lang="ts">
import {
  AlertDialogRoot,
  AlertDialogPortal,
  AlertDialogOverlay,
  AlertDialogContent,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogCancel,
} from 'reka-ui'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { watch } from 'vue'
import { clearSuccess } from '@/composables/useFeedback'
const open = defineModel<boolean>('open', { required: true })
watch(open, (value) => {
  if (value) clearSuccess()
})
defineProps<{
  title: string
  description: string
  busy?: boolean
  label?: string
}>()
defineEmits<{ confirm: [] }>()
const { t } = useI18n()
</script>
<template>
  <AlertDialogRoot
    :open="open"
    @update:open="
      (value) => {
        if (!busy) open = value
      }
    "
  >
    <AlertDialogPortal>
      <AlertDialogOverlay class="modal-overlay" />
      <AlertDialogContent
        class="modal-content confirm-content"
        @escape-key-down="
          (event) => {
            if (busy) event.preventDefault()
          }
        "
      >
        <AlertDialogTitle class="modal-title">{{ title }}</AlertDialogTitle>
        <AlertDialogDescription class="confirm-description">
          {{ description }}
        </AlertDialogDescription>
        <div class="mt-6 flex justify-end gap-3">
          <AlertDialogCancel as-child>
            <Button variant="outline" :disabled="busy">{{ t('common.cancel') }}</Button>
          </AlertDialogCancel>
          <Button variant="destructive" :disabled="busy" @click="$emit('confirm')">
            {{ label || t('common.confirm') }}
          </Button>
        </div>
      </AlertDialogContent>
    </AlertDialogPortal>
  </AlertDialogRoot>
</template>
