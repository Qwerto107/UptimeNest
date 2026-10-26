import { createApp } from 'vue'
import { i18n } from './i18n'
import { router } from './router'
import { initTheme } from './composables/useApp'
import { version } from '../package.json'
import App from './App.vue'
import './styles.css'

console.info(
  `%c ${i18n.global.t('common.brand')} %c v${version}_${import.meta.env.VITE_COMMIT_HASH} `,
  'background:#087f5b;color:#fff;padding:3px 6px;border-radius:3px 0 0 3px;font-family:monospace;',
  'background:#d1fae5;color:#065f46;padding:3px 6px;border-radius:0 3px 3px 0;font-family:monospace;',
)
console.info('%c' + i18n.global.t('status.copyright'), 'color:#087f5b;font-family:monospace;')

initTheme()
createApp(App).use(i18n).use(router).mount('#app')
