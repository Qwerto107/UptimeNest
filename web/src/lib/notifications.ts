import type { AlertOptions, NotificationChannel, ChannelType } from '@/types'

export function defaultAlerts(): AlertOptions {
  return {
    service: { enabled: false, channelIds: [], recovery: false, repeatMinutes: 0, days: 0, failureThreshold: 1 },
    certificate: { enabled: false, channelIds: [], recovery: false, repeatMinutes: 0, days: 0, failureThreshold: 0 },
  }
}

export const channelTypes: ChannelType[] = ['email', 'webhook', 'feishu', 'dingtalk', 'telegram']

export interface TemplateIssue {
  kind: 'unknownVariable' | 'variableSyntax' | 'jsonSyntax' | 'jsonKeyVariable'
  name?: string
}

// 与服务端保持一致：只替换英文变量；JSON 先解析，再检查字符串值。
export function templateIssue(text: string, variables: string[], json = false): TemplateIssue | undefined {
  function check(value: string): TemplateIssue | undefined {
    let unknown: string | undefined
    const remainder = value.replace(/\{\{[ \t\n\f\r]*([a-z_]+)[ \t\n\f\r]*\}\}/g, (_, name: string) => {
      if (!variables.includes(name)) unknown = name
      return ''
    })
    if (unknown) return { kind: 'unknownVariable', name: unknown }
    if (remainder.includes('{{') || remainder.includes('}}')) return { kind: 'variableSyntax' }
  }
  if (!json || !text) return check(text)
  let value: unknown
  try { value = JSON.parse(text) }
  catch { return { kind: 'jsonSyntax' } }
  const pending = [value]
  while (pending.length) {
    const item = pending.pop()
    if (typeof item === 'string') {
      const issue = check(item)
      if (issue) return issue
    } else if (item && typeof item === 'object') {
      for (const [key, next] of Object.entries(item)) {
        if (!Array.isArray(item) && key.includes('{{')) return { kind: 'jsonKeyVariable' }
        pending.push(next)
      }
    }
  }
}

export function defaultChannel(): NotificationChannel {
  return {
    id: 0, name: '', type: 'email', enabled: true, subject: '', body: '', certificateTemplate: { subject: '', body: '' }, configured: [],
    config: { url: '', secret: '', headers: {}, host: '', port: 587, security: 'starttls', username: '', password: '', from: '', to: [], token: '', chatId: '', threadId: 0, proxyURL: '' },
  }
}
