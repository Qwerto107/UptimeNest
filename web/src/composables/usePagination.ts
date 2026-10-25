import { computed, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'

export function usePagination(total: MaybeRefOrGetter<number>, initialSize = 50) {
  const page = ref(1)
  const pageSize = ref(initialSize)
  const pageCount = computed(() => Math.max(1, Math.ceil(toValue(total) / pageSize.value)))
  const offset = computed(() => (page.value - 1) * pageSize.value)
  watch(pageCount, (count) => { page.value = Math.min(page.value, count) })
  watch(pageSize, () => { page.value = 1 })
  return { page, pageSize, offset }
}
