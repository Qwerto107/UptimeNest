import { createRouter, createWebHistory } from 'vue-router'
import { request } from '@/lib/api'
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
        { path: 'groups', component: () => import('@/views/GroupsView.vue') },
        { path: 'panels', component: () => import('@/views/PanelsView.vue') },
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
  if (!to.meta.admin) return true
  try {
    const session = await request<Session>('/admin/session')
    if (!session.authenticated) return '/admin/login'
  } catch {
    return '/admin/login'
  }
  return true
})
