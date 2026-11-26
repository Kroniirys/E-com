import type { components } from './api.generated'
export type Foundation = components['schemas']['Foundation']
type Failure = components['schemas']['Error']

export async function getFoundation(signal?: AbortSignal): Promise<Foundation> {
  const response = await fetch('/api/v1/foundation', {
    signal,
    headers: { Accept: 'application/json' },
  })
  if (!response.ok) {
    const failure: Failure | undefined = await response.json().catch(() => undefined)
    if (response.status === 503)
      throw new Error(
        'Dữ liệu cửa hàng chưa sẵn sàng. Kiểm tra database, migration và seed rồi thử lại.',
      )
    throw new Error(failure?.error?.message ?? 'Không thể kết nối đến dịch vụ. Vui lòng thử lại.')
  }
  const result: { data: Foundation } = await response.json()
  if (!result.data?.store?.name || result.data.database !== 'up')
    throw new Error('Dịch vụ trả về dữ liệu không hợp lệ.')
  return result.data
}
