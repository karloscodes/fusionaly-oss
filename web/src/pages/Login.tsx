import { LoginForm } from "@/components/login-form"
import type { RouteData, FlashMessage } from "@/types"

// Support both old SSR (data prop) and new Inertia (individual props)
interface LoginProps {
  data?: RouteData
  flash?: FlashMessage
}

export function Login({ data: _data, flash }: LoginProps) {
  return (
    <div className="flex min-h-svh w-full items-center justify-center p-6 md:p-10">
      <div className="w-full max-w-sm">
        <LoginForm flash={flash} />
      </div>
    </div>
  )
}
