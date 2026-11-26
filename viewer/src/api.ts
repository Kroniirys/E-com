import type { components } from './api.generated'
export type User = components['schemas']['User']
export type Product = components['schemas']['Product']
export type Category = components['schemas']['Category']
export type Address = components['schemas']['Address']
export type Stock = components['schemas']['Inventory']
export type Movement = components['schemas']['InventoryTransaction']
export type ProductInput = components['schemas']['ProductInput']
export type Page<T> = { items: T[]; page: number; page_size: number; total: number }
export type TokenResponse = components['schemas']['Token']

type ErrorBody = { error?: { code?: string; message?: string } }
const messages: Record<string, string> = {
  INVALID_INPUT: 'Thông tin chưa hợp lệ. Kiểm tra lại các trường trong biểu mẫu.',
  CONFLICT: 'Dữ liệu đã tồn tại hoặc vừa thay đổi. Kiểm tra email, mã SKU hoặc đường dẫn danh mục.',
  UNAUTHORIZED: 'Thông tin đăng nhập không đúng hoặc phiên đã hết hạn.',
  FORBIDDEN: 'Bạn không có quyền thực hiện thao tác này.',
  NOT_FOUND: 'Không tìm thấy dữ liệu hoặc dữ liệu đã được lưu trữ.',
  AUTH_UNAVAILABLE: 'Dịch vụ đăng nhập tạm thời chưa sẵn sàng. Vui lòng thử lại.',
  DEPENDENCY_UNAVAILABLE: 'Dịch vụ tạm thời bị gián đoạn. Vui lòng thử lại.',
  RATE_LIMITED: 'Bạn đã thử quá nhiều lần. Vui lòng chờ rồi thử lại.',
  LAST_ADMIN: 'Cần giữ ít nhất một quản trị viên đang hoạt động.',
  IDEMPOTENCY_CONFLICT: 'Yêu cầu đã được gửi với nội dung khác. Hãy kiểm tra lại tồn kho.',
  INSUFFICIENT_STOCK: 'Điều chỉnh này sẽ làm tồn kho âm.',
  STOCK_LIMIT: 'Số lượng vượt giới hạn cho phép.',
  COUPON_UNAVAILABLE: 'Coupon đã tắt, hết hạn, hết lượt hoặc đơn chưa đạt mức tối thiểu.',
  COUPON_LIMIT_CONFLICT: 'Giới hạn không được nhỏ hơn số lượt đã sử dụng.',
  REVIEW_INELIGIBLE: 'Bạn cần đơn đã giao chứa sản phẩm để đánh giá.',
  PENDING_INTENT: 'Cần kiểm tra kết quả yêu cầu trước trước khi tạo thao tác mới.',
  CART_CHANGED: 'Giỏ hàng đã thay đổi. Vui lòng kiểm tra và xác nhận lại.',
  CART_EMPTY: 'Giỏ hàng đang trống hoặc đã được đặt hàng.',
  QUOTE_CHANGED: 'Giá hoặc phí giao hàng đã thay đổi. Vui lòng kiểm tra và xác nhận lại.',
  PRODUCT_UNAVAILABLE:
    'Một sản phẩm đã ngừng bán hoặc không đủ tồn kho. Vui lòng kiểm tra giỏ hàng.',
  INVALID_TRANSITION: 'Trạng thái đơn hàng đã thay đổi hoặc không cho phép thao tác này.',
  ATTEMPT_PENDING: 'Hãy hoàn tất lần thanh toán đang chờ trước khi tạo lần mới.',
  PAYMENT_RETRY_LIMIT:
    'Đã đạt giới hạn 5 lần thanh toán mô phỏng. Bạn có thể hủy đơn chưa thanh toán.',
  SESSION_CHANGED:
    'Phiên đăng nhập đã thay đổi. Vui lòng kiểm tra lại thông tin rồi thực hiện lại thao tác.',
  ORIGIN_REQUIRED: 'Nguồn đăng nhập không hợp lệ. Hãy mở ứng dụng từ địa chỉ được cấu hình.',
  ORIGIN_FORBIDDEN: 'Địa chỉ ứng dụng chưa được phép kết nối.',
}
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    public retryAfter = 0,
  ) {
    super(messages[code] ?? 'Không thể hoàn tất yêu cầu. Vui lòng thử lại.')
  }
}
export function errorMessage(error: unknown): string {
  if (error instanceof TypeError)
    return 'Không thể kết nối đến dịch vụ. Kiểm tra kết nối và thử lại.'
  return error instanceof Error ? error.message : 'Không thể hoàn tất yêu cầu.'
}
export type RequestOptions = { auth?: boolean; signal?: AbortSignal; idempotencyKey?: string }
export async function http<T>(
  method: string,
  path: string,
  body?: unknown,
  token?: string | null,
  options: RequestOptions = {},
): Promise<T> {
  if (!path.startsWith('/api/v1/')) throw new Error('Đường dẫn API không hợp lệ.')
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (token) headers.Authorization = `Bearer ${token}`
  if (options.idempotencyKey) headers['Idempotency-Key'] = options.idempotencyKey
  const response = await fetch(path, {
    method,
    credentials: 'include',
    headers,
    signal: options.signal,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!response.ok) {
    const payload: ErrorBody | null = await response.json().catch(() => ({}))
    throw new ApiError(
      response.status,
      payload?.error?.code ?? 'DEPENDENCY_UNAVAILABLE',
      Number(response.headers.get('Retry-After') ?? 0),
    )
  }
  if (response.status === 204) return undefined as T
  const payload: unknown = await response.json()
  if (!payload || typeof payload !== 'object' || !('data' in payload))
    throw new Error('Dữ liệu trả về không hợp lệ.')
  return (payload as { data: T }).data
}
export class APIClient {
  constructor(
    private readToken: () => string | null,
    private refresh: () => Promise<string | null>,
    private clear: () => void,
    private readEpoch: () => number,
  ) {}
  async request<T>(
    method: string,
    path: string,
    body?: unknown,
    options: RequestOptions = {},
  ): Promise<T> {
    const authenticated = options.auth !== false
    const epoch = this.readEpoch()
    const unchanged = () => !authenticated || epoch === this.readEpoch()
    try {
      const value = await http<T>(
        method,
        path,
        body,
        authenticated ? this.readToken() : undefined,
        options,
      )
      if (!unchanged()) throw new ApiError(409, 'SESSION_CHANGED')
      return value
    } catch (error) {
      if (!unchanged()) throw new ApiError(409, 'SESSION_CHANGED')
      if (!authenticated || !(error instanceof ApiError) || error.status !== 401) throw error
      const token = await this.refresh()
      if (!unchanged()) throw new ApiError(409, 'SESSION_CHANGED')
      if (!token) {
        this.clear()
        throw error
      }
      try {
        const value = await http<T>(method, path, body, token, options)
        if (!unchanged()) throw new ApiError(409, 'SESSION_CHANGED')
        return value
      } catch (retryError) {
        if (!unchanged()) throw new ApiError(409, 'SESSION_CHANGED')
        if (retryError instanceof ApiError && retryError.status === 401) this.clear()
        throw retryError
      }
    }
  }
}
export const money = (value: number) =>
  new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(value)
