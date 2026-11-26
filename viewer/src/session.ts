import { createContext, useContext } from 'react'
import type { APIClient, User } from './api'
export type Session = {
  user: User | null
  ready: boolean
  sessionError: string | null
  api: APIClient
  login: (email: string, password: string) => Promise<User>
  register: (name: string, email: string, password: string) => Promise<User>
  logout: () => Promise<void>
  changedUser: (user: User) => void
  passwordChanged: () => void
}
export const SessionContext = createContext<Session | null>(null)
export function useSession(): Session {
  const session = useContext(SessionContext)
  if (!session) throw new Error('Session provider is missing')
  return session
}
