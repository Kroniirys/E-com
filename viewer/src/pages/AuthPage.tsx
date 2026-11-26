import { useState } from 'react'
import type { FormEvent } from 'react'
import { errorMessage } from '../api'
import { useSession } from '../session'
import { navigate } from '../hooks'
import { Failure, Field } from '../ui'
export default function AuthPage({ register }: { register: boolean }) {
  const session = useSession()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const values = new FormData(event.currentTarget)
    const email = String(values.get('email'))
    const password = String(values.get('password'))
    if (register && ([...password].length < 12 || new TextEncoder().encode(password).length > 72)) {
      setError('Mật khẩu cần ít nhất 12 ký tự và tối đa 72 byte UTF-8.')
      return
    }
    setBusy(true)
    setError('')
    try {
      const user = register
        ? await session.register(String(values.get('name')), email, password)
        : await session.login(email, password)
      navigate(user.role === 'Admin' ? '/admin/products' : '/')
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <main className="page grid gap-10 lg:grid-cols-2">
      <section className="py-8">
        <p className="eyebrow">Tài khoản cửa hàng</p>
        <h1 className="mt-4 text-4xl font-medium tracking-tight">
          {register ? 'Tạo một khởi đầu mới.' : 'Chào mừng bạn trở lại.'}
        </h1>
        <p className="mt-6 max-w-md leading-7 text-stone-600">
          {register
            ? 'Đăng ký để quản lý thông tin cá nhân và địa chỉ nhận hàng của bạn.'
            : 'Đăng nhập để quản lý tài khoản hoặc truy cập khu vực quản trị theo quyền được cấp.'}
        </p>
      </section>
      <section className="panel">
        <h2 className="mb-6 text-xl font-medium">{register ? 'Đăng ký tài khoản' : 'Đăng nhập'}</h2>
        <form className="space-y-5" onSubmit={submit}>
          {register && (
            <Field label="Họ và tên">
              <input className="input" name="name" autoComplete="name" required maxLength={120} />
            </Field>
          )}
          <Field label="Email">
            <input
              className="input"
              name="email"
              type="email"
              autoComplete="email"
              required
              maxLength={254}
            />
          </Field>
          <Field
            label="Mật khẩu"
            hint={register ? 'Ít nhất 12 ký tự, tối đa 72 byte UTF-8.' : undefined}
          >
            <input
              className="input"
              name="password"
              type="password"
              autoComplete={register ? 'new-password' : 'current-password'}
              required
              minLength={register ? 12 : 1}
              maxLength={72}
            />
          </Field>
          {error && <Failure message={error} />}
          <button className="button-primary w-full" disabled={busy}>
            {busy ? 'Đang xử lý…' : register ? 'Tạo tài khoản' : 'Đăng nhập'}
          </button>
        </form>
        <p className="mt-6 text-sm text-stone-500">
          {register ? 'Đã có tài khoản?' : 'Chưa có tài khoản?'}{' '}
          <a
            className="font-medium text-[#183c32] underline"
            href={register ? '#/login' : '#/register'}
          >
            {register ? 'Đăng nhập' : 'Đăng ký ngay'}
          </a>
        </p>
      </section>
    </main>
  )
}
