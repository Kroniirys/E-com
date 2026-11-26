import { useState } from 'react'
import { ApiError, errorMessage, money } from '../api'
import type { components } from '../api.generated'
import type { Address, Page } from '../api'
import { useSession } from '../session'
import { navigate, useLoad } from '../hooks'
import { performIntent, readIntent } from '../intent'
import { Failure, Field, Loading } from '../ui'

type Cart = components['schemas']['Cart']
type Order = components['schemas']['Order']
export default function CartPage() {
  const { api, user } = useSession()
  const state = useLoad<Cart>('/api/v1/cart')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [pending, setPending] = useState(() => readIntent(user!.id, 'checkout'))
  const mutate = async (method: string, path: string, body?: unknown) => {
    setBusy(true)
    setError('')
    try {
      await api.request(method, path, body)
      state.reload()
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  const checkout = async (body: unknown) => {
    setBusy(true)
    setError('')
    try {
      const order = await performIntent<Order>(
        api,
        user!.id,
        'checkout',
        'POST',
        '/api/v1/orders',
        body,
      )
      navigate(`/orders/${order.id}`)
    } catch (e) {
      setError(errorMessage(e))
      if (
        e instanceof ApiError &&
        [
          'QUOTE_CHANGED',
          'CART_CHANGED',
          'PRODUCT_UNAVAILABLE',
          'INSUFFICIENT_STOCK',
          'COUPON_UNAVAILABLE',
        ].includes(e.code)
      )
        state.reload()
    } finally {
      setPending(readIntent(user!.id, 'checkout'))
      setBusy(false)
    }
  }
  return (
    <main className="page space-y-6">
      <div>
        <p className="eyebrow">Mua sắm</p>
        <h1 className="mt-3 text-3xl font-medium">Giỏ hàng</h1>
        <p className="mt-3 text-stone-600">
          Giỏ hàng chưa giữ tồn kho. Giá và số lượng được kiểm tra lại khi đặt hàng.
        </p>
      </div>
      {error && <Failure message={error} />}
      {pending && (
        <section className="panel space-y-4">
          <p>
            Có yêu cầu đặt hàng chưa nhận được kết quả. Kiểm tra lại yêu cầu cũ trước khi thay đổi
            giỏ hoặc địa chỉ.
          </p>
          <button
            className="button-primary"
            disabled={busy}
            onClick={() => void checkout(pending.body)}
          >
            Kiểm tra yêu cầu đặt hàng trước
          </button>
        </section>
      )}
      {state.loading ? (
        <Loading />
      ) : state.error ? (
        <Failure message={state.error} retry={state.reload} />
      ) : (
        state.data && (
          <>
            {state.data.items.length === 0 ? (
              <section className="panel">
                <p>Giỏ hàng đang trống.</p>
                <a className="mt-4 inline-block underline" href="#/">
                  Tiếp tục mua sắm
                </a>
              </section>
            ) : (
              <>
                <div className="space-y-3">
                  {state.data.items.map((item) => (
                    <article
                      className="panel flex flex-wrap items-center justify-between gap-4"
                      key={item.id}
                    >
                      <div>
                        <a
                          className="font-semibold underline"
                          href={`#/products/${item.product_id}`}
                        >
                          {item.name}
                        </a>
                        <p className="mt-2 text-sm text-stone-500">
                          {money(item.unit_price_minor)} / sản phẩm · Tồn kho {item.stock}
                        </p>
                        {!item.available && (
                          <p className="mt-2 text-red-700">
                            Không đủ hàng hoặc sản phẩm đã ngừng bán. Vui lòng sửa hoặc xóa.
                          </p>
                        )}
                      </div>
                      <form
                        className="flex flex-wrap items-end gap-3"
                        onSubmit={(e) => {
                          e.preventDefault()
                          const quantity = Number(new FormData(e.currentTarget).get('quantity'))
                          void mutate('PUT', `/api/v1/cart/items/${item.id}`, { quantity })
                        }}
                      >
                        <Field label={`Số lượng ${item.name}`}>
                          <input
                            className="input !w-24"
                            key={`${item.id}-${item.quantity}`}
                            name="quantity"
                            type="number"
                            min="1"
                            max="1000"
                            required
                            defaultValue={item.quantity}
                            disabled={busy || !!pending}
                          />
                        </Field>
                        <button className="button-secondary" disabled={busy || !!pending}>
                          Cập nhật
                        </button>
                        <button
                          type="button"
                          className="button-secondary"
                          disabled={busy || !!pending}
                          onClick={() => void mutate('DELETE', `/api/v1/cart/items/${item.id}`)}
                        >
                          Xóa
                        </button>
                      </form>
                      <strong>{money(item.line_total_minor)}</strong>
                    </article>
                  ))}
                </div>
                <button
                  className="button-secondary"
                  disabled={busy || !!pending}
                  onClick={() => {
                    if (window.confirm('Xóa toàn bộ giỏ hàng?'))
                      void mutate('DELETE', '/api/v1/cart')
                  }}
                >
                  Xóa toàn bộ giỏ
                </button>
                <Checkout
                  key={state.data.quote_hash}
                  cart={state.data}
                  busy={busy || !!pending}
                  checkout={checkout}
                  reload={state.reload}
                />
              </>
            )}
          </>
        )
      )}
    </main>
  )
}
function Checkout({
  reload,
  cart,
  busy,
  checkout,
}: {
  reload: () => void
  cart: Cart
  busy: boolean
  checkout: (body: unknown) => Promise<void>
}) {
  const addresses = useLoad<Page<Address>>('/api/v1/me/addresses?page_size=100')
  const [address, setAddress] = useState('')
  const [method, setMethod] = useState('COD')
  const [confirmed, setConfirmed] = useState(false)
  const { api } = useSession()
  const [code, setCode] = useState('')
  const [promo, setPromo] = useState<components['schemas']['CouponPreview'] | null>(null)
  const [promoError, setPromoError] = useState('')
  const [applying, setApplying] = useState(false)
  const apply = async () => {
    setApplying(true)
    setPromoError('')
    setConfirmed(false)
    try {
      const value = await api.request<components['schemas']['CouponPreview']>(
        'POST',
        '/api/v1/coupons/validate',
        { code, cart_version: cart.version },
      )
      if (value.cart_quote_hash !== cart.quote_hash) {
        reload()
        return
      }
      setPromo(value)
    } catch (e) {
      setPromo(null)
      setPromoError(errorMessage(e))
    } finally {
      setApplying(false)
    }
  }

  const selected =
    address ||
    addresses.data?.items.find((a) => a.is_default)?.id ||
    addresses.data?.items[0]?.id ||
    ''
  return (
    <section className="panel grid gap-7 lg:grid-cols-2" aria-label="Xác nhận đơn hàng">
      <div className="space-y-4">
        <h2 className="text-xl font-medium">Giao hàng & thanh toán</h2>
        {addresses.loading ? (
          <Loading />
        ) : addresses.error ? (
          <Failure message={addresses.error} retry={addresses.reload} />
        ) : !addresses.data?.items.length ? (
          <p>
            Thêm địa chỉ trong{' '}
            <a href="#/account" className="underline">
              Tài khoản
            </a>{' '}
            trước khi đặt hàng.
          </p>
        ) : (
          <Field label="Địa chỉ giao hàng">
            <select
              className="input"
              value={selected}
              onChange={(e) => setAddress(e.target.value)}
              disabled={busy}
            >
              {addresses.data.items.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.recipient} · {a.address}
                </option>
              ))}
            </select>
          </Field>
        )}
        <Field label="Phương thức thanh toán">
          <select
            className="input"
            value={method}
            onChange={(e) => setMethod(e.target.value)}
            disabled={busy}
          >
            <option value="COD">Thanh toán khi nhận hàng (COD)</option>
            {cart.mock_payments_enabled && (
              <option value="Mock">Thanh toán mô phỏng (không tiền thật)</option>
            )}
          </select>
        </Field>
        <div className="space-y-3">
          <Field label="Mã giảm giá">
            <input
              className="input"
              value={code}
              maxLength={64}
              disabled={busy || applying}
              onChange={(e) => {
                setCode(e.target.value)
                setPromo(null)
                setConfirmed(false)
              }}
            />
          </Field>
          <div className="flex gap-3">
            <button
              className="button-secondary"
              type="button"
              disabled={busy || applying || !code.trim()}
              onClick={() => void apply()}
            >
              Áp dụng coupon
            </button>
            {promo && (
              <button
                className="button-secondary"
                type="button"
                disabled={busy}
                onClick={() => {
                  setPromo(null)
                  setCode('')
                  setConfirmed(false)
                }}
              >
                Bỏ coupon
              </button>
            )}
          </div>
          {promo && <p role="status">Đã áp dụng {promo.code}</p>}
          {promoError && <Failure message={promoError} />}
        </div>
        <p className="text-sm text-stone-500">
          COD được ghi nhận đã thanh toán khi đơn được giao thành công.
        </p>
      </div>
      <div className="space-y-4">
        <dl className="space-y-3">
          <div className="flex justify-between">
            <dt>Tạm tính</dt>
            <dd>{money(cart.subtotal_minor)}</dd>
          </div>
          <div className="flex justify-between">
            <dt>Phí giao hàng</dt>
            <dd>{money(cart.shipping_minor)}</dd>
          </div>
          {promo && (
            <div className="flex justify-between">
              <dt>Giảm giá</dt>
              <dd>-{money(promo.discount_minor)}</dd>
            </div>
          )}
          <div className="flex justify-between border-t pt-3 text-xl font-semibold">
            <dt>Tổng thanh toán</dt>
            <dd>{money(promo?.total_minor ?? cart.total_minor)}</dd>
          </div>
        </dl>
        {!cart.pricing_valid && (
          <Failure message="Tổng tiền vượt giới hạn. Giảm số lượng hoặc xóa sản phẩm để tiếp tục." />
        )}
        <label className="flex items-start gap-3 text-sm">
          <input
            type="checkbox"
            checked={confirmed}
            onChange={(e) => setConfirmed(e.target.checked)}
            disabled={busy}
          />
          Tôi xác nhận giá, địa chỉ và phương thức thanh toán hiện tại.
        </label>
        <button
          className="button-primary w-full"
          disabled={
            busy ||
            applying ||
            !confirmed ||
            !selected ||
            !cart.pricing_valid ||
            cart.items.some((i) => !i.available)
          }
          onClick={() =>
            void checkout({
              address_id: selected,
              cart_version: cart.version,
              quote_hash: promo?.quote_hash ?? cart.quote_hash,
              ...(promo ? { coupon_code: promo.code } : {}),
              payment_method: method,
            })
          }
        >
          {busy ? 'Đang kiểm tra…' : 'Đặt hàng'}
        </button>
      </div>
    </section>
  )
}
