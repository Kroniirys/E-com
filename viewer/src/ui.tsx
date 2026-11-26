import { Children, cloneElement, useId } from 'react'
import type { ReactElement, ReactNode } from 'react'
import type { Page } from './api'

export function Loading() {
  return (
    <div className="rounded-2xl border border-stone-200 bg-white p-8 text-stone-500" role="status">
      Đang tải dữ liệu…
    </div>
  )
}
export function Failure({ message, retry }: { message: string; retry?: () => void }) {
  return (
    <div
      role="alert"
      className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800"
    >
      <p>{message}</p>
      {retry && (
        <button type="button" className="mt-3 underline" onClick={retry}>
          Thử lại
        </button>
      )}
    </div>
  )
}
export function Notice({ children }: { children: ReactNode }) {
  return (
    <p
      role="status"
      className="rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900"
    >
      {children}
    </p>
  )
}
export function Pagination({
  page,
  pageSize,
  total,
  change,
}: {
  page: number
  pageSize: number
  total: number
  change: (page: number) => void
}) {
  const pages = Math.max(1, Math.ceil(total / pageSize))
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 py-5 text-sm text-stone-600">
      <span>
        {total} kết quả · Trang {page}/{pages}
      </span>
      <div className="flex gap-2">
        <button className="button-secondary" disabled={page <= 1} onClick={() => change(page - 1)}>
          Trang trước
        </button>
        <button
          className="button-secondary"
          disabled={page >= pages}
          onClick={() => change(page + 1)}
        >
          Trang sau
        </button>
      </div>
    </div>
  )
}
export function ListState<T>({
  state,
  children,
}: {
  state: { loading: boolean; error?: string; data?: Page<T>; reload: () => void }
  children: (data: Page<T>) => ReactNode
}) {
  if (state.loading) return <Loading />
  if (state.error) return <Failure message={state.error} retry={state.reload} />
  if (!state.data) return null
  return children(state.data)
}
export function Field({
  label,
  children,
  hint,
}: {
  label: string
  children: ReactNode
  hint?: string
}) {
  const generated = useId()
  const control = Children.only(children) as ReactElement<{
    id?: string
    'aria-describedby'?: string
  }>
  const id = control.props.id ?? generated
  const description =
    [control.props['aria-describedby'], hint ? `${id}-hint` : undefined]
      .filter(Boolean)
      .join(' ') || undefined
  return (
    <div className="grid gap-2 text-sm font-medium text-[#365347]">
      <label htmlFor={id}>{label}</label>
      {cloneElement(control, { id, 'aria-describedby': description })}
      {hint && (
        <span id={`${id}-hint`} className="text-xs font-normal text-stone-500">
          {hint}
        </span>
      )}
    </div>
  )
}
export function Status({ value }: { value: string }) {
  const labels: Record<string, string> = {
    Active: 'Đang hoạt động',
    Disabled: 'Đã khóa',
    Draft: 'Bản nháp',
    Inactive: 'Ngừng bán',
    Admin: 'Quản trị viên',
    Customer: 'Khách hàng',
    PaymentPending: 'Chưa thanh toán',
    Paid: 'Đã thanh toán',
    Failed: 'Thất bại',
    Cancelled: 'Đã hủy',
  }
  return (
    <span
      className={`inline-flex rounded-full px-2.5 py-1 text-xs ${value === 'Active' ? 'bg-emerald-50 text-emerald-800' : 'bg-stone-100 text-stone-600'}`}
    >
      {labels[value] ?? value}
    </span>
  )
}
