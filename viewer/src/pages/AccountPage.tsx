import { useState } from 'react'
import { MyReviews } from './ProductReviews'
import type { FormEvent } from 'react'
import { errorMessage } from '../api'
import type { Address, Page, User } from '../api'
import { useSession } from '../session'
import { navigate, useLoad } from '../hooks'
import { Failure, Field, ListState, Notice, Pagination } from '../ui'

export default function AccountPage() {
  const session = useSession()
  const [page, setPage] = useState(1)
  const addresses = useLoad<Page<Address>>(`/api/v1/me/addresses?page=${page}&page_size=10`)
  const [editing, setEditing] = useState<Address | null | undefined>(undefined)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const profile = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const values = new FormData(event.currentTarget)
    setBusy(true)
    setError('')
    setMessage('')
    try {
      const user = await session.api.request<User>('PATCH', '/api/v1/me', {
        name: String(values.get('name')),
      })
      session.changedUser(user)
      setMessage('Đã cập nhật thông tin cá nhân.')
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  const password = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const values = new FormData(event.currentTarget)
    const next = String(values.get('new_password'))
    if ([...next].length < 12 || new TextEncoder().encode(next).length > 72) {
      setError('Mật khẩu cần ít nhất 12 ký tự và tối đa 72 byte UTF-8.')
      return
    }
    setBusy(true)
    setError('')
    try {
      await session.api.request<void>('POST', '/api/v1/auth/change-password', {
        current_password: String(values.get('current_password')),
        new_password: next,
      })
      session.passwordChanged()
      navigate('/login')
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  const remove = async (address: Address) => {
    if (!window.confirm(`Xóa địa chỉ của ${address.recipient}?`)) return
    setBusy(true)
    setError('')
    try {
      await session.api.request<void>('DELETE', `/api/v1/me/addresses/${address.id}`)
      addresses.reload()
      setMessage('Đã xóa địa chỉ.')
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <main className="page space-y-7">
      <div>
        <p className="eyebrow">Tài khoản của bạn</p>
        <h1 className="mt-3 text-3xl font-medium">Thông tin cá nhân</h1>
      </div>
      {error && <Failure message={error} />} {message && <Notice>{message}</Notice>}
      <div className="grid gap-6 lg:grid-cols-2">
        <section className="panel">
          <h2 className="mb-5 text-xl font-medium">Hồ sơ</h2>
          <form className="space-y-4" onSubmit={profile}>
            <Field label="Email">
              <input className="input" value={session.user?.email ?? ''} disabled />
            </Field>
            <Field label="Họ và tên">
              <input
                className="input"
                name="name"
                defaultValue={session.user?.name}
                required
                maxLength={120}
              />
            </Field>
            <button className="button-primary" disabled={busy}>
              Lưu hồ sơ
            </button>
          </form>
        </section>
        <section className="panel">
          <h2 className="mb-5 text-xl font-medium">Đổi mật khẩu</h2>
          <form className="space-y-4" onSubmit={password}>
            <Field label="Mật khẩu hiện tại">
              <input
                className="input"
                name="current_password"
                type="password"
                autoComplete="current-password"
                required
                maxLength={72}
              />
            </Field>
            <Field label="Mật khẩu mới" hint="Sau khi đổi, bạn cần đăng nhập lại.">
              <input
                className="input"
                name="new_password"
                type="password"
                autoComplete="new-password"
                required
                minLength={12}
                maxLength={72}
              />
            </Field>
            <button className="button-primary" disabled={busy}>
              Đổi mật khẩu
            </button>
          </form>
        </section>
      </div>
      <section>
        <div className="mb-5 flex items-center justify-between gap-4">
          <h2 className="text-xl font-medium">Địa chỉ nhận hàng</h2>
          <button className="button-primary" onClick={() => setEditing(null)}>
            Thêm địa chỉ
          </button>
        </div>
        {editing !== undefined && (
          <AddressForm
            key={editing?.id ?? 'new'}
            editing={editing}
            cancel={() => setEditing(undefined)}
            saved={() => {
              setEditing(undefined)
              addresses.reload()
              setMessage('Đã lưu địa chỉ nhận hàng.')
            }}
          />
        )}
        <ListState state={addresses}>
          {(data) => (
            <>
              {data.items.length === 0 ? (
                <div className="empty">Chưa có địa chỉ nhận hàng.</div>
              ) : (
                <div className="grid gap-4 md:grid-cols-2">
                  {data.items.map((address) => (
                    <article className="panel" key={address.id}>
                      <div className="flex justify-between gap-4">
                        <h3 className="font-medium">{address.recipient}</h3>
                        {address.is_default && (
                          <span className="text-xs text-emerald-700">Mặc định</span>
                        )}
                      </div>
                      <p className="mt-2 text-sm text-stone-500">{address.phone}</p>
                      <p className="mt-2 whitespace-pre-wrap text-sm text-stone-600">
                        {address.address}
                      </p>
                      <div className="mt-5 flex gap-4">
                        <button className="text-sm underline" onClick={() => setEditing(address)}>
                          Sửa địa chỉ
                        </button>
                        <button
                          disabled={busy}
                          className="text-sm text-red-700 underline"
                          onClick={() => void remove(address)}
                        >
                          Xóa địa chỉ
                        </button>
                      </div>
                    </article>
                  ))}
                </div>
              )}
              <Pagination page={page} pageSize={10} total={data.total} change={setPage} />
            </>
          )}
        </ListState>
      </section>
      <MyReviews />
    </main>
  )
}
function AddressForm({
  editing,
  cancel,
  saved,
}: {
  editing: Address | null
  cancel: () => void
  saved: () => void
}) {
  const { api } = useSession()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const values = new FormData(event.currentTarget)
    setBusy(true)
    setError('')
    try {
      await api.request<Address>(
        editing ? 'PUT' : 'POST',
        editing ? `/api/v1/me/addresses/${editing.id}` : '/api/v1/me/addresses',
        {
          recipient: String(values.get('recipient')),
          phone: String(values.get('phone')),
          address: String(values.get('address')),
          is_default: values.get('is_default') === 'on',
        },
      )
      saved()
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <form className="panel mb-6 space-y-4" onSubmit={submit}>
      <h3 className="font-medium">{editing ? 'Sửa địa chỉ' : 'Địa chỉ mới'}</h3>
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Người nhận">
          <input
            className="input"
            name="recipient"
            defaultValue={editing?.recipient}
            required
            maxLength={120}
          />
        </Field>
        <Field label="Số điện thoại">
          <input
            className="input"
            name="phone"
            type="tel"
            defaultValue={editing?.phone}
            required
            minLength={8}
            maxLength={20}
          />
        </Field>
      </div>
      <Field label="Địa chỉ">
        <textarea
          className="input min-h-24"
          name="address"
          defaultValue={editing?.address}
          required
          maxLength={500}
        />
      </Field>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" name="is_default" defaultChecked={editing?.is_default} />
        Đặt làm địa chỉ mặc định
      </label>
      {error && <Failure message={error} />}
      <div className="flex gap-3">
        <button className="button-primary" disabled={busy}>
          Lưu địa chỉ
        </button>
        <button type="button" className="button-secondary" onClick={cancel} disabled={busy}>
          Hủy
        </button>
      </div>
    </form>
  )
}
