import { i18n } from '@/i18n'
import { translateApiError } from '@/locales/api-errors'

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message)
  }
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  if (options.body) headers.set('Content-Type', 'application/json')
  let response: Response
  try {
    response = await fetch(`/api${path}`, {
      ...options,
      headers,
      credentials: 'same-origin',
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error
    throw new ApiError(i18n.global.t('common.networkError'), 0)
  }
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
  return data as T
}

export function send<T>(path: string, method: string, data?: unknown) {
  return request<T>(path, {
    method,
    body: data === undefined ? undefined : JSON.stringify(data),
  })
}
