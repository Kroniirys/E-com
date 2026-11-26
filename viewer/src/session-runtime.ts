import { APIClient, ApiError, errorMessage, http } from './api'
import type { TokenResponse, User } from './api'

async function cookieLock<T>(work: () => Promise<T>): Promise<T> {
  if (navigator.locks) return navigator.locks.request('ecommerce-cookie-session', work)
  return work()
}
type Snapshot = { user: User | null; ready: boolean; sessionError: string | null }
export class SessionRuntime {
  private token: string | null = null
  private epoch = 0
  private flight: Promise<string | null> | null = null
  private channel: BroadcastChannel | null = null
  private tabID = crypto.randomUUID()
  private listeners = new Set<() => void>()
  private snapshot: Snapshot = { user: null, ready: false, sessionError: null }
  readonly api = new APIClient(
    () => this.token,
    () => this.refresh(),
    () => this.clear(),
    () => this.epoch,
  )
  subscribe = (listener: () => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }
  getSnapshot = () => this.snapshot
  private publish(next: Partial<Snapshot>) {
    this.snapshot = { ...this.snapshot, ...next }
    this.listeners.forEach((listener) => listener())
  }
  private clear() {
    this.epoch += 1
    this.token = null
    this.publish({ user: null })
  }
  private notify(action: 'login' | 'logout' | 'password') {
    this.channel?.postMessage({ action, source: this.tabID })
  }
  start = () => {
    if (typeof BroadcastChannel !== 'undefined') {
      this.channel = new BroadcastChannel('ecommerce-auth')
      this.channel.onmessage = (event: MessageEvent<unknown>) => {
        const message = event.data
        if (
          !message ||
          typeof message !== 'object' ||
          !('action' in message) ||
          !('source' in message) ||
          message.source === this.tabID ||
          !['login', 'logout', 'password'].includes(String(message.action))
        )
          return
        this.clear()
        if (message.action === 'login')
          void Promise.resolve(this.flight)
            .catch(() => null)
            .then(() => this.refresh())
            .catch((error: unknown) => this.publish({ sessionError: errorMessage(error) }))
      }
    }
    void this.refresh()
      .catch((error: unknown) => this.publish({ sessionError: errorMessage(error) }))
      .finally(() => this.publish({ ready: true }))
    return () => {
      this.channel?.close()
      this.channel = null
    }
  }
  private refresh(): Promise<string | null> {
    if (this.flight) return this.flight
    const epoch = this.epoch
    const work = cookieLock(async () => {
      try {
        const tokens = await http<TokenResponse>('POST', '/api/v1/auth/refresh')
        const user = await http<User>('GET', '/api/v1/auth/me', undefined, tokens.access_token)
        if (this.epoch !== epoch) return null
        if (this.snapshot.user?.id !== user.id) this.epoch += 1
        this.token = tokens.access_token
        this.publish({ user, sessionError: null })
        return tokens.access_token
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) {
          if (this.epoch === epoch) this.clear()
          return null
        }
        throw error
      }
    }).finally(() => {
      this.flight = null
    })
    this.flight = work
    return work
  }
  login = async (email: string, password: string): Promise<User> =>
    cookieLock(async () => {
      this.clear()
      const epoch = this.epoch
      const tokens = await http<TokenResponse>('POST', '/api/v1/auth/login', { email, password })
      const user = await http<User>('GET', '/api/v1/auth/me', undefined, tokens.access_token)
      if (epoch !== this.epoch) throw new ApiError(409, 'SESSION_CHANGED')
      this.token = tokens.access_token
      this.publish({ user, ready: true, sessionError: null })
      this.notify('login')
      return user
    })
  register = async (name: string, email: string, password: string) => {
    const epoch = this.epoch
    await http<User>('POST', '/api/v1/auth/register', { name, email, password })
    if (this.epoch !== epoch) throw new ApiError(409, 'SESSION_CHANGED')
    return this.login(email, password)
  }
  logout = async () =>
    cookieLock(async () => {
      await http<void>('POST', '/api/v1/auth/logout')
      this.clear()
      this.publish({ sessionError: null })
      this.notify('logout')
    })
  changedUser = (user: User) => this.publish({ user })
  passwordChanged = () => {
    this.clear()
    this.notify('password')
  }
}
