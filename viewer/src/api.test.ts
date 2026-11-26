import { afterEach, describe, expect, it, vi } from 'vitest'
import { APIClient } from './api'
import { SessionRuntime } from './session-runtime'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}
const unauthorized = () =>
  new Response(JSON.stringify({ error: { code: 'UNAUTHORIZED' } }), {
    status: 401,
    headers: { 'Content-Type': 'application/json' },
  })
const success = (data: unknown) =>
  new Response(JSON.stringify({ data }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
afterEach(() => {
  vi.unstubAllGlobals()
})

describe('request ownership across session changes', () => {
  it('never replays an old mutation with a new user token', async () => {
    let epoch = 1
    const pending = deferred<Response>()
    const fetcher = vi.fn().mockReturnValueOnce(pending.promise)
    vi.stubGlobal('fetch', fetcher)
    const refresh = vi.fn().mockResolvedValue('user-B')
    const clear = vi.fn()
    const client = new APIClient(
      () => 'user-A',
      refresh,
      clear,
      () => epoch,
    )
    const request = client.request('PATCH', '/api/v1/me', { name: 'User A edit' })
    epoch = 2
    pending.resolve(unauthorized())
    await expect(request).rejects.toMatchObject({ code: 'SESSION_CHANGED' })
    expect(refresh).not.toHaveBeenCalled()
    expect(clear).not.toHaveBeenCalled()
    expect(fetcher).toHaveBeenCalledTimes(1)
  })
  it('does not clear a new session when an old retry returns 401', async () => {
    let epoch = 1
    const retry = deferred<Response>()
    const entered = deferred<void>()
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce(unauthorized())
      .mockImplementationOnce(() => {
        entered.resolve()
        return retry.promise
      })
    vi.stubGlobal('fetch', fetcher)
    const clear = vi.fn()
    const client = new APIClient(
      () => 'A',
      async () => 'A-refreshed',
      clear,
      () => epoch,
    )
    const request = client.request('GET', '/api/v1/me/addresses')
    await entered.promise
    epoch = 2
    retry.resolve(unauthorized())
    await expect(request).rejects.toMatchObject({ code: 'SESSION_CHANGED' })
    expect(clear).not.toHaveBeenCalled()
  })
  it('does not replay when identity changes during refresh', async () => {
    let epoch = 1
    const refreshed = deferred<string | null>()
    const entered = deferred<void>()
    const fetcher = vi.fn().mockResolvedValueOnce(unauthorized())
    vi.stubGlobal('fetch', fetcher)
    const clear = vi.fn()
    const client = new APIClient(
      () => 'A',
      () => {
        entered.resolve()
        return refreshed.promise
      },
      clear,
      () => epoch,
    )
    const request = client.request('PATCH', '/api/v1/me', { name: 'A edit' })
    await entered.promise
    epoch = 2
    refreshed.resolve('B')
    await expect(request).rejects.toMatchObject({ code: 'SESSION_CHANGED' })
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(clear).not.toHaveBeenCalled()
  })
  it('discards data returned by a request from a replaced session', async () => {
    let epoch = 1
    const pending = deferred<Response>()
    vi.stubGlobal('fetch', vi.fn().mockReturnValueOnce(pending.promise))
    const client = new APIClient(
      () => 'A',
      async () => null,
      vi.fn(),
      () => epoch,
    )
    const request = client.request('GET', '/api/v1/me/addresses')
    epoch = 2
    pending.resolve(success({ private: 'A' }))
    await expect(request).rejects.toMatchObject({ code: 'SESSION_CHANGED' })
  })
  it('refreshes once and preserves the idempotency key on a legitimate retry', async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce(unauthorized())
      .mockResolvedValueOnce(success({ quantity: 3 }))
    vi.stubGlobal('fetch', fetcher)
    const refresh = vi.fn().mockResolvedValue('new-A')
    const client = new APIClient(
      () => 'A',
      refresh,
      vi.fn(),
      () => 1,
    )
    await expect(
      client.request(
        'POST',
        '/api/v1/admin/inventory/adjustments',
        { delta: 1 },
        { idempotencyKey: 'same-key-12345678' },
      ),
    ).resolves.toEqual({ quantity: 3 })
    expect(refresh).toHaveBeenCalledTimes(1)
    expect(fetcher.mock.calls[1][1].headers).toMatchObject({
      Authorization: 'Bearer new-A',
      'Idempotency-Key': 'same-key-12345678',
    })
  })
})

it('coalesces concurrent refresh attempts in the session runtime', async () => {
  const refreshResponse = deferred<Response>()
  const refreshEntered = deferred<void>()
  let refreshCalls = 0
  const user = {
    id: 'user-A',
    name: 'A',
    email: 'a@test.invalid',
    role: 'Customer',
    status: 'Active',
    created_at: '',
    updated_at: '',
  }
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string, init: RequestInit) => {
      if (path.endsWith('/auth/login')) return success({ access_token: 'A1' })
      if (path.endsWith('/auth/me')) return success(user)
      if (path.endsWith('/auth/refresh')) {
        refreshCalls += 1
        refreshEntered.resolve()
        return refreshResponse.promise
      }
      if ((init.headers as Record<string, string>).Authorization === 'Bearer A1')
        return unauthorized()
      return success({ items: [] })
    }),
  )
  const runtime = new SessionRuntime()
  await runtime.login('a@test.invalid', 'not-a-real-password')
  const first = runtime.api.request('GET', '/api/v1/me/addresses')
  const second = runtime.api.request('GET', '/api/v1/me/addresses')
  await refreshEntered.promise
  refreshResponse.resolve(success({ access_token: 'A2' }))
  await Promise.all([first, second])
  expect(refreshCalls).toBe(1)
})
