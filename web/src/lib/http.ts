import type { HTTPOptions } from '@/types'

// 与后端兼容默认值一致，旧监控编辑后不改变既有检测行为。
export function defaultHTTPOptions(): HTTPOptions {
  return {
    version: 'standard',
    maxRedirects: 10,
    validStatusCodes: ['200-299', '300-399'],
    method: 'GET',
    bodyEncoding: 'json',
    body: '',
    headers: '',
    auth: {
      type: 'none',
      username: '',
      password: '',
      tokenURL: '',
      clientID: '',
      clientSecret: '',
      scope: '',
      tokenAuthMethod: 'header',
      clientCert: '',
      clientKey: '',
      caCert: '',
    },
  }
}
