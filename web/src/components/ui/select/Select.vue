<script setup lang="ts" generic="T extends string | number">
import { computed, ref, watch, type HTMLAttributes } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, ChevronDown, ChevronUp } from 'lucide-vue-next'
import {
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectIcon,
  SelectPortal,
  SelectContent,
  SelectViewport,
  SelectGroup,
  SelectLabel,
  SelectItem,
  SelectItemText,
  SelectItemIndicator,
  SelectScrollUpButton,
  SelectScrollDownButton,
} from 'reka-ui'
import { cn } from '@/lib/utils'
import type { SelectOption, SelectOptionGroup } from './types'

defineOptions({ inheritAttrs: false })
const props = defineProps<{
  modelValue?: T | ''
  options?: readonly SelectOption<T>[]
  groups?: readonly SelectOptionGroup<T>[]
  placeholder?: string
  id?: string
  name?: string
  required?: boolean
  disabled?: boolean
  side?: 'top' | 'bottom'
  class?: HTMLAttributes['class']
}>()
const emit = defineEmits<{ 'update:modelValue': [value: T] }>()
const { t } = useI18n()
const open = ref(false)
const optionGroups = computed(() => props.groups || [{ options: props.options || [] }])
const options = computed(() => optionGroups.value.flatMap((group) => group.options))
const selected = computed(() => options.value.find((option) => option.value === props.modelValue))
watch(() => props.disabled, (disabled) => { if (disabled) open.value = false })

function updateValue(value: unknown) {
  // 底层用字符串定位选项，向业务表单返回原始类型，避免数字 ID 被转换成字符串。
  const option = options.value.find((option) => String(option.value) === value)
  if (option && !option.disabled && !props.disabled) emit('update:modelValue', option.value)
}
</script>

<template>
  <SelectRoot
    v-model:open="open"
    :model-value="String(modelValue ?? '')"
    :disabled="disabled"
    :required="required"
    :name="name ?? id"
    @update:model-value="updateValue"
  >
    <SelectTrigger :id="id" v-bind="$attrs" :class="cn('control ui-select-trigger', props.class)">
      <SelectValue :placeholder="placeholder ?? t('common.select')">
        {{ selected?.label ?? placeholder ?? t('common.select') }}
      </SelectValue>
      <SelectIcon class="ui-select-icon"><ChevronDown :size="15" /></SelectIcon>
    </SelectTrigger>
    <SelectPortal>
      <SelectContent
        position="popper"
        :side="side"
        :side-offset="4"
        :collision-padding="8"
        class="ui-select-content"
      >
        <SelectScrollUpButton class="ui-select-scroll"><ChevronUp :size="15" /></SelectScrollUpButton>
        <SelectViewport class="ui-select-viewport">
          <SelectGroup v-for="(group, index) in optionGroups" :key="index" class="ui-select-group">
            <SelectLabel v-if="group.label" class="ui-select-label">{{ group.label }}</SelectLabel>
            <SelectItem
              v-for="option in group.options"
              :key="String(option.value)"
              :value="String(option.value)"
              :disabled="option.disabled"
              :text-value="option.label"
              class="ui-select-item"
            >
              <SelectItemText>{{ option.label }}</SelectItemText>
              <SelectItemIndicator class="ui-select-check"><Check :size="14" /></SelectItemIndicator>
            </SelectItem>
          </SelectGroup>
        </SelectViewport>
        <SelectScrollDownButton class="ui-select-scroll"><ChevronDown :size="15" /></SelectScrollDownButton>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>
