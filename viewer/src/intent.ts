import { ApiError } from './api'
import type { APIClient } from './api'

type Intent = { key: string; path: string; method: string; body?: unknown; fingerprint: string }
const storageKey = (user: string, operation: string) => `ecommerce-intent:${user}:${operation}`
export function readIntent(user: string, operation: string): Intent | null {
  try {
    const value: unknown = JSON.parse(sessionStorage.getItem(storageKey(user, operation)) ?? 'null')
    if (!value || typeof value !== 'object') return null
    const v = value as Partial<Intent>
    if (
      typeof v.key !== 'string' ||
      !/^[a-f0-9-]{36}$/.test(v.key) ||
      typeof v.path !== 'string' ||
      !v.path.startsWith('/api/v1/') ||
      !['POST', 'PATCH'].includes(v.method ?? '') ||
      typeof v.fingerprint !== 'string'
    )
      return null
    return v as Intent
  } catch {
    return null
  }
}
export async function performIntent<T>(
  api: APIClient,
  user: string,
  operation: string,
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const fingerprint = JSON.stringify({ method, path, body })
  const previous = readIntent(user, operation)
  if (previous && previous.fingerprint !== fingerprint) throw new ApiError(409, 'PENDING_INTENT')
  const intent = previous ?? { key: crypto.randomUUID(), path, method, body, fingerprint }
  const key = storageKey(user, operation)
  sessionStorage.setItem(key, JSON.stringify(intent))
  const remove = () => {
    if (readIntent(user, operation)?.key === intent.key) sessionStorage.removeItem(key)
  }
  try {
    const result = await api.request<T>(method, path, body, { idempotencyKey: intent.key })
    remove()
    return result
  } catch (error) {
    if (
      error instanceof ApiError &&
      error.status >= 400 &&
      error.status < 500 &&
      [
        'INVALID_INPUT',
        'QUOTE_CHANGED',
        'CART_CHANGED',
        'CART_EMPTY',
        'PRODUCT_UNAVAILABLE',
        'INSUFFICIENT_STOCK',
        'INVALID_TRANSITION',
        'ATTEMPT_PENDING',
        'PAYMENT_RETRY_LIMIT',
        'STOCK_LIMIT',
        'NOT_FOUND',
        'IDEMPOTENCY_CONFLICT',
      ].includes(error.code)
    )
      remove()
    throw error
  }
}
