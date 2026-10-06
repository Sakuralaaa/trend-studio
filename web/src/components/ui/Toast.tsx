import React, { createContext, useContext, useState, useCallback } from 'react'
import { CheckCircle2, AlertCircle, Info, X } from 'lucide-react'

type ToastType = 'success' | 'error' | 'info'

interface ToastMessage {
  id: string
  type: ToastType
  text: string
}

interface ToastContextType {
  toast: (text: string, type?: ToastType) => void
  success: (text: string) => void
  error: (text: string) => void
  info: (text: string) => void
}

const ToastContext = createContext<ToastContextType | null>(null)

export const ToastProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [toasts, setToasts] = useState<ToastMessage[]>([])

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  const toast = useCallback(
    (text: string, type: ToastType = 'info') => {
      const id = 'toast_' + Math.random().toString(36).substring(2, 8)
      setToasts((prev) => [...prev, { id, type, text }])
      setTimeout(() => {
        removeToast(id)
      }, 3500)
    },
    [removeToast]
  )

  const success = useCallback((text: string) => toast(text, 'success'), [toast])
  const error = useCallback((text: string) => toast(text, 'error'), [toast])
  const info = useCallback((text: string) => toast(text, 'info'), [toast])

  return (
    <ToastContext.Provider value={{ toast, success, error, info }}>
      {children}
      <div className="toast-stack fixed bottom-6 right-6 z-50 flex flex-col gap-2.5 max-w-sm w-full pointer-events-none">
        {toasts.map((t) => (
          <div
            key={t.id}
            role={t.type === 'error' ? 'alert' : 'status'}
            className={`pointer-events-auto flex items-start gap-3 p-4 rounded-xl border shadow-lg transition-all animate-in slide-in-from-bottom-2 ${
              t.type === 'success'
                ? 'bg-white border-emerald-200 text-emerald-950 shadow-emerald-500/5'
                : t.type === 'error'
                ? 'bg-white border-rose-200 text-rose-950 shadow-rose-500/5'
                : 'bg-white border-neutral-200 text-neutral-900 shadow-neutral-500/5'
            }`}
          >
            {t.type === 'success' && <CheckCircle2 className="w-5 h-5 text-emerald-600 shrink-0 mt-0.5" />}
            {t.type === 'error' && <AlertCircle className="w-5 h-5 text-rose-600 shrink-0 mt-0.5" />}
            {t.type === 'info' && <Info className="w-5 h-5 text-neutral-600 shrink-0 mt-0.5" />}
            <div className="text-sm font-medium leading-relaxed flex-1">{t.text}</div>
            <button
              aria-label="关闭通知"
              onClick={() => removeToast(t.id)}
              className="text-neutral-400 hover:text-neutral-600 p-0.5 rounded transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast(): ToastContextType {
  const ctx = useContext(ToastContext)
  if (!ctx) {
    throw new Error('useToast must be used within ToastProvider')
  }
  return ctx
}
