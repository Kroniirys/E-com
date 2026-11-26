import { useEffect, useState, useSyncExternalStore } from 'react'
import type { ReactNode } from 'react'
import { SessionContext } from './session'
import { SessionRuntime } from './session-runtime'

export default function SessionProvider({ children }: { children: ReactNode }) {
  const [runtime] = useState(() => new SessionRuntime())
  const snapshot = useSyncExternalStore(runtime.subscribe, runtime.getSnapshot)
  useEffect(() => runtime.start(), [runtime])
  return (
    <SessionContext
      value={{
        ...snapshot,
        api: runtime.api,
        login: runtime.login,
        register: runtime.register,
        logout: runtime.logout,
        changedUser: runtime.changedUser,
        passwordChanged: runtime.passwordChanged,
      }}
    >
      {children}
    </SessionContext>
  )
}
