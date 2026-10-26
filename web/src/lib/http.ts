import type { HTTPOptions } from '@/types'

// 与后端默认值一致；监控已明确指定的配置由编辑器保留。
export function defaultHTTPOptions(): HTTPOptions {
  return {
    version: 'standard',
    connectIP: '',
    ignoreTLS: false,
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
