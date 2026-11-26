import { afterEach, expect, it, vi } from 'vitest'
import { APIClient } from './api'
import { performIntent, readIntent } from './intent'
const client = () =>
  new APIClient(
    () => 'test-token',
    async () => null,
    () => {},
    () => 1,
  )
afterEach(() => {
  sessionStorage.clear()
  vi.unstubAllGlobals()
})
it('keeps the same key and request across a lost response and a page reload', async () => {
  const fetcher = vi
    .fn()
    .mockRejectedValueOnce(new TypeError('connection lost'))
    .mockResolvedValueOnce(
      new Response(JSON.stringify({ data: { id: 'committed-order' } }), { status: 201 }),
    )
  vi.stubGlobal('fetch', fetcher)
  const body = { address_id: 'address', cart_version: 2, quote_hash: 'hash', payment_method: 'COD' }
  await expect(
    performIntent(client(), 'user', 'checkout', 'POST', '/api/v1/orders', body),
  ).rejects.toThrow('connection lost')
  const saved = readIntent('user', 'checkout')!
  expect(saved.body).toEqual(body)
  await expect(
    performIntent(client(), 'user', 'checkout', saved.method, saved.path, saved.body),
  ).resolves.toEqual({ id: 'committed-order' })
  const first = fetcher.mock.calls[0][1] as RequestInit
  const retry = fetcher.mock.calls[1][1] as RequestInit
  expect(first.headers).toMatchObject({ 'Idempotency-Key': saved.key })
  expect(retry.headers).toMatchObject({ 'Idempotency-Key': saved.key })
  expect(readIntent('user', 'checkout')).toBeNull()
})
it('requires resolving an uncertain intent before a new payload and isolates accounts', async () => {
  const fetcher = vi.fn().mockRejectedValue(new TypeError('offline'))
  vi.stubGlobal('fetch', fetcher)
  await expect(
    performIntent(client(), 'A', 'checkout', 'POST', '/api/v1/orders', { cart_version: 1 }),
  ).rejects.toThrow()
  await expect(
    performIntent(client(), 'A', 'checkout', 'POST', '/api/v1/orders', { cart_version: 2 }),
  ).rejects.toMatchObject({ code: 'PENDING_INTENT' })
  expect(fetcher).toHaveBeenCalledTimes(1)
  expect(readIntent('B', 'checkout')).toBeNull()
})
it('releases a rejected quote so the reviewed quote is a new intent', async () => {
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ error: { code: 'QUOTE_CHANGED' } }), { status: 409 }),
      ),
  )
  await expect(
    performIntent(client(), 'A', 'checkout', 'POST', '/api/v1/orders', { cart_version: 1 }),
  ).rejects.toMatchObject({ code: 'QUOTE_CHANGED' })
  expect(readIntent('A', 'checkout')).toBeNull()
})
it('preserves an uncertain checkout through a refresh rate limit', async () => {
  const fetcher = vi
    .fn()
    .mockRejectedValueOnce(new TypeError('lost response'))
    .mockResolvedValueOnce(
      new Response(JSON.stringify({ error: { code: 'RATE_LIMITED' } }), {
        status: 429,
        headers: { 'Retry-After': '20' },
      }),
    )
    .mockResolvedValueOnce(
      new Response(JSON.stringify({ data: { id: 'existing-order' } }), { status: 201 }),
    )
  vi.stubGlobal('fetch', fetcher)
  const body = { cart_version: 1 }
  await expect(
    performIntent(client(), 'A', 'checkout', 'POST', '/api/v1/orders', body),
  ).rejects.toThrow()
  const key = readIntent('A', 'checkout')!.key
  await expect(
    performIntent(client(), 'A', 'checkout', 'POST', '/api/v1/orders', body),
  ).rejects.toMatchObject({ status: 429 })
  expect(readIntent('A', 'checkout')!.key).toBe(key)
  await expect(
    performIntent(client(), 'A', 'checkout', 'POST', '/api/v1/orders', body),
  ).resolves.toEqual({ id: 'existing-order' })
  expect((fetcher.mock.calls[2][1] as RequestInit).headers).toMatchObject({
    'Idempotency-Key': key,
  })
})
