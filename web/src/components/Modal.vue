<script setup lang="ts">
import {
  DialogRoot,
  DialogPortal,
  DialogOverlay,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from 'reka-ui'
import { X } from 'lucide-vue-next'
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
  description?: string
  wide?: boolean
  busy?: boolean
}>()
const { t } = useI18n()
</script>
<template>
  <DialogRoot
    :open="open"
    @update:open="
      (value) => {
        if (!busy) open = value
      }
    "
  >
    <DialogPortal>
      <DialogOverlay class="modal-overlay" />
      <DialogContent
        class="modal-content"
        :class="{ 'modal-wide': wide }"
        @escape-key-down="
          (event) => {
            if (busy) event.preventDefault()
          }
        "
        @interact-outside="
          (event) => {
            if (busy) event.preventDefault()
          }
        "
      >
        <div class="modal-heading">
          <div>
            <DialogTitle class="text-xl font-semibold">{{ title }}</DialogTitle>
            <DialogDescription
              :class="description ? 'mt-2 text-sm text-muted-foreground' : 'sr-only'"
            >
              {{ description || title }}
            </DialogDescription>
          </div>
          <DialogClose as-child>
            <Button variant="ghost" size="icon" :disabled="busy" :aria-label="t('common.close')">
              <X />
            </Button>
          </DialogClose>
        </div>
        <slot />
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
