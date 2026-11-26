import { useEffect, useState } from 'react'
import { getFoundation } from '../client'
import type { Foundation } from '../client'

type State =
  | { status: 'loading' }
  | { status: 'ready'; data: Foundation }
  | { status: 'error'; message: string }

export default function FoundationPage() {
  const [state, setState] = useState<State>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)
  useEffect(() => {
    const controller = new AbortController()
    getFoundation(controller.signal).then(
      (data) => {
        if (!controller.signal.aborted) setState({ status: 'ready', data })
      },
      (error: unknown) => {
        if (!controller.signal.aborted)
          setState({
            status: 'error',
            message: error instanceof Error ? error.message : 'Kết nối bị gián đoạn.',
          })
      },
    )
    return () => controller.abort()
  }, [attempt])
  const retry = () => {
    setState({ status: 'loading' })
    setAttempt((value) => value + 1)
  }
  const ready = state.status === 'ready'
  return (
    <div className="min-h-screen bg-[#f7f8f2] text-[#183c32]">
      <main className="mx-auto max-w-6xl px-6 pb-12 pt-12 lg:px-10 lg:pt-20">
        <div className="mb-6 flex items-center gap-3 text-xs font-semibold uppercase tracking-[0.18em]">
          <span className="size-2 rounded-full bg-[#769a43]" aria-hidden="true" /> Phase 01{' '}
          <span className="text-[#9ba797]">/</span> Thiết kế & nền tảng
        </div>
        <div className="grid gap-10 lg:grid-cols-[1.35fr_1fr] lg:gap-16">
          <section>
            <h1 className="max-w-2xl text-4xl font-medium leading-[1.12] tracking-[-0.045em] sm:text-6xl">
              Một khởi đầu tốt.
              <br />
              <span className="text-[#789665]">Một nền tảng vững.</span>
            </h1>
            <p className="mt-6 max-w-lg text-base leading-7 text-[#68796f]">
              Không gian khởi đầu cho cửa hàng của bạn. Theo dõi kết nối và cấu hình trước khi mở
              rộng các chức năng bán hàng.
            </p>
            <div className="mt-8 inline-flex items-center gap-2 rounded-full border border-[#cbd5c3] bg-white/60 px-4 py-2 text-xs text-[#68796f]">
              <span aria-hidden="true">◎</span> Dữ liệu trực tiếp từ hệ thống
            </div>
          </section>
          <section
            id="status"
            aria-labelledby="status-title"
            className="rounded-3xl border border-[#d5decc] bg-white p-7 shadow-[0_12px_36px_-24px_#183c3233]"
          >
            <div className="mb-7 flex items-center justify-between gap-3">
              <h2 id="status-title" className="font-medium">
                Kết nối cửa hàng
              </h2>
              <span
                className={`rounded-full px-3 py-1 text-xs font-medium ${ready ? 'bg-[#e9f2d9] text-[#42692e]' : state.status === 'error' ? 'bg-red-50 text-red-700' : 'bg-stone-100 text-stone-600'}`}
                role="status"
              >
                {ready ? 'Đã kết nối' : state.status === 'error' ? 'Chưa kết nối' : 'Đang kết nối…'}
              </span>
            </div>
            {state.status === 'loading' && (
              <div
                className="min-h-40 animate-pulse space-y-4"
                aria-label="Đang tải cấu hình cửa hàng"
              >
                <div className="h-5 w-2/3 rounded bg-stone-100" />
                <div className="h-4 rounded bg-stone-100" />
                <div className="h-4 w-3/4 rounded bg-stone-100" />
              </div>
            )}
            {state.status === 'error' && (
              <div role="alert" className="min-h-40">
                <p className="text-lg font-medium">Chưa thể tải cấu hình</p>
                <p className="mt-3 text-sm leading-6 text-[#68796f]">{state.message}</p>
              </div>
            )}
            {state.status === 'ready' && (
              <dl className="space-y-4 text-sm">
                <div className="flex justify-between gap-4">
                  <dt className="text-[#68796f]">Tên cửa hàng</dt>
                  <dd className="text-right font-medium" data-testid="store-name">
                    {state.data.store.name}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-[#68796f]">Tiền tệ</dt>
                  <dd>{state.data.store.currency}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-[#68796f]">Phí vận chuyển</dt>
                  <dd>
                    {new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(
                      state.data.store.shipping_fee_minor,
                    )}
                  </dd>
                </div>
                <div className="flex justify-between border-t border-[#e8ece2] pt-4">
                  <dt className="text-[#68796f]">Cơ sở dữ liệu</dt>
                  <dd className="flex items-center gap-2">
                    <span aria-hidden="true" className="size-1.5 rounded-full bg-[#769a43]" /> Đang
                    hoạt động
                  </dd>
                </div>
              </dl>
            )}
            <button
              onClick={retry}
              disabled={state.status === 'loading'}
              className="mt-7 w-full rounded-xl bg-[#183c32] px-4 py-3 text-sm font-medium text-white transition hover:bg-[#285445] disabled:cursor-wait disabled:opacity-50"
            >
              {state.status === 'loading' ? 'Đang kiểm tra…' : 'Kiểm tra lại kết nối'}
            </button>
          </section>
        </div>
        <section aria-label="Phạm vi nền tảng" className="mt-14 grid gap-4 sm:grid-cols-3 lg:mt-20">
          {[
            {
              n: '01',
              title: 'Kết nối rõ ràng',
              body: 'Trạng thái được lấy từ API và cơ sở dữ liệu, không dùng dữ liệu mẫu.',
            },
            {
              n: '02',
              title: 'Cấu hình nhất quán',
              body: 'Thông tin cửa hàng được lưu bền vững và đọc từ một nguồn chuẩn.',
            },
            {
              n: '03',
              title: 'Sẵn sàng mở rộng',
              body: 'Tài khoản, hàng hóa và mua hàng sẽ được triển khai trong các phase tiếp theo.',
            },
          ].map((item) => (
            <article key={item.n} className="rounded-2xl border border-[#dae1d4] p-6">
              <span className="text-xs text-[#68794f]">{item.n}</span>
              <h2 className="mt-4 font-medium">{item.title}</h2>
              <p className="mt-2 text-sm leading-6 text-[#68796f]">{item.body}</p>
            </article>
          ))}
        </section>
        <footer className="mt-10 flex flex-wrap justify-between gap-3 text-xs text-[#68796f]">
          <p>E-commerce Base · Nền tảng dự án</p>
          <p>Phase 01 / Chưa mở chức năng bán hàng</p>
        </footer>
      </main>
    </div>
  )
}
