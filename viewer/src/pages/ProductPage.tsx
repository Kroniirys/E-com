import { useState } from 'react'
import ProductReviews from './ProductReviews'
import { money, errorMessage } from '../api'
import { useSession } from '../session'
import type { Product } from '../api'
import { useCategoryOptions, useLoad } from '../hooks'
import { Failure, Field, Loading, Notice } from '../ui'
export default function ProductPage({ id }: { id: string }) {
  const state = useLoad<Product>(`/api/v1/products/${encodeURIComponent(id)}`, false)
  const categories = useCategoryOptions()
  const { api, user } = useSession()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [added, setAdded] = useState(false)
  if (state.loading)
    return (
      <main className="page">
        <Loading />
      </main>
    )
  if (state.error || !state.data)
    return (
      <main className="page">
        <Failure message={state.error ?? 'Không tìm thấy sản phẩm.'} retry={state.reload} />
      </main>
    )
  const product = state.data
  return (
    <main className="page">
      <a href="#/" className="text-sm text-stone-500 hover:underline">
        ← Về danh mục
      </a>
      <div className="mt-7 grid gap-10 lg:grid-cols-2">
        <div>
          <div className="grid aspect-square place-items-center overflow-hidden rounded-3xl bg-[#e8edde]">
            {product.images[0] ? (
              <img
                className="size-full object-cover"
                src={product.images[0].url}
                alt={product.images[0].alt || product.name}
                referrerPolicy="no-referrer"
              />
            ) : (
              <span className="text-9xl text-[#80966c]" aria-hidden="true">
                {product.name.charAt(0)}
              </span>
            )}
          </div>
          {product.images.length > 1 && (
            <div className="mt-4 grid grid-cols-3 gap-3">
              {product.images.slice(1).map((image, index) => (
                <img
                  className="aspect-square w-full rounded-xl object-cover"
                  key={`${image.url}-${index}`}
                  src={image.url}
                  alt={image.alt || product.name}
                  loading="lazy"
                  referrerPolicy="no-referrer"
                />
              ))}
            </div>
          )}
        </div>
        <section className="py-4">
          <p className="eyebrow">{product.sku}</p>
          <h1 className="mt-4 text-4xl font-medium tracking-tight">{product.name}</h1>
          <div className="my-6 flex items-center gap-4">
            <strong className="text-2xl">{money(product.price_minor)}</strong>
            {product.original_price_minor > product.price_minor && (
              <del className="text-stone-400">{money(product.original_price_minor)}</del>
            )}
          </div>
          <p className="rounded-xl border border-stone-200 bg-white p-4 text-sm">
            {product.out_of_stock
              ? 'Sản phẩm hiện đã hết hàng.'
              : `Còn ${product.stock} sản phẩm trong kho.`}
          </p>
          <p className="mt-6 whitespace-pre-wrap leading-8 text-stone-600">
            {product.description || 'Chưa có mô tả cho sản phẩm này.'}
          </p>
          <div className="mt-6 flex flex-wrap gap-2">
            {categories.items
              .filter((item) => product.category_ids.includes(item.id))
              .map((item) => (
                <span key={item.id} className="rounded-full bg-[#e8edde] px-3 py-1 text-xs">
                  {item.name}
                </span>
              ))}
          </div>
          <div className="mt-8 border-t border-stone-200 pt-5">
            {error && <Failure message={error} />}
            {added && (
              <Notice>
                Đã thêm vào giỏ hàng.{' '}
                <a href="#/cart" className="underline">
                  Xem giỏ hàng
                </a>
              </Notice>
            )}
            {user ? (
              <form
                className="mt-4 flex flex-wrap items-end gap-3"
                onSubmit={async (e) => {
                  e.preventDefault()
                  const quantity = Number(new FormData(e.currentTarget).get('quantity'))
                  setBusy(true)
                  setError('')
                  setAdded(false)
                  try {
                    await api.request('POST', '/api/v1/cart/items', {
                      product_id: product.id,
                      quantity,
                    })
                    setAdded(true)
                  } catch (error) {
                    setError(errorMessage(error))
                  } finally {
                    setBusy(false)
                  }
                }}
              >
                <Field label="Số lượng mua">
                  <input
                    name="quantity"
                    type="number"
                    className="input !w-24"
                    defaultValue={1}
                    min={1}
                    max={Math.min(1000, product.stock)}
                    required
                    disabled={busy || product.out_of_stock}
                  />
                </Field>
                <button className="button-primary" disabled={busy || product.out_of_stock}>
                  {busy ? 'Đang thêm…' : 'Thêm vào giỏ hàng'}
                </button>
              </form>
            ) : (
              <a href="#/login" className="button-primary inline-block">
                Đăng nhập để mua hàng
              </a>
            )}
          </div>
        </section>
      </div>
      <ProductReviews id={id} />
    </main>
  )
}
