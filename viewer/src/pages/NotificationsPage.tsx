import { useState } from 'react'
import type { components } from '../api.generated'
import type { Page } from '../api'
import { errorMessage } from '../api'
import { useLoad } from '../hooks'
import { useSession } from '../session'
import { Failure, ListState, Pagination } from '../ui'
type Notification = components['schemas']['Notification']
export default function NotificationsPage() {
  const { api } = useSession()
  const [page, setPage] = useState(1)
  const state = useLoad<Page<Notification> & { unread_count: number }>(
    `/api/v1/notifications?page=${page}`,
  )
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  return (
    <main className="page space-y-5">
      <div className="flex flex-wrap justify-between gap-3">
        <h1 className="text-3xl font-medium">Thông báo</h1>
        <button className="button-secondary" onClick={state.reload}>
          Làm mới thông báo
        </button>
      </div>
      <p className="text-sm text-stone-500">
        Thông báo xuất hiện sau khi worker xử lý sự kiện. Dùng làm mới để kiểm tra cập nhật.
      </p>
      {error && <Failure message={error} />}
      <ListState state={state}>
        {(data) => (
          <>
            <p>{state.data?.unread_count ?? 0} thông báo chưa đọc</p>
            {data.items.length === 0 && <p className="panel">Chưa có thông báo.</p>}
            {data.items.map((n) => (
              <article
                className={`panel space-y-3 ${n.read_at ? '' : 'border-l-4 border-l-emerald-700'}`}
                key={n.id}
              >
                <h2 className="text-lg font-semibold">{n.title}</h2>
                <p className="break-words">{n.body}</p>
                <p className="text-sm text-stone-500">
                  {new Date(n.occurred_at).toLocaleString('vi-VN')}
                </p>
                <div className="flex gap-4">
                  <a className="underline" href={`#/orders/${n.order_id}`}>
                    Xem đơn hàng
                  </a>
                  {!n.read_at && (
                    <button
                      className="underline"
                      disabled={busy}
                      onClick={async () => {
                        setBusy(true)
                        setError('')
                        try {
                          await api.request('POST', `/api/v1/notifications/${n.id}/read`)
                          state.reload()
                        } catch (e) {
                          setError(errorMessage(e))
                        } finally {
                          setBusy(false)
                        }
                      }}
                    >
                      Đánh dấu đã đọc
                    </button>
                  )}
                </div>
              </article>
            ))}
            <Pagination page={page} pageSize={data.page_size} total={data.total} change={setPage} />
          </>
        )}
      </ListState>
    </main>
  )
}
