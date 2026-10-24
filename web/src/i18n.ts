import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN'

// 新增语言时只需增加语言文件并注册，业务组件继续通过键名取文案。
export const i18n = createI18n({
  legacy: false,
  locale: 'zh-CN',
  fallbackLocale: 'zh-CN',
  messages: { 'zh-CN': zhCN },
})
