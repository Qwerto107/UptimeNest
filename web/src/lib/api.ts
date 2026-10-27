import { i18n } from '@/i18n'
import { translateApiError } from '@/locales/api-errors'

type RequestOptions = RequestInit & { revalidate?: boolean }
const publicResponses = new Map<string, { etag: string; data: unknown }>()

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message)
  }
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { revalidate, ...init } = options
  const conditional = revalidate && path.startsWith('/public/') && (!init.method || init.method === 'GET')
  const cached = conditional ? publicResponses.get(path) : undefined
  const headers = new Headers(init.headers)
  if (init.body) headers.set('Content-Type', 'application/json')
  if (cached) headers.set('If-None-Match', cached.etag)
  let response: Response
  try {
    response = await fetch(`/api${path}`, {
      ...init,
      ...(conditional ? { cache: 'no-store' as const } : {}),
      headers,
      credentials: 'same-origin',
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error
    throw new ApiError(i18n.global.t('common.networkError'), 0)
  }
  if (response.status === 304 && cached) return cached.data as T
  const data = await response.json().catch(() => null)
  if (!response.ok) {
    if (response.status === 401 && path !== '/admin/session') {
      window.dispatchEvent(new Event('session-expired'))
    }
    let message = i18n.global.t('common.requestError')
    if (typeof data?.error === 'string' && data.error) {
      message = translateApiError(data.error, i18n.global.locale.value)
    }
    throw new ApiError(message, response.status)
  }
  // 仅缓存公开响应，最多8个请求键，管理配置和凭据不进入条件请求缓存。
  const etag = response.headers.get('ETag')
  if (conditional && etag) {
    publicResponses.delete(path)
    if (publicResponses.size >= 8) publicResponses.delete(publicResponses.keys().next().value!)
    publicResponses.set(path, { etag, data })
  }
  return data as T
}

export function send<T>(path: string, method: string, data?: unknown) {
  return request<T>(path, {
    method,
    body: data === undefined ? undefined : JSON.stringify(data),
  })
}
