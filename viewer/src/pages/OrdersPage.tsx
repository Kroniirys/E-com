import { useState } from 'react'
import { errorMessage, money } from '../api'
import type { Page } from '../api'
import type { components } from '../api.generated'
import { useSession } from '../session'
import { useLoad } from '../hooks'
import { performIntent, readIntent } from '../intent'
import { Failure, Field, ListState, Loading, Notice, Pagination, Status } from '../ui'

type Order = components['schemas']['Order']
type Attempt = components['schemas']['PaymentAttempt']
const statusNames: Record<string, string> = {
  Pending: 'Chờ xác nhận',
  Confirmed: 'Đã xác nhận',
  Processing: 'Đang xử lý',
  Shipped: 'Đang giao',
  Delivered: 'Đã giao',
  Cancelled: 'Đã hủy',
  Failed: 'Thất bại',
}
export default function OrdersPage({ id, admin = false }: { id?: string; admin?: boolean }) {
  const [page, setPage] = useState(1)
  const base = admin ? '/api/v1/admin/orders' : '/api/v1/orders'
  const list = useLoad<Page<Order>>(`${base}?page=${page}`)
  if (id) return <OrderDetail key={id} id={id} admin={admin} />
  return (
    <section className="space-y-6">
      <h1 className="text-3xl font-medium">{admin ? 'Quản lý đơn hàng' : 'Đơn hàng của tôi'}</h1>
      <ListState state={list}>
        {(data) => (
          <>
            {data.items.length === 0 ? (
              <p className="panel">Chưa có đơn hàng.</p>
            ) : (
              data.items.map((order) => (
                <a
                  className="panel flex flex-wrap items-center justify-between gap-3 !block"
                  key={order.id}
                  href={`#/${admin ? 'admin/orders' : 'orders'}/${order.id}`}
                >
                  <div className="flex flex-wrap justify-between gap-4">
                    <div>
                      <strong className="break-all">Đơn {order.id}</strong>
                      <p className="mt-2 text-sm text-stone-500">
                        {new Date(order.created_at).toLocaleString('vi-VN')} · {order.items.length}{' '}
                        sản phẩm
                      </p>
                    </div>
                    <div className="text-right">
                      <p>{statusNames[order.status]}</p>
                      <p className="mt-2 font-semibold">{money(order.total_minor)}</p>
                    </div>
                  </div>
                </a>
              ))
            )}
            <Pagination page={page} pageSize={data.page_size} total={data.total} change={setPage} />
          </>
        )}
      </ListState>
    </section>
  )
}
function OrderDetail({ id, admin }: { id: string; admin: boolean }) {
  const { api, user } = useSession()
  const state = useLoad<Order>(
    `${admin ? '/api/v1/admin/orders' : '/api/v1/orders'}/${encodeURIComponent(id)}`,
  )
  const operation = `order:${id}`
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [pending, setPending] = useState(() => readIntent(user!.id, operation))
  const action = async (method: string, path: string, body?: unknown) => {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await performIntent<unknown>(api, user!.id, operation, method, path, body)
      setNotice('Đã cập nhật đơn hàng.')
      state.reload()
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setPending(readIntent(user!.id, operation))
      setBusy(false)
    }
  }
  if (state.loading) return <Loading />
  if (state.error || !state.data)
    return <Failure message={state.error ?? 'Không tìm thấy đơn hàng.'} retry={state.reload} />
  const o = state.data
  const pendingAttempt = o.payment.attempts.find((a) => a.status === 'Pending')
  const transitions = admin
    ? o.status === 'Pending'
      ? o.payment.method === 'COD'
        ? ['Confirmed', 'Cancelled', 'Failed']
        : ['Cancelled', 'Failed']
      : o.status === 'Confirmed'
        ? ['Processing', ...(o.payment.status === 'Pending' ? ['Cancelled'] : [])]
        : o.status === 'Processing'
          ? ['Shipped']
          : o.status === 'Shipped'
            ? ['Delivered']
            : []
    : []
  return (
    <section className="space-y-6">
      <a className="underline" href={`#/${admin ? 'admin/orders' : 'orders'}`}>
        ← Danh sách đơn hàng
      </a>
      <header>
        <h1 className="break-all text-2xl font-medium">Đơn hàng {o.id}</h1>
        <p className="mt-3">
          {statusNames[o.status]} · {new Date(o.created_at).toLocaleString('vi-VN')}
        </p>
      </header>
      {error && <Failure message={error} />}
      {notice && <Notice>{notice}</Notice>}
      {pending && (
        <div className="panel space-y-3">
          <p>Chưa xác định kết quả thao tác trước. Thử lại cùng yêu cầu để tránh tạo trùng.</p>
          <button
            className="button-primary"
            disabled={busy}
            onClick={() => void action(pending.method, pending.path, pending.body)}
          >
            Kiểm tra thao tác trước
          </button>
        </div>
      )}
      <div className="grid gap-6 lg:grid-cols-2">
        <section className="panel space-y-4">
          <h2 className="text-xl font-medium">Sản phẩm đã đặt</h2>
          {o.items.map((item) => (
            <div className="border-b border-stone-100 pb-3" key={item.product_id}>
              <strong>{item.name}</strong>
              <p className="mt-2 text-sm text-stone-500">
                {item.sku} · {item.quantity} × {money(item.unit_price_minor)}
              </p>
              <p className="mt-2">{money(item.line_total_minor)}</p>
            </div>
          ))}
          {o.coupon_snapshot && (
            <p>
              Coupon {o.coupon_snapshot.code}: -{money(o.discount_minor)}
            </p>
          )}
          <p>Phí giao hàng: {money(o.shipping_minor)}</p>
          <p className="text-xl font-semibold">Tổng: {money(o.total_minor)}</p>
        </section>
        <section className="panel space-y-4">
          <h2 className="text-xl font-medium">Giao hàng & thanh toán</h2>
          <p>
            {o.shipping_address.recipient} · {o.shipping_address.phone}
          </p>
          <p className="break-words">{o.shipping_address.address}</p>
          <p>
            Phương thức:{' '}
            {o.payment.method === 'COD' ? 'Thanh toán khi nhận hàng' : 'Thanh toán mô phỏng'}
          </p>
          <div>
            Thanh toán:{' '}
            <Status value={o.payment.status === 'Pending' ? 'PaymentPending' : o.payment.status} />
          </div>
          <p className="text-sm text-stone-500">
            Thông tin đặt hàng là bản chụp tại thời điểm tạo đơn.
          </p>
          {!admin &&
            ['Pending', 'Confirmed'].includes(o.status) &&
            o.payment.status === 'Pending' && (
              <button
                className="button-secondary"
                disabled={busy || !!pending}
                onClick={() => {
                  if (window.confirm('Hủy đơn và hoàn lại tồn kho?'))
                    void action('POST', `/api/v1/orders/${o.id}/cancel`)
                }}
              >
                Hủy đơn hàng
              </button>
            )}
          {!admin &&
            o.payment.method === 'Mock' &&
            o.mock_payments_enabled &&
            o.status === 'Pending' && (
              <div className="space-y-3 border-t pt-4">
                <p className="text-sm">
                  Mô phỏng thanh toán local, không chuyển tiền thật. Tối đa 5 lần thử.
                </p>
                {pendingAttempt ? (
                  <div className="flex flex-wrap gap-3">
                    <button
                      className="button-primary"
                      disabled={busy || !!pending}
                      onClick={() =>
                        void action(
                          'POST',
                          `/api/v1/payment-attempts/${pendingAttempt.id}/simulate`,
                          { result: 'Success' },
                        )
                      }
                    >
                      Mô phỏng thành công
                    </button>
                    <button
                      className="button-secondary"
                      disabled={busy || !!pending}
                      onClick={() =>
                        void action(
                          'POST',
                          `/api/v1/payment-attempts/${pendingAttempt.id}/simulate`,
                          { result: 'Failure', failure_code: 'MOCK_DECLINED' },
                        )
                      }
                    >
                      Mô phỏng thất bại
                    </button>
                  </div>
                ) : (
                  <button
                    className="button-primary"
                    disabled={busy || !!pending || o.payment.attempts.length >= 5}
                    onClick={() => void action('POST', `/api/v1/orders/${o.id}/payment-attempts`)}
                  >
                    Tạo lần thanh toán mô phỏng
                  </button>
                )}
              </div>
            )}
          {o.payment.attempts.length > 0 && (
            <ul className="space-y-2" aria-label="Các lần thanh toán">
              {o.payment.attempts.map((a: Attempt, index) => (
                <li key={a.id} className="text-sm">
                  Lần {index + 1}:{' '}
                  <Status value={a.status === 'Pending' ? 'PaymentPending' : a.status} />
                  {a.failure_code && ` · ${a.failure_code}`}
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
      {admin && transitions.length > 0 && (
        <form
          className="panel flex flex-wrap items-end gap-4"
          onSubmit={(e) => {
            e.preventDefault()
            const form = new FormData(e.currentTarget)
            void action('PATCH', `/api/v1/admin/orders/${o.id}/status`, {
              status: String(form.get('status')),
              reason: String(form.get('reason')),
            })
          }}
        >
          <Field label="Trạng thái đơn tiếp theo">
            <select name="status" className="input" disabled={busy || !!pending}>
              {transitions.map((s) => (
                <option value={s} key={s}>
                  {statusNames[s]}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Lý do cập nhật">
            <input name="reason" className="input" maxLength={500} disabled={busy || !!pending} />
          </Field>
          <button className="button-primary" disabled={busy || !!pending}>
            Cập nhật trạng thái đơn
          </button>
        </form>
      )}
      <section className="panel space-y-4">
        <h2 className="text-xl font-medium">Lịch sử đơn hàng</h2>
        <ol className="space-y-3">
          {o.history.map((h) => (
            <li key={h.id} className="border-l-2 border-[#c6d3bd] pl-4 text-sm">
              <p>
                {h.from_status ? `${statusNames[h.from_status]} → ` : ''}
                {statusNames[h.to_status]}
              </p>
              <p className="mt-1 text-stone-500">
                {new Date(h.created_at).toLocaleString('vi-VN')}
                {h.reason && ` · ${h.reason}`}
              </p>
            </li>
          ))}
        </ol>
      </section>
    </section>
  )
}
