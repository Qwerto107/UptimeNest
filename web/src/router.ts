import { createRouter, createWebHistory } from 'vue-router'
import { request } from '@/lib/api'
import { loadBootstrap } from '@/composables/useApp'
import type { Session } from '@/types'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: () => import('@/views/HomeView.vue') },
    {
      path: '/status/:slug',
      component: () => import('@/views/StatusView.vue'),
    },
    { path: '/admin/login', component: () => import('@/views/LoginView.vue') },
    {
      path: '/admin',
      component: () => import('@/layouts/AdminLayout.vue'),
      meta: { admin: true },
      children: [
        { path: '', redirect: '/admin/monitors' },
        {
          path: 'monitors',
          component: () => import('@/views/MonitorsView.vue'),
        },
        {
          path: 'monitors/:id',
          component: () => import('@/views/MonitorDetailView.vue'),
        },
        { path: 'groups', redirect: '/admin/monitors' },
        { path: 'panels', component: () => import('@/views/PanelsView.vue') },
        { path: 'notifications', component: () => import('@/views/NotificationsView.vue') },
        {
          path: 'settings',
          component: () => import('@/views/SettingsView.vue'),
        },
        { path: 'account', component: () => import('@/views/AccountView.vue') },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      component: () => import('@/views/NotFoundView.vue'),
    },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach(async (to) => {
  if (to.path === '/') {
    // 确认路由前解析入口，避免先显示首页标题栏再跳转到状态页。
    try {
      const value = await loadBootstrap()
      if (value.entryPage === 'admin') return '/admin'
      if (value.defaultPanel) return `/status/${encodeURIComponent(value.defaultPanel)}`
    } catch {
      // 入口配置读取失败时，由首页保留错误提示和重试入口。
    }
    return true
  }
  if (!to.meta.admin) return true
  try {
    const session = await request<Session>('/admin/session')
    if (!session.authenticated) return '/admin/login'
  } catch {
    return '/admin/login'
  }
  return true
})
