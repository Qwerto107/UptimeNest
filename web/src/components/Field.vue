<script setup lang="ts">
import { cloneVNode, useSlots } from 'vue'
const props = defineProps<{ id: string; label: string; hint?: string; required?: boolean }>()
const slots = useSlots()

// 根据控件 ID 自动关联帮助文字，避免每个业务表单重复拼接无障碍属性。
const Control = () =>
  (slots.default?.() || []).map((node) => {
    if (!props.hint || node.props?.id !== props.id) return node
    const describedBy = [node.props['aria-describedby'], `${props.id}-hint`]
      .filter(Boolean)
      .join(' ')
    return cloneVNode(node, { 'aria-describedby': describedBy })
  })
</script>
<template>
  <div class="field">
    <label :for="id" class="field-label">
      {{ label }}
      <span v-if="required" aria-hidden="true" class="text-destructive">*</span>
    </label>
    <Control />
    <p v-if="hint" :id="`${id}-hint`" class="field-hint">{{ hint }}</p>
  </div>
</template>
