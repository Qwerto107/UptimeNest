import { createApp } from 'vue'
import { i18n } from './i18n'
import { router } from './router'
import { initTheme } from './composables/useApp'
import App from './App.vue'
import './styles.css'

initTheme()
createApp(App).use(i18n).use(router).mount('#app')
