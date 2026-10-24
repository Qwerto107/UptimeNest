export type MonitorStatus = 'up' | 'down' | 'unknown' | 'paused'
export type MonitorType = 'http' | 'tcp' | 'ping' | 'dns'

export interface Heartbeat {
  id: number
  monitorId: number
  time: number
  status: MonitorStatus
  latency: number
  message: string
  tlsVersion?: string
  httpVersion?: string
  tlsLatency?: number
  certExpiry?: number
  certIssuerOrg?: string
  tlsKeyExchange?: string
  tlsCipherSuite?: string
  quicVersion?: string
  zeroRTT?: boolean
}

export interface Summary {
  status: MonitorStatus
  uptime: number | null
  average: number | null
  latest: Heartbeat | null
  heartbeats: Heartbeat[]
}

export interface Monitor {
  id: number
  name: string
  type: MonitorType
  target: string
  hideAddress: boolean
  port: number
  dnsRecord: string
  dnsServer: string
  expected: string
  interval: number
  retries: number
  retryInterval: number
  timeout: number
  invert: boolean
  description: string
  tags: string[]
  groupId: number
  enabled: boolean
  http?: HTTPOptions
}

export interface HTTPAuth {
  type: 'none' | 'basic' | 'oauth2' | 'mtls'
  username: string
  password: string
  tokenURL: string
  clientID: string
  clientSecret: string
  scope: string
  tokenAuthMethod: 'header' | 'body'
  clientCert: string
  clientKey: string
  caCert: string
}

export interface HTTPOptions {
  version: 'standard' | 'auto' | 'http3'
  maxRedirects: number
  validStatusCodes: string[]
  method: string
  bodyEncoding: 'json' | 'form' | 'xml' | 'text'
  body: string
  headers: string
  auth: HTTPAuth
}

export interface MonitorView extends Monitor {
  summary: Summary
}
export interface Group {
  id: number
  name: string
  parentId: number
}
export interface PanelGroup {
  id: string
  name: string
  monitorIds: number[]
}
export interface Panel {
  id: number
  name: string
  slug: string
  description: string
  monitorIds: number[]
  groups: PanelGroup[]
  isDefault: boolean
}
export interface Settings {
  timezone: string
  language: string
  entryPage: 'status' | 'admin'
  userAgent: string
  retentionDays: number
}
export interface Bootstrap {
  timezone: string
  language: string
  entryPage: string
  defaultPanel: string
  panels: { name: string; slug: string }[]
}
export interface Session {
  initialized: boolean
  authenticated: boolean
}
export interface MonitorLog {
  id: number
  monitorId: number
  time: number
  status: MonitorStatus
  message: string
}
export type PublicMonitor = Pick<
  MonitorView,
  'id' | 'name' | 'type' | 'description' | 'tags' | 'summary'
> & { address?: string; protocol: string }
export interface PublicPanel {
  panel: Pick<Panel, 'name' | 'slug' | 'description'>
  monitors: PublicMonitor[]
  groups: PanelGroup[]
  timezone: string
}
