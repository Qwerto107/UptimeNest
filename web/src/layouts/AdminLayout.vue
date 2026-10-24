<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  Activity,
  Folder,
  PanelsTopLeft,
  Settings2,
  UserRound,
  LogOut,
  ArrowUpRight,
  Menu,
  X,
} from 'lucide-vue-next'
import { send } from '@/lib/api'
import { bootstrap, loadBootstrap } from '@/composables/useApp'
import { useAction, notify } from '@/composables/useFeedback'
import Brand from '@/components/Brand.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { Button } from '@/components/ui/button'
const { t } = useI18n()
const router = useRouter()
const mobileMenu = ref(false)
const { busy, run } = useAction()
const links = [
  { path: '/admin/monitors', label: 'nav.monitors', icon: Activity },
  { path: '/admin/groups', label: 'nav.groups', icon: Folder },
  { path: '/admin/panels', label: 'nav.panels', icon: PanelsTopLeft },
  { path: '/admin/settings', label: 'nav.settings', icon: Settings2 },
  { path: '/admin/account', label: 'nav.account', icon: UserRound },
]
onMounted(() => {
  void loadBootstrap().catch((error) => notify(error.message, true))
})
async function logout() {
  await run(async () => {
    await send('/admin/session', 'DELETE')
    await router.replace('/admin/login')
  })
}
</script>
<template>
  <div class="admin-shell">
    <header class="admin-header">
      <RouterLink to="/admin"><Brand /></RouterLink>
      <span class="admin-label">{{ t('nav.admin') }}</span>
      <div class="header-spacer" />
      <Button as-child variant="ghost" class="public-link">
        <RouterLink
          :to="bootstrap?.defaultPanel ? `/status/${bootstrap.defaultPanel}` : '/'"
          target="_blank"
        >
          {{ t('nav.public') }}
          <ArrowUpRight />
        </RouterLink>
      </Button>
      <ThemeToggle />
      <Button
        variant="ghost"
        size="icon"
        class="mobile-menu-button"
        :aria-label="t('nav.menu')"
        :aria-expanded="mobileMenu"
        @click="mobileMenu = !mobileMenu"
      >
        <X v-if="mobileMenu" />
        <Menu v-else />
      </Button>
    </header>
    <aside class="admin-sidebar" :class="{ 'sidebar-open': mobileMenu }">
      <nav :aria-label="t('nav.menu')">
        <RouterLink
          v-for="link in links"
          :key="link.path"
          :to="link.path"
          class="sidebar-link"
          @click="mobileMenu = false"
        >
          <component :is="link.icon" :size="18" />
          <span>{{ t(link.label) }}</span>
        </RouterLink>
      </nav>
      <div class="sidebar-bottom">
        <Button variant="ghost" class="w-full justify-start" :disabled="busy" @click="logout">
          <LogOut />
          {{ t('nav.logout') }}
        </Button>
        <span>v0.1.0</span>
      </div>
    </aside>
    <button
      v-if="mobileMenu"
      class="sidebar-backdrop"
      :aria-label="t('common.close')"
      @click="mobileMenu = false"
    />
    <main class="admin-main">
      <RouterView v-slot="{ Component, route }">
        <component :is="Component" :key="route.fullPath" />
      </RouterView>
    </main>
  </div>
</template>
