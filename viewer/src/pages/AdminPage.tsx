import { useState } from 'react'
import OrdersPage from './OrdersPage'
import EngagementAdmin from './EngagementAdmin'
import type { FormEvent } from 'react'
import { errorMessage, money } from '../api'
import type { Category, Movement, Page, Product, ProductInput, Stock, User } from '../api'
import { useSession } from '../session'
import { useCategoryOptions, useLoad } from '../hooks'
import { Failure, Field, ListState, Loading, Notice, Pagination, Status } from '../ui'

const tabs = [
  { key: 'products', title: 'Sản phẩm' },
  { key: 'categories', title: 'Danh mục' },
  { key: 'inventory', title: 'Tồn kho' },
  { key: 'users', title: 'Người dùng' },
  { key: 'orders', title: 'Đơn hàng' },
  { key: 'coupons', title: 'Coupon' },
  { key: 'reviews', title: 'Đánh giá' },
  { key: 'dashboard', title: 'Tổng quan' },
]
export default function AdminPage({ tab }: { tab: string }) {
  return (
    <main className="page space-y-7">
      <div>
        <p className="eyebrow">Khu vực quản trị</p>
        <h1 className="mt-3 text-3xl font-medium">Quản lý cửa hàng</h1>
      </div>
      <nav
        aria-label="Quản trị cửa hàng"
        className="flex gap-2 overflow-x-auto rounded-xl border border-stone-200 bg-white p-2"
      >
        {tabs.map((item) => (
          <a
            href={`#/admin/${item.key}`}
            key={item.key}
            aria-current={tab.split('/')[0] === item.key ? 'page' : undefined}
            className={`shrink-0 rounded-lg px-5 py-3 text-sm ${tab === item.key ? 'bg-[#183c32] text-white' : 'hover:bg-stone-100'}`}
          >
            {item.title}
          </a>
        ))}
      </nav>
      {['coupons', 'reviews', 'dashboard'].includes(tab) ? (
        <EngagementAdmin key={tab} tab={tab} />
      ) : tab === 'orders' || tab.startsWith('orders/') ? (
        <OrdersPage
          admin
          id={tab.startsWith('orders/') ? tab.slice('orders/'.length) : undefined}
        />
      ) : tab === 'products' ? (
        <Products />
      ) : tab === 'categories' ? (
        <Categories />
      ) : tab === 'inventory' ? (
        <Inventory />
      ) : tab === 'users' ? (
        <Users />
      ) : (
        <Failure message="Không tìm thấy mục quản trị." />
      )}
    </main>
  )
}
function Search({ label, submit }: { label: string; submit: (q: string) => void }) {
  return (
    <form
      className="flex items-end gap-2"
      onSubmit={(event) => {
        event.preventDefault()
        submit(String(new FormData(event.currentTarget).get('q') ?? ''))
      }}
    >
      <Field label={label}>
        <input className="input" name="q" maxLength={200} />
      </Field>
      <button className="button-secondary">Tìm kiếm</button>
    </form>
  )
}
function Products() {
  const { api } = useSession()
  const [page, setPage] = useState(1)
  const [query, setQuery] = useState('')
  const [editing, setEditing] = useState<Product | null | undefined>(undefined)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const params = new URLSearchParams({ page: String(page), page_size: '20', q: query })
  const state = useLoad<Page<Product>>(`/api/v1/admin/products?${params}`)
  const categories = useCategoryOptions()
  const remove = async (product: Product) => {
    if (!window.confirm(`Lưu trữ sản phẩm ${product.name}?`)) return
    setBusy(true)
    setError('')
    try {
      await api.request<void>('DELETE', `/api/v1/admin/products/${product.id}`)
      state.reload()
      setMessage('Đã lưu trữ sản phẩm. Lịch sử tồn kho được giữ lại.')
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <section className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Search
          label="Tìm sản phẩm quản trị"
          submit={(q) => {
            setQuery(q)
            setPage(1)
          }}
        />
        <button className="button-primary" onClick={() => setEditing(null)}>
          Tạo sản phẩm
        </button>
      </div>
      {error && <Failure message={error} />} {message && <Notice>{message}</Notice>}
      {editing !== undefined &&
        (categories.loading ? (
          <Loading />
        ) : categories.error ? (
          <Failure message={categories.error} retry={categories.reload} />
        ) : (
          <ProductForm
            key={editing?.id ?? 'new'}
            editing={editing}
            categories={categories.items}
            cancel={() => setEditing(undefined)}
            saved={() => {
              setEditing(undefined)
              state.reload()
              setMessage('Đã lưu sản phẩm.')
            }}
          />
        ))}
      <ListState state={state}>
        {(data) => (
          <>
            {data.items.length === 0 ? (
              <div className="empty">Chưa có sản phẩm phù hợp.</div>
            ) : (
              <div className="table-wrap">
                <table>
                  <caption className="sr-only">Danh sách sản phẩm quản trị</caption>
                  <thead>
                    <tr>
                      <th>Sản phẩm / SKU</th>
                      <th>Giá bán</th>
                      <th>Trạng thái</th>
                      <th>Tồn kho</th>
                      <th>Thao tác</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.items.map((product) => (
                      <tr key={product.id}>
                        <td>
                          <strong>{product.name}</strong>
                          <div className="mt-1 text-xs text-stone-500">{product.sku}</div>
                        </td>
                        <td>{money(product.price_minor)}</td>
                        <td>
                          <Status value={product.status} />
                        </td>
                        <td>{product.stock}</td>
                        <td>
                          <div className="flex gap-4">
                            <button className="link-button" onClick={() => setEditing(product)}>
                              Sửa
                            </button>
                            <button
                              disabled={busy}
                              className="link-button text-red-700"
                              onClick={() => void remove(product)}
                            >
                              Lưu trữ
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <Pagination page={page} pageSize={20} total={data.total} change={setPage} />
          </>
        )}
      </ListState>
    </section>
  )
}
function ProductForm({
  editing,
  categories,
  cancel,
  saved,
}: {
  editing: Product | null
  categories: Category[]
  cancel: () => void
  saved: () => void
}) {
  const { api } = useSession()
  const [images, setImages] = useState<ProductInput['images']>(editing?.images ?? [])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const values = new FormData(event.currentTarget)
    const payload: ProductInput = {
      sku: String(values.get('sku')),
      name: String(values.get('name')),
      description: String(values.get('description')),
      price_minor: Number(values.get('price_minor')),
      original_price_minor: Number(values.get('original_price_minor')),
      status: String(values.get('status')) as ProductInput['status'],
      category_ids: values.getAll('category_ids').map(String),
      images: images.map((image, index) => ({ ...image, sort_order: index })),
    }
    setBusy(true)
    setError('')
    try {
      await api.request<Product>(
        editing ? 'PUT' : 'POST',
        editing ? `/api/v1/admin/products/${editing.id}` : '/api/v1/admin/products',
        payload,
      )
      saved()
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  const imageChange = (index: number, field: 'url' | 'alt', value: string) =>
    setImages((items) =>
      items.map((item, position) => (position === index ? { ...item, [field]: value } : item)),
    )
  return (
    <form className="panel space-y-5" onSubmit={submit}>
      <h2 className="text-xl font-medium">{editing ? 'Sửa sản phẩm' : 'Sản phẩm mới'}</h2>
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Tên sản phẩm">
          <input
            className="input"
            name="name"
            defaultValue={editing?.name}
            required
            maxLength={200}
          />
        </Field>
        <Field label="Mã SKU">
          <input className="input" name="sku" defaultValue={editing?.sku} required maxLength={64} />
        </Field>
        <Field label="Giá bán (VND)">
          <input
            className="input"
            name="price_minor"
            type="number"
            min={0}
            max={9000000000000}
            step={1}
            defaultValue={editing?.price_minor ?? 0}
            required
          />
        </Field>
        <Field label="Giá gốc (VND)">
          <input
            className="input"
            name="original_price_minor"
            type="number"
            min={0}
            max={9000000000000}
            step={1}
            defaultValue={editing?.original_price_minor ?? 0}
            required
          />
        </Field>
        <Field label="Trạng thái sản phẩm">
          <select className="input" name="status" defaultValue={editing?.status ?? 'Draft'}>
            <option value="Draft">Bản nháp</option>
            <option value="Active">Đang hoạt động</option>
            <option value="Inactive">Ngừng bán</option>
          </select>
        </Field>
        <Field label="Danh mục sản phẩm" hint="Giữ Ctrl hoặc Command để chọn nhiều danh mục.">
          <select
            className="input"
            name="category_ids"
            multiple
            size={3}
            defaultValue={editing?.category_ids ?? []}
          >
            {categories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </select>
        </Field>
      </div>
      <Field label="Mô tả">
        <textarea
          className="input min-h-24"
          name="description"
          maxLength={10000}
          defaultValue={editing?.description ?? ''}
        />
      </Field>
      <fieldset className="space-y-3">
        <legend className="mb-3 text-sm font-medium">Hình ảnh sản phẩm ({images.length}/10)</legend>
        {images.map((image, index) => (
          <div
            key={index}
            className="grid items-end gap-3 rounded-xl border border-stone-200 p-3 md:grid-cols-[2fr_1fr_auto]"
          >
            <Field label={`URL ảnh ${index + 1}`}>
              <input
                className="input"
                type="url"
                maxLength={2048}
                value={image.url}
                onChange={(event) => imageChange(index, 'url', event.target.value)}
                required
              />
            </Field>
            <Field label={`Mô tả ảnh ${index + 1}`}>
              <input
                className="input"
                maxLength={200}
                value={image.alt}
                onChange={(event) => imageChange(index, 'alt', event.target.value)}
              />
            </Field>
            <button
              type="button"
              className="button-secondary"
              onClick={() =>
                setImages((items) => items.filter((_, position) => position !== index))
              }
            >
              Bỏ ảnh {index + 1}
            </button>
          </div>
        ))}
        <button
          type="button"
          disabled={images.length >= 10}
          className="button-secondary"
          onClick={() =>
            setImages((items) => [...items, { url: '', alt: '', sort_order: items.length }])
          }
        >
          Thêm ảnh URL
        </button>
      </fieldset>
      <p className="text-xs text-stone-500">
        Tồn kho ban đầu bằng 0. Điều chỉnh số lượng ở mục Tồn kho để có lịch sử đầy đủ.
      </p>
      {error && <Failure message={error} />}
      <div className="flex gap-3">
        <button className="button-primary" disabled={busy}>
          Lưu sản phẩm
        </button>
        <button type="button" className="button-secondary" disabled={busy} onClick={cancel}>
          Hủy
        </button>
      </div>
    </form>
  )
}
function Categories() {
  const { api } = useSession()
  const [page, setPage] = useState(1)
  const [editing, setEditing] = useState<Category | null | undefined>(undefined)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const state = useLoad<Page<Category>>(`/api/v1/admin/categories?page=${page}&page_size=20`)
  const remove = async (category: Category) => {
    if (!window.confirm(`Xóa danh mục ${category.name}? Sản phẩm vẫn được giữ lại.`)) return
    setBusy(true)
    setError('')
    try {
      await api.request<void>('DELETE', `/api/v1/admin/categories/${category.id}`)
      state.reload()
      setMessage('Đã xóa danh mục và gỡ các liên kết sản phẩm.')
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <section className="space-y-5">
      <div className="flex justify-end">
        <button className="button-primary" onClick={() => setEditing(null)}>
          Tạo danh mục
        </button>
      </div>
      {error && <Failure message={error} />} {message && <Notice>{message}</Notice>}
      {editing !== undefined && (
        <CategoryForm
          key={editing?.id ?? 'new'}
          editing={editing}
          cancel={() => setEditing(undefined)}
          saved={() => {
            setEditing(undefined)
            state.reload()
            setMessage('Đã lưu danh mục.')
          }}
        />
      )}
      <ListState state={state}>
        {(data) => (
          <>
            {data.items.length === 0 ? (
              <div className="empty">Chưa có danh mục.</div>
            ) : (
              <div className="table-wrap">
                <table>
                  <caption className="sr-only">Danh sách danh mục</caption>
                  <thead>
                    <tr>
                      <th>Tên danh mục</th>
                      <th>Đường dẫn</th>
                      <th>Thao tác</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.items.map((category) => (
                      <tr key={category.id}>
                        <td>{category.name}</td>
                        <td>{category.slug}</td>
                        <td>
                          <div className="flex gap-4">
                            <button className="link-button" onClick={() => setEditing(category)}>
                              Sửa
                            </button>
                            <button
                              className="link-button text-red-700"
                              disabled={busy}
                              onClick={() => void remove(category)}
                            >
                              Xóa
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <Pagination page={page} pageSize={20} total={data.total} change={setPage} />
          </>
        )}
      </ListState>
    </section>
  )
}
function CategoryForm({
  editing,
  cancel,
  saved,
}: {
  editing: Category | null
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
      await api.request<Category>(
        editing ? 'PUT' : 'POST',
        editing ? `/api/v1/admin/categories/${editing.id}` : '/api/v1/admin/categories',
        { name: String(values.get('name')), slug: String(values.get('slug')) },
      )
      saved()
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <form className="panel space-y-4" onSubmit={submit}>
      <h2 className="text-xl font-medium">{editing ? 'Sửa danh mục' : 'Danh mục mới'}</h2>
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Tên danh mục">
          <input
            className="input"
            name="name"
            defaultValue={editing?.name}
            required
            maxLength={120}
          />
        </Field>
        <Field label="Đường dẫn danh mục" hint="Chữ thường không dấu, số và dấu gạch nối.">
          <input
            className="input"
            name="slug"
            defaultValue={editing?.slug}
            required
            maxLength={120}
            pattern="[a-z0-9]+(-[a-z0-9]+)*"
          />
        </Field>
      </div>
      {error && <Failure message={error} />}
      <div className="flex gap-3">
        <button className="button-primary" disabled={busy}>
          Lưu danh mục
        </button>
        <button type="button" className="button-secondary" disabled={busy} onClick={cancel}>
          Hủy
        </button>
      </div>
    </form>
  )
}
function Users() {
  const { api } = useSession()
  const [page, setPage] = useState(1)
  const [query, setQuery] = useState('')
  const [editing, setEditing] = useState<User | null | undefined>(undefined)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const params = new URLSearchParams({ page: String(page), page_size: '20', q: query })
  const state = useLoad<Page<User>>(`/api/v1/admin/users?${params}`)
  const remove = async (user: User) => {
    if (!window.confirm(`Vô hiệu hóa và lưu trữ tài khoản ${user.email}?`)) return
    setBusy(true)
    setError('')
    try {
      await api.request<void>('DELETE', `/api/v1/admin/users/${user.id}`)
      state.reload()
      setMessage('Đã vô hiệu hóa tài khoản.')
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <section className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Search
          label="Tìm người dùng"
          submit={(q) => {
            setQuery(q)
            setPage(1)
          }}
        />
        <button className="button-primary" onClick={() => setEditing(null)}>
          Tạo người dùng
        </button>
      </div>
      {error && <Failure message={error} />} {message && <Notice>{message}</Notice>}
      {editing !== undefined && (
        <UserForm
          key={editing?.id ?? 'new'}
          editing={editing}
          cancel={() => setEditing(undefined)}
          saved={() => {
            setEditing(undefined)
            state.reload()
            setMessage('Đã lưu người dùng. Các phiên cũ của tài khoản đã bị thu hồi.')
          }}
        />
      )}
      <ListState state={state}>
        {(data) => (
          <>
            {data.items.length === 0 ? (
              <div className="empty">Chưa có người dùng phù hợp.</div>
            ) : (
              <div className="table-wrap">
                <table>
                  <caption className="sr-only">Danh sách người dùng</caption>
                  <thead>
                    <tr>
                      <th>Người dùng</th>
                      <th>Vai trò</th>
                      <th>Trạng thái</th>
                      <th>Thao tác</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.items.map((user) => (
                      <tr key={user.id}>
                        <td>
                          <strong>{user.name}</strong>
                          <div className="mt-1 text-xs text-stone-500">{user.email}</div>
                        </td>
                        <td>
                          <Status value={user.role} />
                        </td>
                        <td>
                          <Status value={user.status} />
                        </td>
                        <td>
                          <div className="flex gap-4">
                            <button className="link-button" onClick={() => setEditing(user)}>
                              Sửa
                            </button>
                            <button
                              className="link-button text-red-700"
                              disabled={busy}
                              onClick={() => void remove(user)}
                            >
                              Vô hiệu hóa
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <Pagination page={page} pageSize={20} total={data.total} change={setPage} />
          </>
        )}
      </ListState>
    </section>
  )
}
function UserForm({
  editing,
  cancel,
  saved,
}: {
  editing: User | null
  cancel: () => void
  saved: () => void
}) {
  const { api } = useSession()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const values = new FormData(event.currentTarget)
    const payload: {
      email: string
      name: string
      role: string
      status: string
      password?: string
    } = {
      email: String(values.get('email')),
      name: String(values.get('name')),
      role: String(values.get('role')),
      status: String(values.get('status')),
    }
    if (!editing) {
      payload.password = String(values.get('password'))
      if (
        [...payload.password].length < 12 ||
        new TextEncoder().encode(payload.password).length > 72
      ) {
        setError('Mật khẩu cần ít nhất 12 ký tự và tối đa 72 byte UTF-8.')
        return
      }
    }
    setBusy(true)
    setError('')
    try {
      await api.request<User>(
        editing ? 'PUT' : 'POST',
        editing ? `/api/v1/admin/users/${editing.id}` : '/api/v1/admin/users',
        payload,
      )
      saved()
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <form className="panel space-y-4" onSubmit={submit}>
      <h2 className="text-xl font-medium">{editing ? 'Sửa người dùng' : 'Người dùng mới'}</h2>
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Tên người dùng">
          <input
            className="input"
            name="name"
            defaultValue={editing?.name}
            required
            maxLength={120}
          />
        </Field>
        <Field label="Email người dùng">
          <input
            className="input"
            name="email"
            type="email"
            defaultValue={editing?.email}
            required
            maxLength={254}
          />
        </Field>
        <Field label="Vai trò">
          <select className="input" name="role" defaultValue={editing?.role ?? 'Customer'}>
            <option value="Customer">Khách hàng</option>
            <option value="Admin">Quản trị viên</option>
          </select>
        </Field>
        <Field label="Trạng thái tài khoản">
          <select className="input" name="status" defaultValue={editing?.status ?? 'Active'}>
            <option value="Active">Đang hoạt động</option>
            <option value="Disabled">Đã khóa</option>
          </select>
        </Field>
        {!editing && (
          <Field label="Mật khẩu ban đầu">
            <input
              className="input"
              name="password"
              type="password"
              required
              minLength={12}
              maxLength={72}
              autoComplete="new-password"
            />
          </Field>
        )}
      </div>
      {error && <Failure message={error} />}
      <div className="flex gap-3">
        <button className="button-primary" disabled={busy}>
          Lưu người dùng
        </button>
        <button type="button" className="button-secondary" disabled={busy} onClick={cancel}>
          Hủy
        </button>
      </div>
    </form>
  )
}
function Inventory() {
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<Stock | null>(null)
  const state = useLoad<Page<Stock>>(`/api/v1/admin/inventory?page=${page}&page_size=20`)
  return (
    <section className="space-y-5">
      <p className="text-sm text-stone-600">
        Mọi thay đổi tồn kho có lý do, người thực hiện và lịch sử. Yêu cầu gửi lại giữ nguyên khóa
        để tránh điều chỉnh hai lần.
      </p>
      {selected && (
        <StockPanel
          key={selected.product_id}
          stock={selected}
          cancel={() => setSelected(null)}
          updated={state.reload}
        />
      )}
      <ListState state={state}>
        {(data) => (
          <>
            {data.items.length === 0 ? (
              <div className="empty">Chưa có sản phẩm để quản lý tồn kho.</div>
            ) : (
              <div className="table-wrap">
                <table>
                  <caption className="sr-only">Tồn kho sản phẩm</caption>
                  <thead>
                    <tr>
                      <th>Sản phẩm</th>
                      <th>Số lượng</th>
                      <th>Phiên bản</th>
                      <th>Thao tác</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.items.map((stock) => (
                      <tr key={stock.product_id}>
                        <td>
                          <strong>{stock.product_name ?? stock.product_id}</strong>
                          <div className="mt-1 text-xs text-stone-500">
                            {stock.sku ?? stock.product_id}
                          </div>
                        </td>
                        <td>{stock.quantity}</td>
                        <td>{stock.version}</td>
                        <td>
                          <button className="link-button" onClick={() => setSelected(stock)}>
                            Điều chỉnh & lịch sử
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <Pagination page={page} pageSize={20} total={data.total} change={setPage} />
          </>
        )}
      </ListState>
    </section>
  )
}
function StockPanel({
  stock,
  cancel,
  updated,
}: {
  stock: Stock
  cancel: () => void
  updated: () => void
}) {
  const { api } = useSession()
  const [page, setPage] = useState(1)
  const [busy, setBusy] = useState(false)
  const [key, setKey] = useState(() => crypto.randomUUID())
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const product = useLoad<Product>(`/api/v1/admin/products/${stock.product_id}`)
  const history = useLoad<Page<Movement>>(
    `/api/v1/admin/inventory/${stock.product_id}/history?page=${page}&page_size=10`,
  )
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const form = event.currentTarget
    const values = new FormData(form)
    const delta = Number(values.get('delta'))
    if (!Number.isSafeInteger(delta) || delta === 0) {
      setError('Số lượng điều chỉnh phải là số nguyên khác 0.')
      return
    }
    setBusy(true)
    setError('')
    setMessage('')
    try {
      await api.request<Stock>(
        'POST',
        '/api/v1/admin/inventory/adjustments',
        { product_id: stock.product_id, delta, reason: String(values.get('reason')) },
        { idempotencyKey: key },
      )
      form.reset()
      setKey(crypto.randomUUID())
      updated()
      history.reload()
      setMessage('Đã điều chỉnh tồn kho và ghi lịch sử.')
    } catch (error) {
      setError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <section className="panel space-y-5" aria-label="Điều chỉnh tồn kho">
      <div className="flex items-center justify-between gap-4">
        <h2 className="text-xl font-medium">{product.data?.name ?? 'Điều chỉnh tồn kho'}</h2>
        <button type="button" className="button-secondary" onClick={cancel} disabled={busy}>
          Đóng
        </button>
      </div>
      <form
        className="space-y-4"
        onSubmit={submit}
        onChange={() => {
          if (!busy) setKey(crypto.randomUUID())
        }}
      >
        <div className="grid gap-4 md:grid-cols-2">
          <Field label="Số lượng điều chỉnh" hint="Số dương để nhập thêm; số âm để giảm tồn.">
            <input
              className="input"
              type="number"
              name="delta"
              step={1}
              min={-9000000000000}
              max={9000000000000}
              required
            />
          </Field>
          <Field label="Lý do điều chỉnh">
            <input className="input" name="reason" required maxLength={500} />
          </Field>
        </div>
        {error && <Failure message={error} />} {message && <Notice>{message}</Notice>}
        <button className="button-primary" disabled={busy}>
          Áp dụng điều chỉnh
        </button>
      </form>
      <h3 className="font-medium">Lịch sử tồn kho</h3>
      <ListState state={history}>
        {(data) => (
          <>
            {data.items.length === 0 ? (
              <div className="empty">Chưa có điều chỉnh nào.</div>
            ) : (
              <div className="table-wrap">
                <table>
                  <caption className="sr-only">Lịch sử điều chỉnh tồn kho</caption>
                  <thead>
                    <tr>
                      <th>Thời điểm</th>
                      <th>Thay đổi</th>
                      <th>Trước → Sau</th>
                      <th>Lý do</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.items.map((item) => (
                      <tr key={item.id}>
                        <td>{new Date(item.created_at).toLocaleString('vi-VN')}</td>
                        <td>
                          {item.delta > 0 ? '+' : ''}
                          {item.delta}
                        </td>
                        <td>
                          {item.before} → {item.after}
                        </td>
                        <td>{item.reason}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <Pagination page={page} pageSize={10} total={data.total} change={setPage} />
          </>
        )}
      </ListState>
    </section>
  )
}
