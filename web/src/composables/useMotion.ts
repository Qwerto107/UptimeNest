import { onScopeDispose, watch } from 'vue'
import { usePreferredReducedMotion } from '@vueuse/core'

export const motionEase = 'cubic-bezier(.2,.7,.2,1)'

// 单次动效集中处理取消与减少动态效果，组件卸载后不遗留动画。
export function useMotion() {
  const reducedMotion = usePreferredReducedMotion()
  const active = new Set<Animation>()
  const elements = new WeakMap<Element, Animation>()
  function cancelAll() {
    for (const animation of active) animation.cancel()
    active.clear()
  }
  function animate(element: Element | undefined | null, frames: Keyframe[], duration = 200, delay = 0, easing = motionEase) {
    if (!element || reducedMotion.value === 'reduce' || !element.animate) return undefined
    elements.get(element)?.cancel()
    const animation = element.animate(frames, { duration, delay, easing, fill: 'backwards' })
    elements.set(element, animation)
    active.add(animation)
    void animation.finished.catch(() => {}).finally(() => active.delete(animation))
    return animation
  }
  watch(reducedMotion, (value) => { if (value === 'reduce') cancelAll() })
  onScopeDispose(cancelAll)
  return { animate, reducedMotion }
}
