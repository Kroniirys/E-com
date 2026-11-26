import { useState } from 'react'
import type { FormEvent } from 'react'
import { money } from '../api'
import type { Page, Product } from '../api'
import { useCategoryOptions, useLoad } from '../hooks'
import { Failure, Field, ListState, Pagination } from '../ui'

export default function CatalogPage() {
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState('')
  const [sort, setSort] = useState('created_at')
  const [page, setPage] = useState(1)
  const categories = useCategoryOptions()
  const parameters = new URLSearchParams({ q: query, sort, page: String(page), page_size: '12' })
  if (category) parameters.set('category_id', category)
  const state = useLoad<Page<Product>>(`/api/v1/products?${parameters}`, false)
  const search = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setQuery(String(new FormData(event.currentTarget).get('q') ?? ''))
    setPage(1)
  }
  return (
    <main className="page space-y-8">
      <section className="grid gap-8 rounded-3xl bg-[#e8edde] p-8 lg:grid-cols-[1.4fr_1fr] lg:p-12">
        <div>
          <p className="eyebrow">E-commerce Base · Bộ sưu tập</p>
          <h1 className="mt-4 max-w-xl text-4xl font-medium leading-tight tracking-tight sm:text-5xl">
            Những điều giản dị.
            <br />
            <span className="text-[#627a49]">Cho không gian của bạn.</span>
          </h1>
          <p className="mt-5 max-w-lg leading-7 text-[#52665a]">
            Khám phá sản phẩm và thông tin tồn kho trực tiếp từ cửa hàng.
          </p>
        </div>
        <div
          className="flex min-h-40 items-center justify-center rounded-2xl border border-[#cad7bb] bg-[#f4f6ed]"
          aria-hidden="true"
        >
          <div className="text-center">
            <div className="text-7xl font-light text-[#839c62]">e.</div>
            <div className="mt-2 text-xs tracking-[0.25em] text-[#52665a]">CURATED EVERYDAY</div>
          </div>
        </div>
      </section>
      <section aria-labelledby="catalog-title">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="eyebrow">Danh mục cửa hàng</p>
            <h2 id="catalog-title" className="mt-2 text-2xl font-medium">
              Khám phá sản phẩm
            </h2>
          </div>
          <span className="text-sm text-stone-500">Chọn sản phẩm để thêm vào giỏ hàng</span>
        </div>
        <div className="mb-6 grid gap-4 rounded-2xl border border-stone-200 bg-white p-4 md:grid-cols-[1.5fr_1fr_1fr]">
          <form onSubmit={search} className="flex items-end gap-2">
            <div className="grow">
              <Field label="Tìm sản phẩm">
                <input
                  className="input"
                  name="q"
                  placeholder="Tên sản phẩm hoặc SKU"
                  defaultValue={query}
                  maxLength={200}
                />
              </Field>
            </div>
            <button className="button-primary" type="submit">
              Tìm kiếm
            </button>
          </form>
          <Field label="Danh mục">
            <select
              className="input"
              value={category}
              onChange={(event) => {
                setCategory(event.target.value)
                setPage(1)
              }}
            >
              <option value="">Tất cả danh mục</option>
              {categories.items.map((item) => (
                <option value={item.id} key={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Sắp xếp">
            <select
              className="input"
              value={sort}
              onChange={(event) => {
                setSort(event.target.value)
                setPage(1)
              }}
            >
              <option value="created_at">Mới nhất</option>
              <option value="name">Tên sản phẩm</option>
              <option value="price_asc">Giá tăng dần</option>
              <option value="price_desc">Giá giảm dần</option>
            </select>
          </Field>
        </div>
        {categories.error && <Failure message={categories.error} retry={categories.reload} />}
        <ListState state={state}>
          {(data) => (
            <>
              {data.items.length === 0 ? (
                <div className="empty">
                  Chưa có sản phẩm phù hợp. Thử đổi từ khóa hoặc danh mục.
                </div>
              ) : (
                <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
                  {data.items.map((product) => (
                    <a
                      key={product.id}
                      href={`#/products/${product.id}`}
                      className="group overflow-hidden rounded-2xl border border-stone-200 bg-white transition hover:-translate-y-1 hover:shadow-lg"
                    >
                      <div className="relative grid aspect-[4/3] place-items-center bg-[#edf0e6]">
                        {product.images[0] ? (
                          <img
                            src={product.images[0].url}
                            alt={product.images[0].alt || product.name}
                            loading="lazy"
                            referrerPolicy="no-referrer"
                            className="size-full object-cover"
                          />
                        ) : (
                          <span aria-hidden="true" className="text-7xl font-light text-[#879c73]">
                            {product.name.charAt(0)}
                          </span>
                        )}
                        <span
                          className={`absolute left-4 top-4 rounded-full px-3 py-1 text-xs ${product.out_of_stock ? 'bg-stone-100 text-stone-600' : 'bg-white text-emerald-900'}`}
                        >
                          {product.out_of_stock ? 'Hết hàng' : 'Còn hàng'}
                        </span>
                      </div>
                      <div className="p-5">
                        <p className="text-xs uppercase tracking-wider text-stone-500">
                          {product.sku}
                        </p>
                        <h3 className="mt-2 text-lg font-medium group-hover:underline">
                          {product.name}
                        </h3>
                        <div className="mt-3 flex items-center gap-3">
                          <span className="font-semibold">{money(product.price_minor)}</span>
                          {product.original_price_minor > product.price_minor && (
                            <del className="text-sm text-stone-400">
                              {money(product.original_price_minor)}
                            </del>
                          )}
                        </div>
                      </div>
                    </a>
                  ))}
                </div>
              )}
              <Pagination page={page} pageSize={12} total={data.total} change={setPage} />
            </>
          )}
        </ListState>
      </section>
    </main>
  )
}
