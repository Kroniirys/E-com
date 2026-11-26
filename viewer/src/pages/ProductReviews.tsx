import { useState } from 'react'
import type { components } from '../api.generated'
import { errorMessage } from '../api'
import { useSession } from '../session'
import { useLoad } from '../hooks'
import { Failure, Field, Loading, Pagination } from '../ui'
type ReviewPage = components['schemas']['ReviewPage']
type Eligibility = components['schemas']['ReviewEligibility']
export default function ProductReviews({ id }: { id: string }) {
  const [page, setPage] = useState(1)
  const state = useLoad<ReviewPage>(`/api/v1/products/${id}/reviews?page=${page}`, false)
  const { user } = useSession()
  return (
    <section className="panel mt-8 space-y-5">
      <h2 className="text-2xl font-medium">Đánh giá sản phẩm</h2>
      {state.loading ? (
        <Loading />
      ) : state.error ? (
        <Failure message={state.error} retry={state.reload} />
      ) : (
        state.data && (
          <>
            <p>
              {state.data.summary.average_rating.toFixed(2)} / 5 · {state.data.summary.count} đánh
              giá
            </p>
            {state.data.items.map((r) => (
              <article className="border-b py-4" key={r.id}>
                <strong>{r.rating} / 5 · Khách đã mua hàng</strong>
                <p className="mt-2 whitespace-pre-wrap break-words">{r.content}</p>
              </article>
            ))}
            <Pagination
              page={page}
              pageSize={state.data.page_size}
              total={state.data.total}
              change={setPage}
            />
          </>
        )
      )}
      {user ? (
        <OwnReview key={user.id} id={id} changed={state.reload} />
      ) : (
        <a href="#/login" className="underline">
          Đăng nhập để đánh giá sản phẩm đã mua
        </a>
      )}
    </section>
  )
}
function OwnReview({ id, changed }: { id: string; changed: () => void }) {
  const { api } = useSession()
  const state = useLoad<Eligibility>(`/api/v1/products/${id}/review-eligibility`)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const run = async (method: string, path: string, body?: unknown) => {
    setBusy(true)
    setError('')
    try {
      await api.request(method, path, body)
      state.reload()
      changed()
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="space-y-4 border-t pt-5">
      {error && <Failure message={error} />}{' '}
      {state.loading ? (
        <Loading />
      ) : state.error ? (
        <Failure message={state.error} retry={state.reload} />
      ) : state.data?.review ? (
        <>
          <p>
            {state.data.review.deleted
              ? 'Bạn đã xóa đánh giá; mỗi khách chỉ được đánh giá sản phẩm một lần.'
              : `Đánh giá của bạn: ${state.data.review.visibility === 'Hidden' ? 'Đang bị ẩn' : 'Đang hiển thị'}`}
          </p>
          {!state.data.review.deleted && (
            <>
              <p className="whitespace-pre-wrap break-words">{state.data.review.content}</p>
              <button
                className="button-secondary"
                disabled={busy}
                onClick={() => {
                  if (window.confirm('Xóa đánh giá của bạn?'))
                    void run('DELETE', `/api/v1/reviews/${state.data!.review!.id}`)
                }}
              >
                Xóa đánh giá của tôi
              </button>
            </>
          )}
        </>
      ) : state.data?.can_review ? (
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault()
            const form = new FormData(e.currentTarget)
            void run('POST', `/api/v1/products/${id}/reviews`, {
              rating: Number(form.get('rating')),
              content: String(form.get('content')),
            })
          }}
        >
          <Field label="Số sao">
            <select name="rating" className="input" disabled={busy}>
              {[5, 4, 3, 2, 1].map((n) => (
                <option key={n} value={n}>
                  {n} sao
                </option>
              ))}
            </select>
          </Field>
          <Field label="Nội dung đánh giá">
            <textarea className="input" name="content" required maxLength={2000} disabled={busy} />
          </Field>
          <button className="button-primary" disabled={busy}>
            Gửi đánh giá
          </button>
        </form>
      ) : (
        <p>Đơn hàng chứa sản phẩm cần được giao thành công trước khi đánh giá.</p>
      )}
    </div>
  )
}

export function MyReviews() {
  const { api } = useSession()
  const [page, setPage] = useState(1)
  const state = useLoad<{
    items: components['schemas']['Review'][]
    total: number
    page_size: number
  }>(`/api/v1/me/reviews?page=${page}`)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  return (
    <section className="panel space-y-4">
      <h2 className="text-xl font-medium">Đánh giá của tôi</h2>
      {error && <Failure message={error} />}{' '}
      {state.loading ? (
        <Loading />
      ) : state.error ? (
        <Failure message={state.error} retry={state.reload} />
      ) : (
        state.data && (
          <>
            {state.data.items.length === 0 && <p>Bạn chưa có đánh giá.</p>}
            {state.data.items.map((r) => (
              <article key={r.id} className="space-y-3 border-b pb-4">
                <p>
                  {r.rating}/5 · {r.visibility === 'Hidden' ? 'Đang bị ẩn' : 'Đang hiển thị'}
                </p>
                <p className="break-words whitespace-pre-wrap">{r.content}</p>
                <button
                  className="button-secondary"
                  disabled={busy}
                  onClick={async () => {
                    if (!window.confirm('Xóa đánh giá của bạn?')) return
                    setBusy(true)
                    setError('')
                    try {
                      await api.request('DELETE', `/api/v1/reviews/${r.id}`)
                      state.reload()
                    } catch (e) {
                      setError(errorMessage(e))
                    } finally {
                      setBusy(false)
                    }
                  }}
                >
                  Xóa đánh giá
                </button>
              </article>
            ))}
            <Pagination
              page={page}
              pageSize={state.data.page_size}
              total={state.data.total}
              change={setPage}
            />
          </>
        )
      )}
    </section>
  )
}
