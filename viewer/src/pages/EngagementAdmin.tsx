import { useState } from 'react'
import { errorMessage } from '../api'
import type { Page } from '../api'
import type { components } from '../api.generated'
import { useSession } from '../session'
import { useLoad } from '../hooks'
import { Failure, Field, ListState, Loading, Pagination } from '../ui'
type Coupon = components['schemas']['Coupon']
type AdminReview = components['schemas']['AdminReview']
type Dashboard = components['schemas']['Dashboard']
export default function EngagementAdmin({ tab }: { tab: string }) {
  return tab === 'dashboard' ? (
    <DashboardPanel />
  ) : tab === 'coupons' ? (
    <CouponsPanel />
  ) : (
    <ReviewsPanel />
  )
}
function DashboardPanel() {
  const [query, setQuery] = useState('')
  const state = useLoad<Dashboard>(`/api/v1/admin/dashboard${query}`)
  return (
    <section className="space-y-5">
      <h2 className="text-2xl">Tổng quan cửa hàng</h2>
      <form
        className="panel flex flex-wrap items-end gap-4"
        onSubmit={(e) => {
          e.preventDefault()
          const v = new FormData(e.currentTarget)
          const p = new URLSearchParams()
          for (const key of ['from', 'to', 'timezone'])
            if (v.get(key)) p.set(key, String(v.get(key)))
          setQuery('?' + p)
        }}
      >
        <Field label="Từ ngày">
          <input className="input" name="from" type="date" />
        </Field>
        <Field label="Đến ngày (không bao gồm)">
          <input className="input" name="to" type="date" />
        </Field>
        <Field label="Múi giờ IANA">
          <input className="input" name="timezone" defaultValue="Asia/Ho_Chi_Minh" />
        </Field>
        <button className="button-primary">Áp dụng khoảng ngày</button>
      </form>
      <p className="text-sm text-stone-500">
        Lọc theo ngày tạo. Tổng tiền chỉ gồm đơn đã giao và đã thanh toán; người dùng/sản phẩm không
        gồm bản ghi đã xóa.
      </p>
      {state.loading ? (
        <Loading />
      ) : state.error ? (
        <Failure message={state.error} retry={state.reload} />
      ) : (
        state.data && (
          <div className="grid gap-4 md:grid-cols-2">
            {[
              ['Người dùng', state.data.user_count],
              ['Sản phẩm', state.data.product_count],
              ['Đơn hàng', state.data.order_count],
              [
                'Tổng tiền đơn đã giao và đã thanh toán',
                new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(
                  BigInt(state.data.delivered_paid_total_minor),
                ),
              ],
            ].map(([label, value]) => (
              <div className="panel" key={String(label)}>
                <p>{label}</p>
                <strong className="mt-3 block break-words text-2xl">{value}</strong>
              </div>
            ))}
          </div>
        )
      )}
    </section>
  )
}
function CouponsPanel() {
  const { api } = useSession()
  const [page, setPage] = useState(1)
  const state = useLoad<Page<Coupon>>(`/api/v1/admin/coupons?page=${page}`)
  const [edit, setEdit] = useState<Coupon | null | undefined>()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const save = async (form: FormData) => {
    setBusy(true)
    setError('')
    try {
      await api.request(
        edit ? 'PUT' : 'POST',
        `/api/v1/admin/coupons${edit ? '/' + edit.id : ''}`,
        {
          code: String(form.get('code')),
          kind: String(form.get('kind')),
          value: Number(form.get('value')),
          minimum_minor: Number(form.get('minimum')),
          expires_at: String(form.get('expires')) + ':00Z',
          usage_limit: Number(form.get('limit')),
          enabled: form.get('enabled') === 'on',
        },
      )
      setEdit(undefined)
      state.reload()
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <section className="space-y-5">
      <div className="flex justify-between">
        <h2 className="text-2xl">Mã giảm giá</h2>
        <button className="button-primary" onClick={() => setEdit(null)}>
          Tạo coupon
        </button>
      </div>
      {error && <Failure message={error} />}{' '}
      {edit !== undefined && (
        <form
          className="panel grid gap-4 md:grid-cols-2"
          key={edit?.id ?? 'new'}
          onSubmit={(e) => {
            e.preventDefault()
            void save(new FormData(e.currentTarget))
          }}
        >
          <Field label="Mã coupon">
            <input
              className="input"
              name="code"
              required
              maxLength={64}
              defaultValue={edit?.code}
            />
          </Field>
          <Field label="Loại giảm giá">
            <select className="input" name="kind" defaultValue={edit?.kind ?? 'Percentage'}>
              <option value="Percentage">Phần trăm</option>
              <option value="Fixed">Số tiền cố định</option>
            </select>
          </Field>
          <Field label="Giá trị giảm">
            <input
              className="input"
              name="value"
              type="number"
              min={1}
              max={9000000000000}
              required
              defaultValue={edit?.value ?? 10}
            />
          </Field>
          <Field label="Giá trị đơn tối thiểu (VND)">
            <input
              className="input"
              name="minimum"
              type="number"
              min={0}
              max={9000000000000}
              required
              defaultValue={edit?.minimum_minor ?? 0}
            />
          </Field>
          <Field label="Hết hạn (UTC)">
            <input
              className="input"
              name="expires"
              type="datetime-local"
              required
              defaultValue={edit ? new Date(edit.expires_at).toISOString().slice(0, 16) : undefined}
            />
          </Field>
          <Field label="Giới hạn lượt dùng">
            <input
              className="input"
              name="limit"
              type="number"
              min={1}
              max={9000000000000}
              required
              defaultValue={edit?.usage_limit ?? 100}
            />
          </Field>
          <label>
            <input name="enabled" type="checkbox" defaultChecked={edit?.enabled ?? true} /> Cho phép
            sử dụng
          </label>
          <div className="flex gap-3">
            <button className="button-primary" disabled={busy}>
              Lưu coupon
            </button>
            <button type="button" className="button-secondary" onClick={() => setEdit(undefined)}>
              Hủy
            </button>
          </div>
        </form>
      )}
      <ListState state={state}>
        {(data) => (
          <>
            {data.items.map((c) => (
              <div className="panel flex flex-wrap justify-between gap-4" key={c.id}>
                <div>
                  <strong>{c.code}</strong>
                  <p>
                    {c.kind} · {c.value} · Đã dùng {c.used_count}/{c.usage_limit} ·{' '}
                    {c.enabled ? 'Đang bật' : 'Đang tắt'}
                  </p>
                </div>
                <div className="flex gap-3">
                  <button className="button-secondary" onClick={() => setEdit(c)}>
                    Sửa coupon
                  </button>
                  <button
                    className="button-secondary"
                    disabled={busy}
                    onClick={async () => {
                      if (!window.confirm('Lưu trữ coupon này?')) return
                      setBusy(true)
                      try {
                        await api.request('DELETE', `/api/v1/admin/coupons/${c.id}`)
                        state.reload()
                      } catch (e) {
                        setError(errorMessage(e))
                      } finally {
                        setBusy(false)
                      }
                    }}
                  >
                    Lưu trữ coupon
                  </button>
                </div>
              </div>
            ))}
            <Pagination page={page} pageSize={data.page_size} total={data.total} change={setPage} />
          </>
        )}
      </ListState>
    </section>
  )
}
function ReviewsPanel() {
  const { api } = useSession()
  const [page, setPage] = useState(1)
  const [visibility, setVisibility] = useState('')
  const state = useLoad<Page<AdminReview>>(
    `/api/v1/admin/reviews?page=${page}&visibility=${visibility}`,
  )
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  return (
    <section className="space-y-5">
      <h2 className="text-2xl">Kiểm duyệt đánh giá</h2>
      <Field label="Lọc trạng thái đánh giá">
        <select
          className="input"
          value={visibility}
          onChange={(e) => {
            setPage(1)
            setVisibility(e.target.value)
          }}
        >
          <option value="">Tất cả</option>
          <option value="Visible">Đang hiển thị</option>
          <option value="Hidden">Đang ẩn</option>
        </select>
      </Field>
      {error && <Failure message={error} />}
      <ListState state={state}>
        {(data) => (
          <>
            {data.items.map((r) => (
              <article className="panel space-y-4" key={r.id}>
                <a className="underline break-all" href={`#/products/${r.product_id}`}>
                  Sản phẩm {r.product_id}
                </a>
                <p>
                  {r.rating}/5 · {r.visibility === 'Visible' ? 'Đang hiển thị' : 'Đang ẩn'}
                </p>
                <p className="whitespace-pre-wrap break-words">{r.content}</p>
                {r.moderation_reason && <p>Lý do trước: {r.moderation_reason}</p>}
                <form
                  className="flex flex-wrap items-end gap-3"
                  onSubmit={async (e) => {
                    e.preventDefault()
                    const reason = String(new FormData(e.currentTarget).get('reason'))
                    setBusy(true)
                    setError('')
                    try {
                      await api.request('PATCH', `/api/v1/admin/reviews/${r.id}/moderation`, {
                        visibility: r.visibility === 'Visible' ? 'Hidden' : 'Visible',
                        reason,
                      })
                      state.reload()
                    } catch (e) {
                      setError(errorMessage(e))
                    } finally {
                      setBusy(false)
                    }
                  }}
                >
                  <Field label="Lý do kiểm duyệt">
                    <input className="input" name="reason" required maxLength={500} />
                  </Field>
                  <button className="button-primary" disabled={busy}>
                    {r.visibility === 'Visible' ? 'Ẩn đánh giá' : 'Hiện đánh giá'}
                  </button>
                </form>
              </article>
            ))}
            <Pagination page={page} pageSize={data.page_size} total={data.total} change={setPage} />
          </>
        )}
      </ListState>
    </section>
  )
}
