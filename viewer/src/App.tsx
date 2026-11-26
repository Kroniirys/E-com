import { useState } from 'react'
import type { ReactNode } from 'react'
import SessionProvider from './SessionProvider'
import { errorMessage } from './api'
import { useSession } from './session'
import { useRoute } from './hooks'
import { Failure, Loading } from './ui'
import AccountPage from './pages/AccountPage'
import NotificationsPage from './pages/NotificationsPage'
import CartPage from './pages/CartPage'
import OrdersPage from './pages/OrdersPage'
import AdminPage from './pages/AdminPage'
import AuthPage from './pages/AuthPage'
import CatalogPage from './pages/CatalogPage'
import FoundationPage from './pages/FoundationPage'
import ProductPage from './pages/ProductPage'

export default function App() {
  return (
    <SessionProvider>
      <Workspace />
    </SessionProvider>
  )
}
function Workspace() {
  const route = useRoute()
  const session = useSession()
  const [error, setError] = useState('')
  const logout = async () => {
    setError('')
    try {
      const before = window.location.hash
      await session.logout()
      if (window.location.hash === before) window.location.hash = '/'
    } catch (error) {
      setError(errorMessage(error))
    }
  }
  let content: ReactNode
  if (route === '/') content = <CatalogPage />
  else if (route === '/system') content = <FoundationPage />
  else if (route === '/login' || route === '/register')
    content = <AuthPage key={route} register={route === '/register'} />
  else if (route.startsWith('/products/'))
    content = <ProductPage key={route} id={route.slice('/products/'.length)} />
  else if (
    route === '/notifications' ||
    route === '/account' ||
    route === '/cart' ||
    route === '/orders' ||
    route.startsWith('/orders/') ||
    route.startsWith('/admin/')
  ) {
    if (!session.ready)
      content = (
        <main className="page">
          <Loading />
        </main>
      )
    else if (!session.user)
      content = (
        <main className="page">
          <section className="panel">
            <h1 className="text-2xl font-medium">Bạn cần đăng nhập</h1>
            <p className="mt-4 text-stone-600">Vui lòng đăng nhập để truy cập khu vực này.</p>
            <a className="button-primary mt-6 inline-block" href="#/login">
              Đến trang đăng nhập
            </a>
          </section>
        </main>
      )
    else if (route.startsWith('/admin/') && session.user.role !== 'Admin')
      content = (
        <main className="page">
          <Failure message="Bạn không có quyền truy cập khu vực quản trị." />
        </main>
      )
    else if (route === '/notifications') content = <NotificationsPage key={session.user.id} />
    else if (route === '/account') content = <AccountPage key={session.user.id} />
    else if (route === '/cart') content = <CartPage key={session.user.id} />
    else if (route === '/orders' || route.startsWith('/orders/'))
      content = (
        <main className="page">
          <OrdersPage
            key={`${route}-${session.user.id}`}
            id={route === '/orders' ? undefined : route.slice('/orders/'.length)}
          />
        </main>
      )
    else
      content = (
        <AdminPage key={`${route}-${session.user.id}`} tab={route.slice('/admin/'.length)} />
      )
  } else
    content = (
      <main className="page">
        <Failure message="Không tìm thấy trang. Vui lòng quay về danh mục." />
      </main>
    )
  return (
    <div className="min-h-screen bg-[#f7f8f2] text-[#183c32]">
      <header className="border-b border-[#dae1d4] bg-[#f7f8f2]">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-5 px-6 py-5 lg:px-10">
          <a
            href="#/"
            className="flex items-center gap-3 font-semibold tracking-tight"
            aria-label="E-commerce Base, trang chủ"
          >
            <span
              aria-hidden="true"
              className="grid size-10 place-items-center rounded-xl bg-[#183c32] text-xl text-[#d5ef91]"
            >
              e.
            </span>
            <span>
              E-commerce <span className="font-normal text-stone-500">/ Base</span>
            </span>
          </a>
          <nav aria-label="Điều hướng chính" className="flex flex-wrap items-center gap-4 text-sm">
            <a href="#/" className="hover:underline">
              Cửa hàng
            </a>
            <a href="#/system" className="hover:underline">
              Trạng thái
            </a>
            {session.user ? (
              <>
                <a href="#/notifications" className="hover:underline">
                  Thông báo
                </a>
                <a href="#/cart" className="hover:underline">
                  Giỏ hàng
                </a>
                <a href="#/orders" className="hover:underline">
                  Đơn hàng
                </a>
                <a href="#/account" className="hover:underline">
                  Tài khoản
                </a>
                {session.user.role === 'Admin' && (
                  <a href="#/admin/products" className="hover:underline">
                    Quản trị
                  </a>
                )}
                <button className="button-secondary" onClick={() => void logout()}>
                  Đăng xuất
                </button>
              </>
            ) : (
              <>
                <a href="#/login" className="hover:underline">
                  Đăng nhập
                </a>
                <a href="#/register" className="button-primary">
                  Đăng ký
                </a>
              </>
            )}
          </nav>
        </div>
        {session.user && (
          <div className="mx-auto max-w-6xl px-6 pb-3 text-xs text-stone-500 lg:px-10">
            Xin chào, {session.user.name} ·{' '}
            {session.user.role === 'Admin' ? 'Quản trị viên' : 'Khách hàng'}
          </div>
        )}
      </header>
      {(error || session.sessionError) && (
        <div className="page !py-4">
          <Failure message={error || session.sessionError || ''} />
        </div>
      )}
      {content}
      <footer className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 border-t border-[#dae1d4] px-6 py-7 text-xs text-[#68796f] lg:px-10">
        <span>E-commerce Base · Phase 05</span>
        <a className="underline" href="/api-docs/index.html">
          Tài liệu API ↗
        </a>
      </footer>
    </div>
  )
}
