import { useEffect, useState, useSyncExternalStore } from 'react'
import { errorMessage } from './api'
import type { Category, Page } from './api'
import { useSession } from './session'

export function useRoute() {
  return useSyncExternalStore(
    (callback) => {
      window.addEventListener('hashchange', callback)
      return () => window.removeEventListener('hashchange', callback)
    },
    () => window.location.hash.slice(1) || '/',
    () => '/',
  )
}
export const navigate = (path: string) => {
  window.location.hash = path
}
export function useLoad<T>(path: string, auth = true) {
  const { api } = useSession()
  const [version, setVersion] = useState(0)
  const [result, setResult] = useState<{ path: string; version: number; data?: T; error?: string }>(
    { path: '', version: -1 },
  )
  useEffect(() => {
    const controller = new AbortController()
    api.request<T>('GET', path, undefined, { auth, signal: controller.signal }).then(
      (data) => {
        if (!controller.signal.aborted) setResult({ path, version, data })
      },
      (error: unknown) => {
        if (!controller.signal.aborted) setResult({ path, version, error: errorMessage(error) })
      },
    )
    return () => controller.abort()
  }, [api, path, version, auth])
  return {
    data: result.path === path && result.version === version ? result.data : undefined,
    error: result.path === path && result.version === version ? result.error : undefined,
    loading: result.path !== path || result.version !== version,
    reload: () => setVersion((value) => value + 1),
  }
}
export function useCategoryOptions() {
  const { api } = useSession()
  const [version, setVersion] = useState(0)
  const [state, setState] = useState<{ items: Category[]; loading: boolean; error?: string }>({
    items: [],
    loading: true,
  })
  useEffect(() => {
    const controller = new AbortController()
    const load = async () => {
      const first = await api.request<Page<Category>>(
        'GET',
        '/api/v1/categories?page_size=100',
        undefined,
        { auth: false, signal: controller.signal },
      )
      const items = [...first.items]
      for (let page = 2; page <= Math.ceil(first.total / 100); page += 1) {
        const next = await api.request<Page<Category>>(
          'GET',
          `/api/v1/categories?page_size=100&page=${page}`,
          undefined,
          { auth: false, signal: controller.signal },
        )
        items.push(...next.items)
      }
      return items
    }
    load().then(
      (items) => {
        if (!controller.signal.aborted) setState({ items, loading: false })
      },
      (error: unknown) => {
        if (!controller.signal.aborted)
          setState({ items: [], loading: false, error: errorMessage(error) })
      },
    )
    return () => controller.abort()
  }, [api, version])
  return {
    ...state,
    reload: () => {
      setState({ items: [], loading: true })
      setVersion((value) => value + 1)
    },
  }
}
